# taiga roadmap

The goal is seeing and comparing runs in a browser.
Pieces: this API, the browser frontend (`taiga-frontend`), and the `taco` CLI (`taiga-cliclient`) for scripting.
Milestones are roughly ordered; each should fit in one PR per repo. M5 comes before M6/M7 because both need old workouts re-read from their files.

**Non-goals:** sharing and anything cross-user (friends, feeds, public profiles, leaderboards between users).
Each user's data is private to them, and every query stays scoped by `user_id`.
Multiple accounts exist only so more than one person can use the same deployment.

---

## M1 — Sign in with an OIDC provider (alongside passwords)

**Goal:** a user can sign in through any OpenID Connect provider (authentik is the first target, but nothing may be authentik-specific), and password login keeps working.

### Design

- **Generic OIDC, configured by issuer URL.**
  Endpoints come from the provider's discovery document (`/.well-known/openid-configuration`), so authentik, Keycloak, Google etc. are config, not code.
  Config holds a list of providers:
  ```yaml
  auth:
    password_login: true
    public_base_url: "https://taiga.example.com"   # what the browser sees; callbacks are built from it
    oidc:
      - name: authentik
        display_name: "authentik"
        issuer: "https://auth.example.com/application/o/taiga/"
        client_id: "..."
        client_secret_file: "/run/secrets/taiga-oidc-authentik"
        scopes: [openid, email, profile]
  ```
  Libraries: `github.com/coreos/go-oidc/v3` (discovery, ID-token verification against the JWKS) and `golang.org/x/oauth2` (authorization code + PKCE).
- **The backend runs the flow; the rest of the API doesn't change.**
  - `GET /auth/providers` lists the configured providers (name, display name) so the login page can render buttons, and says whether password login is enabled.
  - `GET /auth/oidc/{provider}/login` redirects to the provider with `state`, `nonce` and a PKCE challenge.
    Those live in a short-lived, HttpOnly, signed cookie scoped to the callback path, so the server stays stateless.
  - `GET /auth/oidc/{provider}/callback` checks `state`, exchanges the code, verifies the ID token (issuer, audience, expiry, `nonce`), resolves the user, and mints the **same taiga JWT** `/auth/tokens` issues today.
    It then redirects to the frontend with the token in the URL fragment (`/#/login/callback?token=…`), which browsers don't send to servers or write to access logs.
    The frontend stores it exactly as it stores a password-login token.
  - Behind the frontend's nginx `/api` proxy the redirect URI is `{public_base_url}/api/auth/oidc/{provider}/callback`, and that's what gets registered with the provider.
- **Identity is (issuer, subject), never email.**
  New table `user_identities (id, user_id → users, issuer, subject, email_at_link, created_at, UNIQUE (issuer, subject))`.
  Callback lookup is by `(issuer, sub)` only.
  `users.password` is already nullable, so an OIDC-only user has no password row.
- **Account creation and linking.**
  - Unknown identity → create a user (display name from `name`/`preferred_username`, email from `email`) and link it.
    A per-provider `allow_signup: false` turns this off for invite-only setups.
  - Linking to an existing password account is explicit: from a logged-in session, `POST /auth/identities/{provider}` starts the same flow and attaches the identity to the current user.
  - Auto-linking by matching email is **off by default**.
    A per-provider `link_by_verified_email: true` enables it, and only when the ID token has `email_verified: true` — otherwise anyone who can set an email at some provider could take over an account.
  - `GET /auth/identities` and `DELETE /auth/identities/{id}` to see and unlink, refusing to remove a user's last way to sign in.
- **Password login stays.**
  `auth.password_login: false` disables `/auth/register` and `/auth/tokens` for SSO-only deployments.
- **Out of scope for M1:** single logout at the provider, refresh tokens (the taiga JWT still lasts 2h; with an active provider session re-auth is a redirect and back), and SCIM or group-based roles.

### Tasks

- [x] Config: `auth.*` section, provider list, secrets read from files; refuse to start if a provider's discovery fails at boot (log which).
  - From env alone, `TAIGA_AUTH_OIDC` holds the list as JSON.
- [x] Migration `user_identities` (000012) (postgres + sqlite) and sqlc queries.
- [x] `internal/oidc`: provider registry from config, login redirect, callback handling, state/nonce/PKCE cookie.
  - The cookie's HMAC key is derived from the token signing key, so a flow survives a restart and works across replicas.
- [x] Routes: `/auth/providers`, `/auth/oidc/{provider}/{login,callback}`, `/auth/identities` (list, link, unlink).
  - Linking is `POST /auth/identities/{provider}` returning `{url}` rather than a redirect, since a fetch carrying the bearer token can't navigate; the flow cookie remembers whom to link to.
- [x] Tests against an in-process fake provider (`testsupport.FakeOIDC`, written for the job instead of mockoidc so each test can pick the audience, expiry and nonce): happy path, bad `state`, no or replayed flow cookie, bad `nonce`, wrong audience, expired token, the provider's own error, signup disabled, email auto-link on and off, `email_verified: false`, a new account without an email, linking and unlinking, password login off.
- [ ] Frontend: provider buttons on the login page from `/auth/providers`, a `#/login/callback` route that stores the token, and a linked-accounts section.
- [x] Docs: an authentik walkthrough plus the generic settings any provider needs (`docs/oidc.md`).

**Exit criteria:** with only an authentik provider configured, a new user can sign in from the frontend and use every page; an existing password user can link authentik and then sign in either way; swapping in a second provider (e.g. Keycloak) needs config only.

---

## M2 — Upload hygiene

- [x] Unparsable FIT files and multi-session files (e.g. triathlons) return **422** with a reason, not 500.
- [x] Don't leave orphan `activity_file` rows when parsing fails (or expose them so they can be retried or deleted). Exposed via `GET /activity_files?without_workout=true`; kept rather than deleted so multi-sport files can be processed once supported.
- [x] Deduplicate uploads: the same file (same content hash, already computed) for the same user returns the existing workout instead of creating a second one.
- [x] Backfill segments for workouts created before `workout_segments` existed. (First as `POST /segments/recompute`; M5 replaced it with reprocessing, which backfills on its own.)
- [ ] Frontend and `taco`: say "already uploaded" on a `200` re-upload, offer a backfill action, and list files that failed to parse.

## M3 — Deploy

- [ ] Run backend + frontend images together. Test stack at `taiga.i.hruday.me` on the homelab k8s cluster (CNPG Postgres, local files PVC); manifests and runbook live in proxmox-homelab (`k8s-manifests/taiga`, `docs/taiga.md`), not here.
- [x] Env-only configuration: every config key binds to its `TAIGA_*` variable, and the image starts the server by default.
- [x] A production CORS story: none needed, because the frontend's `/api` proxy is the only browser entry point and `TAIGA_APP_ENVIRONMENT=production` keeps the dev-only wildcard CORS off.

### Before any public access

The test stack is internal-only (LAN and tailnet). These must land before taiga is reachable from the internet.

- [x] **Closable registration.** Before this, `POST /auth/register` was always open, so anyone who could reach the API could make an account.
  - `auth.registration: open | closed` (env `TAIGA_AUTH_REGISTRATION`); closed returns 403 from `/auth/register`.
  - Default closed outside development, so a prod deploy is closed unless it opts in.
  - `cmd/useradd` to create accounts while registration is closed (run via `kubectl exec`).
  - `GET /auth/providers` reports `registration_open` (the endpoint M1 extends with OIDC providers); the frontend hides its register link from it, but the 403 is the real control.
  - Separate from M1's `auth.password_login: false`, which disables password login entirely. With OIDC, `closed` plus a provider's `allow_signup: false` gives invite-only through authentik.

## M4 — Cleanup

- [ ] Remove the `hats` template code and its table.
- [ ] Retire or refresh `terraform/` (AWS ECS + RDS): the homelab stack replaces it as the deploy path, and it predates env-only config.
- [x] Fix CLAUDE.md drift (`/auth/tokens`, config keys, migration directories, package list) and replace the template README.
- [ ] Rename the `trygo-template` service name that `GET /` and `GET /health` still report.
- [ ] Tests for `internal/tokens`, `internal/store`, `internal/filerepo`.

## M5 — Reprocessing and a work queue

The FIT file is the source of truth; everything taiga computes from it (summary, segments, and later series and heart rate) is derived data that should be rebuildable whenever the app learns something new.

- [x] **Versioned processing.** Each workout carries the `processor_version` that built it: `extractorVersion` (bumped by hand when extraction changes) plus a hash of `StandardDistances`, so editing the distance list makes every workout stale without anyone remembering to bump.
- [x] **Reprocess from the file.** `ReprocessWorkout` rebuilds the summary and replaces the whole segment set in one transaction (via the new `store.InTx`), so added distances get filled in and removed ones disappear. It replaced `POST /segments/recompute`.
- [x] **Hand edits.** A workout from a file is derived data only: `PUT` returns 422 for it. If hand corrections turn out to be wanted, they go in separate override columns that reprocessing never touches.
- [x] **A job queue in the database**, not new infrastructure: a `jobs` table and a worker goroutine in the server (`internal/jobs`).
  - The claim is one `UPDATE … WHERE id = (SELECT … LIMIT 1) AND state = 'queued' RETURNING *` that runs on both Postgres and SQLite; a lost race claims nothing. `SKIP LOCKED` buys nothing with one replica.
  - Leases (5 min) so a crashed worker's job is claimed again; exponential backoff from 30s; dead after 5 attempts, or at once for errors a retry can't fix.
  - On boot the worker queues `process_workout` for every stale workout. `POST /workouts/reprocess` queues all of a user's; `GET /jobs`, `POST /jobs/{id}/retry` to watch and retry.
- [x] Standard distances gained `3k` and `2mi`; existing workouts pick them up through the above.
- [ ] Client side: a reprocess action and dead jobs, as part of the admin panel (see Later).

**Deferred:**
- **Async uploads.** Parsing takes milliseconds, so `202` plus polling in both clients isn't worth it yet. Uploads that genuinely run in the background arrive with M8's `sync_source` jobs.
- **`update_stats` jobs.** Nothing is aggregated across workouts yet. Add the job kind with the first stat that needs it (M6 trends, M7 zones).

## M6 — Visualisations

- [x] **Per-record series.** Built at processing time and stored as one JSON document per workout (`workout_series`), rebuilt by M5 reprocessing (`extractorVersion` 2 backfills old workouts).
  Columns: time, distance, speed, altitude, heart rate, lat/long, sampled evenly to at most 2000 points; a column the device never recorded is omitted.
  Served gzipped from `GET /workouts/{id}/series`, so pages never re-parse the FIT file.
- [x] **Mile splits** on the workout page: time, pace, elevation change and average HR per mile (or km), computed in the frontend from the series by interpolating where each mile falls, so nothing new is stored.
- [x] **Graphs** in the frontend: pace, elevation and heart rate overlaid on one graph (an axis each, toggles) over distance or time; a segment or split highlighted on it; a distance's best-effort trend over time.
- [x] **Maps.** The GPS track from the series drawn with Leaflet on OpenStreetMap tiles, with segments highlighted. Tiles come from openstreetmap.org, so the map's area is visible to the tile server.
- [x] Compare view: overlay workouts' pace graphs aligned by distance.
- [x] Workout page laid out as a desktop dashboard: map top left, splits top right, the graph along the bottom, sized to the screen.
- [x] **Best efforts by time**: the farthest stretch in 30 minutes and an hour, stored as `kind = duration` segments beside the distance ones and ranked by distance; they show on the workout page, best efforts and compare. More durations (20 minutes, 2 hours) are one line in `StandardDurations`, and reprocessing backfills them.
- [x] **Quieter map** (feedback 2026-11-11). The map took too much of the dashboard and the OSM style competed with the track.
  - The map gets less of the grid (3:2 columns, half the height), so splits and graphs get more room.
  - The OSM tiles are greyed by a CSS filter (inverted in dark mode), so road colours drop out. CARTO Positron was the first choice, but its tiles now need an API key.
  - The track is drawn at full opacity over a dark casing, with larger start/finish markers.
  - Still open: a self-hosted vector style (PMTiles + MapLibre) would give full control of the basemap, and would also keep the map's area away from a tile server.
- [x] **Grafana-style graph legend.** The colour swatches and checkbox toggles above the graph are gone; the legend under it lists each series with its colour and value at the cursor, and clicking an entry shows or hides that series.
- [x] **Years on time axes** (feedback 2026-11-12). The date axes on the best-effort pace trend and a route's runs labelled ticks with month and day only, so the year showed only in the hover readout. Now the first tick and each new year's first carry the year on a second line (`timeAxis` in the frontend's `chart.ts`); month steps drop the day, and year steps show just the year.

### Series sampling: today, and upgrades for when they're needed

**Today.**
- `BuildSeries` keeps every Nth record (an even stride) so a workout has at most 2000 points, always keeping the last; nothing is averaged. At 1 record/s a one-hour run keeps every 2nd record, a marathon about every 7th.
- Values are rounded per column (time 0.1s, speed 0.001 m/s, altitude 0.1m, HR whole bpm, lat/long 6 decimals ≈ 10cm).
- Segments and workout totals use every record in the file; only graphs, the map and splits see the thinned series.
- The thinned series is stored (`workout_series`); full-resolution records are not, and the FIT file is reread only on reprocessing.
- The frontend plots the series as is: pace smoothed over a trailing 200m (gaps slower than 30 min/mi), splits interpolated at exact mile/km marks (good to about one stored point, a few seconds on a long run), elevation gain with 1m hysteresis, compare overlays resampled onto a shared distance grid.

**Upgrades, each with what would call for it.** Bumping `extractorVersion` rebuilds every workout from its file, so changing the method is cheap.
- [ ] **Peak-preserving downsampling** (LTTB, or min/max per bucket) instead of a plain stride, which can drop a short HR spike or sprint and makes the shape depend on where the stride falls. Needed for interval sessions, M7 HR zones and max HR read from the series, or zooming into a short stretch of a long run.
- [ ] **Shape-aware track simplification** (Douglas–Peucker / Visvalingam) for lat/long, keeping corners sharp with fewer points. M9 route matching resamples evenly by distance anyway, so this is only for drawing.
- [ ] **Detail on zoom:** keep a full-resolution (or second, denser) series and fetch a time window of it when the graph is zoomed. Only once second-by-second analysis is wanted; it costs storage roughly in proportion to recording rate.
- [ ] **Splits on the backend** from every record, stored like segments. Needed if splits appear where they must be exact (records pages, comparisons across runs, trends) or very long runs make the stride coarse.
- [ ] **Better elevation:** smoothing, or correction against a terrain model. This moves gain/loss more than sampling does; worth it when elevation numbers start to matter.

## M7 — Heart rate (lower priority)

- [ ] Average and max HR per workout from the FIT session (`avg_heart_rate`, `max_heart_rate`); HR is already in the M6 series. Old workouts pick these up through M5 reprocessing.
- [ ] A per-user max HR (and optionally resting HR) setting, defaulting to the highest HR seen across the user's workouts.
- [ ] Time in HR zones per workout, derived from that setting. Changing the setting enqueues stats jobs rather than reprocessing files.

## M8 — Automatic import (exploration)

Today every run is a manual FIT export from COROS, then `taco upload` or the browser.
The goal is new runs showing up on their own. Each option still needs checking against current docs and terms before building.

- [ ] **Survey the routes**, cheapest first:
  - **Via intervals.icu.** COROS already auto-syncs to intervals.icu, and its API returns the original uploaded file (`GET /api/v1/activity/{id}/file`, gzipped FIT) with a personal API key. Works for a single user today, at the cost of a third party in the path.
  - **COROS self-service developer access.** Reportedly OAuth 2.0 for personal apps with no approval step. Unconfirmed: whether it can download FIT files, and what the limits are.
  - **COROS Partner API.** Webhook push and FIT downloads, but it needs an application to api@coros.com and is aimed at established platforms. Probably not worth it for personal use.
  - **No API:** a bulk import of a COROS data export (t.coros.com), and possibly a watched folder that a phone or desktop sync drops files into.
- [ ] **A source abstraction.** Each user connection (kind, credentials, cursor) gets a periodic `sync_source` job on the M5 queue that fetches new activities and feeds them through the normal upload path.
  Content-hash dedupe already makes re-fetching harmless. Also store the source's activity ID, to skip downloads that would only dedupe.
- [ ] Credentials stored encrypted at rest, revocable per connection, and never logged.
- [x] **Endomondo archive import.** Endomondo shut down in 2020; its export has a `.tcx` and a `.json` per workout. Surveyed on a 405-workout export (2016–2020):
  - **Sport needs the JSON.** TCX says `Running` for 202 runs, `Biking` for 1 ride and `Other` for everything else, walks (83) and treadmill runs (23) included. The JSON's `sport` (`RUNNING`, `WALKING`, `TREADMILL_RUNNING`, `CYCLING_SPORT`, `SPINNING`, `STAIR_CLIMBING`, `WEIGHT_TRAINING`, …) maps onto FIT names, e.g. `TREADMILL_RUNNING` → running / treadmill.
  - **Points are in both.** Time, position, cumulative distance and altitude every 1–5s, and heart rate (`heart_rate_bpm`) where a strap was worn. TCX times are ISO UTC; JSON point timestamps switch between `"Thu Jul 23 22:20:14 UTC 2020"` and `"2020-12-07 20:44:46.0"`.
  - **GPS:** 280 of 285 runs and walks have a track. Treadmill, spinning, stairs and weights have 1–5 placeholder points, and 14 are hand-entered (`source: INPUT_MANUAL*`): no points, just a distance and duration.
  - **Broken files exist:** one run has 16 points over 120m against a stated 4.02 km (GPS lost, distance typed in), and its points jump back an hour after the first. Several 2016 runs' points carry on minutes past `end_time`, which is the start plus moving time, so that's a pause, not a fault.
  - 52 workouts carry a note (`message`); photo URLs point at Endomondo's dead servers.
  - **Built:** upload the `.json` files (`taco upload Workouts/*.json`, or the browser). `workouts.ReadActivityFile` tells FIT from Endomondo JSON by content and reads either into the same `workouts.Run`, so storage, reprocessing, segments and routes work unchanged. Untrustworthy files lean towards not counting: hand-entered, no distance, time running backwards, or points that disagree with the totals keep their stated totals but no points, and an `excluded_reason`. On the export that leaves 198 of 202 runs and 80 of 83 walks counted.
  - [ ] Notes (`message`) aren't kept; workouts have nowhere to put them yet.
  - [ ] Content dedupe won't catch a run that's also in a FIT export; match on start time and sport. Not needed for this archive, which ends before the COROS history starts.
- [x] **Each workout's source** (`source`): what recorded it, from the FIT `file_id` (its product name, e.g. `COROS PACE 3`, else the manufacturer and Garmin product), or `Endomondo`. Reprocessing fills it in for old workouts.
- [x] **Google Fit Takeout import.** Surveyed on a 3,427-session export (`Fit/Activities`, 2016–2026, TCX only):
  - **No app name.** Every file's author is `Google Fit`; Takeout's "All sessions" JSON would name the recording app, but isn't needed to find duplicates.
  - **One file per session,** made of the stretches Google Fit detected (`Walking`, `Running`, `Biking`, ...), each its own `<Activity>` restarting its distance. 367 files mix walking and running, often with a stray point hours before the start and time running backwards.
  - **Mostly noise.** 2,380 walks and 264 rides are step- or motion-detected (2021–23 walks average 0.7 mi in 18 min, most with under 20 GPS fixes a mile). Tracked runs have 200+ fixes a mile.
  - **Duplicates:** 271 sessions copy Endomondo workouts (117 of them over 10% short) and 80 copy COROS ones; 289 of the 351 start within 2 minutes of the original.
  - **Per-point distance is unusable:** it updates every half minute or so, or sits at the session total from the second point. Sessions are padded with fixless points out to the hour (a 25-minute run reads as 1:00:00). Positions are fine, so points are measured along the GPS track and the time is the fixes' span.
  - **Track vs. total:** on 109 of 122 runs the track is within 10% of the stated distance. 2026 phone runs jump 100–260m between fixes and overshoot by 20–58%; one 2023 run has a clean track 17% over its total. Disagreement excludes the run rather than guessing which is right.
  - [ ] **A short track can mean an inflated total.** A track only cuts corners, so with dense fixes it can under-measure but not over-measure. The 2023-04-25 run's phone took fixes every 5–35s for its first 3 minutes: 600m of track against Google Fit's 997m (a 4:40/mi pace), then both agree at 9:00/mi. Its total reads 2.96 mi at 8:10/mi, really about 2.7 mi, and it tops the 1-mile board; the track is only 7.6% short, so the 10% check passes it. Idea: when the track is dense and shorter than the total, use the track's distance. Check first how many runs start with sparse fixes.
  - **Built:** `workouts.ReadTCXFile`. A scored session that mixes sports, has time running backwards, has under 50 fixes a mile, or whose track strays from its total gets an `excluded_reason`. First pass uploaded only the 122 tracked runs that overlap nothing already stored (113 from 2021–2023, between Endomondo and COROS), so no duplicates went in; 109 count.
  - [ ] **Cross-source duplicates:** a per-user job (like `detect_routes`) pairs workouts of one sport whose starts are minutes apart and whose spans mostly overlap. The richer source wins (watch FIT, then Endomondo, then Google Fit); the other copy stays listed with an `excluded_reason` naming the winner, so deleting the winner brings it back.

## M9 — Routes

Most runs repeat a handful of routes. The goal: group runs on the same route automatically, so a route's runs can be compared on total time and its history shown as a trend.
Builds on the M6 series (positions); everything is per user, never matched across users.

- [x] **Detecting a route match** (`workouts.SameRoute`): starts and ends within 150m, lengths within 10%, then a discrete Fréchet distance under 75m between tracks resampled every 20m by distance. Only runs and walks, each sport on its own. Fréchet respects order, so a loop run clockwise and anticlockwise are separate routes; an out-and-back retraces itself and matches either way. Tuned on a two-year export of 255 GPS activities: 50m split routes that GPS drift had nudged apart, 75m puts 107 of 134 runs on 16 routes, and running the same loop the other way round still lands 400m+ away.
- [x] **Clusters.** Each run, oldest first, joins the route whose first run it matches most closely, or waits as a candidate until a second run matches it; then routes whose representatives (the member closest to the rest) match are merged, since an off first run can split one route in two.
- [x] **As derived data:** a per-user `detect_routes` job on the M5 queue, after a workout is created, reprocessed or deleted, regroups from scratch. A rebuilt group keeps the id and name of the old route it overlaps most.
- [ ] **A coarse index** (the set of ~150m grid cells a track passes through, compared by overlap) to find candidate routes without comparing against every route's first run. Not needed at hundreds of runs: most pairs fail the endpoint check.
- [ ] **Variations.**
  - Small detours (a block around construction, GPS drift) fall inside the tolerance and still match.
  - Extensions (the usual loop plus an extra mile) cover most of the route but add length: match them as a variant of the route, flagged, and keep them out of the route's total-time comparison. Today they fail the length check and become a route of their own.
  - Truly different runs become their own route. Partial overlap between routes is what sub-route segments (Strava-style, user-drawn or detected from shared stretches) would handle; that's a later step.
- [ ] **User control beats detection:** naming is done (`PUT /routes/{id}`). Still to do: merge two routes, move a run to another route or detach it, mark "not this route". Those decisions are stored separately and survive re-detection. A route that dissolves (down to one run) loses its name today; overrides would fix that too.
- [x] Frontend: a routes list, a route page (map of the representative track, its runs and a time trend), and the route shown on each workout.

## M10 — Corrections

The file is the source of truth, but files are sometimes wrong: a ride saved as a walk, a GPS glitch, a run that shouldn't count. Corrections are stored beside the derived data and survive reprocessing.

- [x] **Per-sport plausibility cap.** Efforts faster than a sport could manage (a walking mile at 3:38) aren't stored as segments, so a mislabeled activity or a GPS jump can't top a leaderboard. The caps (`maxSpeedMps`, next to `ScoredSports`) are about the 1k world record for running and the race-walking mile record for walking.
- [x] **Exclude from stats.** A per-workout reason that keeps it listed but out of best efforts, route comparisons and trends (a run with a friend, a watch left running in the car): `user_excluded_reason`, set with `PUT /workouts/{id}/corrections` and applied over the file on every reprocess. Processing's own `excluded_reason`, for untrustworthy Endomondo and Google Fit files, wins over it.
  - First case, to exclude once this is deployed: the 2023-04-25 Google Fit run (test stack workout 739), whose total is inflated by a sparse GPS start (see M8).
- [x] **Edit a workout's sport**: `sport_override`, set the same way, wins over the file's sport, and changing it reprocesses the workout and requeues route detection, so segments and routes follow. The file's sub-sport stays, so a treadmill run corrected to a walk is still indoors.
- [ ] Other fields the file got wrong, as more override columns applied the same way, once there's a case for one.
- [x] Frontend: the sport and exclusion controls on the workout page ("Correct"), with a corrected sport marked and an exclusion shown as "Counted: No".
- [x] **Indoor runs aren't scored.** Treadmill, `indoor_running` and `indoor_walking` sessions measure distance with a belt or foot pod, so they get no segments and no track (`RunSummary.Scored`, #65).
- [ ] Route changes (rename is done; merge, move, detach, "not this route") are M9's user control.

## M11 — Time windows

Years of history (an Endomondo archive goes back to 2016) make an all-time list long and all-time bests stale.

- [x] **Restrict the workout list by time.** `GET /workouts?from=&to=&limit=&offset=`, and `GET /workouts/totals` for a window's count, distance and time per sport (counted workouts only) with the earliest start, so the frontend can list the years there are. The frontend opens on this year, keeps the period in the URL (`?period=`), and shows its totals above the list; a year narrows to a month.
- [x] **Mileage per week or month.** `GET /workouts/totals/week` and `/month` sum each period of a window per sport, at the viewer's local midnights (`?tz=`), empty periods included so a chart has no gaps. The workout list charts it above the totals: weeks for a year or a month, months for all time.
- [x] **Best efforts per year or window.** The same `from`/`to` on `GET /segments/{name}/best`. The best page picks all time, the last 12 months, the last 90 days or a year, and under all time shows every distance's best in each year from one call, `GET /segments/yearly-bests`.
- [x] **Route pages take the same window**, so a route's best can be this year's: `?from=&to=` on `GET /routes` and `GET /routes/{id}`, and a period picker on both pages.

## Later

- **Faster CI.** Backend runs take 9–13 minutes on every push: `nix flake check` about 4 (formatting, lint, tests), then a second full build of the binary about 3½, then the image publish about 1; the frontend takes about 2. Ideas, roughly in order of payoff:
  - Build the binary once and have the checks reuse it, or drop the separate build step when the flake check already compiled the same derivation.
  - Make sure the Nix cache actually hits for Go modules (a vendored-dependencies derivation that only changes with `go.mod`/`gomod2nix.toml`), so a code change doesn't rebuild dependencies.
  - Run formatting, lint and tests as parallel jobs, so a formatting failure reports in seconds.
  - Skip the image build and publish on PRs, or only on `master` and tags; homelab deploys only use `master-<sha>`.
- `taco compare` / `taco best <distance>` in the CLI.
- **Admin panel** in the frontend, for running the deployment rather than looking at runs: the job queue (queued, running, dead jobs with their last error, retry), reprocess one or all workouts, files that failed to parse, and eventually account management in place of `kubectl exec … useradd`. Needs a notion of an admin user first.
