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

- [ ] Config: `auth.*` section, provider list, secrets read from files; refuse to start if a provider's discovery fails at boot (log which).
- [ ] Migration `000004_user_identities` (postgres + sqlite) and sqlc queries.
- [ ] `internal/oidc`: provider registry from config, login redirect, callback handling, state/nonce/PKCE cookie.
- [ ] Routes: `/auth/providers`, `/auth/oidc/{provider}/{login,callback}`, `/auth/identities` (list, link, unlink).
- [ ] Tests against an in-process fake provider (e.g. `github.com/oauth2-proxy/mockoidc`): happy path, bad `state`, bad `nonce`, wrong audience, expired token, signup disabled, email auto-link on and off, `email_verified: false`.
- [ ] Frontend: provider buttons on the login page from `/auth/providers`, a `#/login/callback` route that stores the token, and a linked-accounts section.
- [ ] Docs: an authentik walkthrough (OAuth2/OpenID provider + application, redirect URI, scopes) as the worked example, plus the generic settings any provider needs.

**Exit criteria:** with only an authentik provider configured, a new user can sign in from the frontend and use every page; an existing password user can link authentik and then sign in either way; swapping in a second provider (e.g. Keycloak) needs config only.

---

## M2 — Upload hygiene

- [x] Unparsable FIT files and multi-session files (e.g. triathlons) return **422** with a reason, not 500.
- [x] Don't leave orphan `activity_file` rows when parsing fails (or expose them so they can be retried or deleted). Exposed via `GET /activity_files?without_workout=true`; kept rather than deleted so multi-sport files can be processed once supported.
- [x] Deduplicate uploads: the same file (same content hash, already computed) for the same user returns the existing workout instead of creating a second one.
- [x] Backfill segments for workouts created before `workout_segments` existed: `POST /segments/recompute` (all) and `POST /workouts/{id}/segments/recompute`.
- [ ] Frontend and `taco`: say "already uploaded" on a `200` re-upload, offer a backfill action, and list files that failed to parse.

## M3 — Deploy

- [ ] Run backend + frontend images together. Test stack at `taiga.i.hruday.me` on the homelab k8s cluster (CNPG Postgres, local files PVC); manifests and runbook live in proxmox-homelab (`k8s-manifests/taiga`, `docs/taiga.md`), not here.
- [x] Env-only configuration: every config key binds to its `TAIGA_*` variable, and the image starts the server by default.
- [x] A production CORS story: none needed, because the frontend's `/api` proxy is the only browser entry point and `TAIGA_APP_ENVIRONMENT=production` keeps the dev-only wildcard CORS off.

### Before any public access

The test stack is internal-only (LAN and tailnet). These must land before taiga is reachable from the internet.

- [ ] **Closable registration.** `POST /auth/register` is always open today, so anyone who can reach the API can make an account.
  - `auth.registration: open | closed` (env `TAIGA_AUTH_REGISTRATION`); closed returns 403 from `/auth/register`.
  - Default closed outside development, so a prod deploy is closed unless it opts in.
  - `cmd/useradd` to create accounts while registration is closed (run via `kubectl exec`).
  - The frontend hides its register link based on `/auth/providers` (M1), but the 403 is the real control.
  - Separate from M1's `auth.password_login: false`, which disables password login entirely. With OIDC, `closed` plus a provider's `allow_signup: false` gives invite-only through authentik.

## M4 — Cleanup

- [ ] Remove the `hats` template code and its table.
- [ ] Retire or refresh `terraform/` (AWS ECS + RDS): the homelab stack replaces it as the deploy path, and it predates env-only config.
- [ ] Fix CLAUDE.md drift (it documents `/auth/login`; the endpoint is `/auth/tokens`).
- [ ] Tests for `internal/tokens`, `internal/store`, `internal/filerepo`.

## M5 — Reprocessing and a work queue

The FIT file is the source of truth; everything taiga computes from it (summary, segments, and later series and heart rate) is derived data that should be rebuildable whenever the app learns something new.
`POST /segments/recompute` is a one-off version of this; M5 generalises it.

- [ ] **Versioned processing.** A `processor_version` on each workout, bumped in code whenever extraction changes (new fields, a segment algorithm fix). Workouts below the current version are stale.
- [ ] **Reprocess from the file.** Re-read the FIT file and rebuild every derived table for a workout in one place, replacing `RecomputeSegments`.
  Decide what happens to fields a user edited through `PUT /workouts/{id}`: either reprocessing never overwrites them, or edits move to separate override columns.
- [ ] **A job queue in Postgres**, not new infrastructure: a `jobs` table claimed with `SELECT … FOR UPDATE SKIP LOCKED`, and a worker loop in the server process (SQLite in dev runs a single worker).
  - Job kinds to start: `process_workout`, `reprocess_stale` (enqueues one `process_workout` per stale workout on boot or on demand), and `update_stats`.
  - Retries with backoff, and a dead state with the last error, visible through the API.
- [ ] **Async uploads.** Creating a workout enqueues `process_workout` and returns `202` with a job to poll, so a large file doesn't hold a request open; the frontend and `taco` show progress.
  Keep the 422 for files that can't be decoded at all, checked up front, so obviously bad uploads still fail fast.
- [ ] **Stats as derived data.** Anything aggregated across workouts (weekly/monthly totals, best-effort history) is recomputed by `update_stats` jobs after processing, not on every read.

## M6 — Visualisations

- [ ] **Per-record series.** Store a downsampled series per workout at processing time (time, distance, pace, elevation, heart rate, lat/long), as a derived table or one compact blob per workout, rebuilt by M5.
  Serve it from `GET /workouts/{id}/series`. Don't re-parse the FIT file on every page view.
- [ ] **Graphs** in the frontend: pace, elevation and heart rate over distance or time for one workout; the best segments from `workout_segments` highlighted on the pace graph; a distance's best-effort trend over time.
- [ ] **Maps.** The GPS track from the series (FIT stores positions in semicircles; convert at processing time), drawn with Leaflet on OpenStreetMap tiles, with segments highlighted.
- [ ] Compare view: overlay two workouts' pace graphs aligned by distance.

## M7 — Heart rate (lower priority)

- [ ] Average and max HR per workout from the FIT session (`avg_heart_rate`, `max_heart_rate`), and HR in the M6 series. Old workouts pick these up through M5 reprocessing.
- [ ] A per-user max HR (and optionally resting HR) setting, defaulting to the highest HR seen across the user's workouts.
- [ ] Time in HR zones per workout, derived from that setting. Changing the setting enqueues stats jobs rather than reprocessing files.

## Later

- `taco compare` / `taco best <distance>` in the CLI.
