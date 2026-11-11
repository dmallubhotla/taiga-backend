# taiga-backend

The API behind taiga, for seeing and comparing runs: upload FIT files, and it derives workouts, fastest segments at standard distances, and per-record series (pace, elevation, heart rate, GPS) for the [browser frontend](https://github.com/dmallubhotla/taiga-frontend) and the `taco` CLI.

Go with chi, sqlc and golang-migrate; PostgreSQL in production, SQLite in development.

## Quick start

```bash
nix develop               # dev shell with Go, sqlc, just
just generate_keypair     # first time only: RSA keys for JWT signing
just serve                # http://localhost:8080 on SQLite (./data.db)
```

Registration is open in development: `POST /auth/register`, then `POST /auth/tokens` for a token.
Outside development it's closed by default; create accounts with `go run cmd/useradd/main.go -email you@example.com`.

## Configuration

`config.yaml`, overridable by environment variables prefixed `TAIGA_`, e.g. `TAIGA_APP_PORT`, `TAIGA_DB_DRIVER=postgres`, `TAIGA_DB_HOST`, `TAIGA_AUTH_REGISTRATION=open`.
Every config field is bound, so a deployment can be configured by environment alone.

## More

- [CLAUDE.md](CLAUDE.md): architecture, commands, configuration and the full API.
- [ROADMAP.md](ROADMAP.md): what's done and what's next.
