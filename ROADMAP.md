# taiga roadmap

The goal is seeing and comparing runs in a browser.
Pieces: this API, the browser frontend (`taiga-frontend`), and the `taco` CLI (`taiga-cliclient`) for scripting.
Milestones are roughly ordered; each should fit in one PR per repo.

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

- [ ] Run backend + frontend images together (homelab k8s or the existing AWS terraform), with Postgres and the S3 file repo.
- [ ] A production CORS story: none needed if the frontend's `/api` proxy is the only browser entry point; keep the dev-only wildcard CORS out of production.

## M4 — Cleanup

- [ ] Remove the `hats` template code and its table.
- [ ] Fix CLAUDE.md drift (it documents `/auth/login`; the endpoint is `/auth/tokens`).
- [ ] Tests for `internal/tokens`, `internal/store`, `internal/filerepo`.

## Later

- `taco compare` / `taco best <distance>` in the CLI.
- Charts in the frontend: pace over a workout, a distance's best-effort trend over time.
- Per-record data (pace/HR over time) exposed by the API for those charts.
