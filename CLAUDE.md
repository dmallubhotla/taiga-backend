# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

This is the taiga API: upload FIT files, derive workouts, segments and per-record series from them, and serve them to the browser frontend and the `taco` CLI. PostgreSQL in production, SQLite in development, JWT auth. The application follows clean architecture principles with clear separation of concerns.

### Core Components

- **cmd/server**: Application entrypoint with graceful shutdown and automatic migrations
- **cmd/migrate**: Standalone migration tool with up/down/steps/goto commands
- **cmd/useradd**: Create an account while registration is closed
- **cmd/fit_view**: Utility for viewing FIT (Garmin) workout files

- **internal/config**: Configuration management with Viper
  - YAML config files and environment variable support
  - Database driver switching (PostgreSQL/SQLite)
  - JWT token configuration for RSA keys
  - File repository configuration
  - Development/production environment detection

- **internal/routes**: HTTP request handlers and routing
  - Auth endpoints with JWT token generation/validation
  - Health checks with database connectivity
  - Hat management (example CRUD operations)
  - Activity files API for secure file upload/download
  - Protected routes using JWT middleware

- **internal/server**: HTTP server setup and middleware
  - Chi router with standard middleware
  - CORS support for development
  - Connection pooling and timeouts

- **internal/store**: Database abstraction layer
  - PostgreSQL and SQLite implementations
  - Error handling and connection management
  - Interface-based design for testability

- **internal/db**: Generated sqlc code (after running `sqlc generate`)
  - Type-safe database operations
  - Compiled SQL queries with Go interfaces

- **internal/tokens**: JWT token management
  - RSA key-based signing and verification
  - Middleware for route authentication
  - Token generation and validation

- **internal/filerepo**: Content-addressable file storage
  - SHA-256 based file organization
  - Configurable prefix length for directory structure

- **internal/workouts**: Garmin FIT file processing
  - FIT file parsing and workout data extraction
  - Segment analysis and activity processing
  - `BuildSeries` (per-record columns, thinned to `MaxSeriesPoints`) and `ProcessorVersion`

- **internal/models**: The model layer handlers call; processing a file into a workout (`processing.go`) and the job queue (`jobs.go`)

- **internal/jobs**: The in-process worker that drains the `jobs` table (see Reprocessing below)

- **internal/migration**: golang-migrate wrapper used on startup and by `cmd/migrate`

- **internal/testsupport**: Real dependencies for integration tests (see Testing below)

## Development Commands

### Setup and Code Generation
```bash
go mod tidy              # Install dependencies
just chores              # Run gomod2nix and sqlc generate
sqlc generate           # Generate database code from SQL queries
```

### Running the Application
```bash
just serve               # Run server with go run (recommended)
go run cmd/server/main.go # Run with SQLite (default)
just build               # Build using nix
nix build                # Alternative nix build
```

### Testing and Development
```bash
just test                # Run full test suite with coverage
just full_test           # Run tests + sqlc vet + sqlc diff
just vet_sqlc            # Validate sqlc configuration
just fmt                 # Format code using nix
just update              # nix flake update
```

### Releasing (hanko)
The project uses [hanko](https://github.com/dmallubhotla/hanko) `seal` to bump
the version stamped in `flake.nix`, create the release commit, tag, and push.
Configuration lives in `.hanko.yaml`.

```bash
just release-plan        # Preview what `just release` would do (dry run)
just release             # Stamp version, commit, tag, push
```

A `seal` GitHub workflow (`.github/workflows/seal.yml`) does the same thing on
`workflow_dispatch`, signed by a GitHub App token (secrets:
`RELEASE_APP_CLIENT_ID`, `RELEASE_APP_PRIVATE_KEY`).

### JWT Key Generation (Development Only)
```bash
just generate_keypair    # Generate RSA keypair for JWT signing
```

### Docker Commands
```bash
just build-docker        # Build Docker image with nix
just load-docker         # Load Docker image into Docker daemon
just build-load          # Build and load Docker image
just exec-docker         # Run interactive shell in Docker container
```

### Utility Commands
```bash
go run cmd/fit_view/main.go [file.fit]  # View FIT workout files
go run cmd/migrate/main.go [options]    # Manual database migrations
go run cmd/useradd/main.go -email you@example.com  # Create an account while registration is closed (prompts for the password; in the cluster: kubectl -n taiga exec -it deploy/taiga-backend -- useradd -email ...)
```

## Project Structure

- **Go 1.26** with modern module structure
- **sqlc** for type-safe SQL code generation
- **golang-migrate** for database migrations
- **Chi v5** router with middleware support
- **Viper** for configuration management
- **JWT** with RSA key authentication
- **PostgreSQL** (production) and **SQLite** (development) support
- **Nix flakes** for reproducible development environment
- **Just** task runner for common operations
- **Content-addressable file storage** for binary assets
- **FIT file processing** for Garmin workout data
- **Docker deployment** with layered image building

## Database Operations

### Adding New Queries

1. **Write SQL in `queries/` directory**:
   ```sql
   -- name: GetUser :one
   SELECT * FROM users WHERE id = $1;
   ```

   Name parameters with `sqlc.arg(name)`, not `@name`: `just fmt` rewrites `@name` to `@ name`, which sqlc can't parse.
   `sqlc.embed(table)` breaks the same way (`sqlc.embed (table)`), so fetch the rows separately and join them in Go.

2. **Generate Go code**:
   ```bash
   sqlc generate
   ```

3. **Use in handlers**:
   ```go
   user, err := queries.GetUser(ctx, userID)
   ```

### Database Migrations

The project uses **golang-migrate** for database schema management with proper up and down migrations.

#### Migration Files
- Two directories with the same numbering: `migrations/` (PostgreSQL) and `migrations-sqlite/` (SQLite); every migration needs both
- Naming convention: `NNNNNN_description.up.sql` and `NNNNNN_description.down.sql`
- Use the next free number after the highest existing one

#### Migration Commands

**Using the CLI tool:**
```bash
# Run all up migrations
go run cmd/migrate/main.go -command=up

# Rollback all migrations
go run cmd/migrate/main.go -command=down

# Check current version
go run cmd/migrate/main.go -command=version

# Step up/down by N migrations
go run cmd/migrate/main.go -command=steps -steps=1   # up 1
go run cmd/migrate/main.go -command=steps -steps=-1  # down 1

# Migrate to specific version
go run cmd/migrate/main.go -command=goto -version=2
```

**Automatic migrations:**
- Migrations run automatically on server startup when `db.auto_migrate_up: true` (default)
- `db.drop_on_start: true` (development only) drops every table on startup

#### Creating New Migrations

1. **Create up migration:**
   ```sql
   -- migrations/NNNNNN_add_comments.up.sql (and the same in migrations-sqlite/)
   CREATE TABLE comments (
       id SERIAL PRIMARY KEY,
       post_id INTEGER REFERENCES posts(id),
       content TEXT NOT NULL,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
   );
   ```

2. **Create down migration:**
   ```sql
   -- migrations/NNNNNN_add_comments.down.sql
   DROP TABLE IF EXISTS comments;
   ```

3. **Run migrations:**
   ```bash
   go run cmd/migrate/main.go -command=up
   ```

## Configuration

### Config File (`config.yaml`)
```yaml
app:
  port: "8080"
  environment: "development"

auth:
  registration: "open"             # "open" or "closed"; defaults to open only in development

db:
  driver: "sqlite"                 # "sqlite" or "postgres"
  filepath: "./data.db"            # for sqlite
  migration_path: "./migrations-sqlite"
  drop_on_start: false             # dev only - drops all tables on start
  auto_migrate_up: true            # automatically run migrations on startup

tokens:
  public_key_path: "cert/test.pem"   # RSA public key for JWT verification
  private_key_path: "cert/test.key"  # RSA private key for JWT signing

file_repo:
  type: "local"                      # "local" or "s3" (s3_bucket, s3_region, s3_prefix)
  asset_path: "local/filerepo/"      # content-addressable file storage
  prefix_length: 2                   # directory structure depth
```

### Environment Variables
- Prefix: `TAIGA_`
- Examples: `TAIGA_APP_PORT`, `TAIGA_DB_DRIVER`, `TAIGA_DB_HOST`, `TAIGA_TOKENS_PRIVATE_KEY_PATH`
- Override any config file setting using dot notation converted to underscores, e.g. `db.migration_path` → `TAIGA_DB_MIGRATION_PATH`; every field in `config.Config` is bound, so a deployment can be configured by env alone

## API Endpoints

### Public Endpoints
- `GET /` - Hello World with service info
- `GET /ping` - Simple ping/pong response
- `GET /health` - Health check with database connectivity status
- `POST /auth/register` - User registration; `403` when `auth.registration` is closed (env `TAIGA_AUTH_REGISTRATION=open|closed`, default open in development and closed otherwise)
- `GET /auth/providers` - What the login page should offer: `{password_login, registration_open, providers}` (OIDC providers arrive with ROADMAP M1)
- `POST /auth/tokens` - User login with `{email, password}` (returns a JWT token)

### Protected Endpoints (require JWT token)
- `GET /hats` - List all hats
- `POST /hats` - Create new hat
- `GET /hats/{id}` - Get specific hat
- `PUT /hats/{id}` - Update hat
- `DELETE /hats/{id}` - Delete hat

### Activity Files API (requires JWT token)
- `GET /activity_files` - List all activity files for authenticated user (newest first); `?without_workout=true` lists only files with no workout, e.g. ones that failed to parse
- `GET /activity_files/{id}` - Get specific activity file metadata
- `POST /activity_files` - Upload new activity file (multipart form with `file` and optional `timestamp`); `201` when new, `200` with the existing file when the user already uploaded the same content
- `PUT /activity_files/{id}` - Update activity file metadata (timestamp, file_repo_hash)
- `DELETE /activity_files/{id}` - Delete the activity file record (stored content is shared by hash and kept); `409` while workouts still reference it
- `GET /activity_files/{id}/download` - Download the actual file content

### Workouts and Segments API (requires JWT token)
- `GET /workouts` - List workouts (newest first); `GET /workouts/{id}`, `POST`, `PUT /workouts/{id}`, `DELETE /workouts/{id}` for CRUD. `PUT` only edits hand-entered workouts; one from a file is `422`, since reprocessing owns its summary
- `POST /workouts/from-activity-file/{id}` - Parse an uploaded FIT file into a workout and store its segments; `201` when new, `200` with the existing workout if the file already has one, `404` for an unknown file, `422` with the reason for an unreadable or multi-session file
- `GET /workouts/{id}/segments` - Fastest effort at each standard distance the workout covers, shortest distance first, then the farthest effort for each standard duration it lasts (`kind` is `distance` or `duration`)
- `GET /workouts/{id}/series` - The workout's records as parallel columns (`time_s`, `distance_m`, `speed_mps`, `altitude_m`, `heart_rate`, `lat`, `lon`; null where a sample is missing, a column omitted if never recorded), at most 2000 points, gzipped; `404` for a hand-entered workout
- `POST /workouts/{id}/reprocess` - Rebuild the workout's summary and segments from its file now and return it; `422` for a hand-entered workout with no file
- `POST /workouts/reprocess` - Queue reprocessing of all the user's file-backed workouts; `202` with `{enqueued}`, progress under `/jobs`
- `GET /workouts/compare?ids=1,2` - Up to 10 workouts with segments keyed by name, plus `distances` (the segment names any of them have, durations last)
- `GET /segments` - The standard segments: distances (`1k`, `1mi`, `3k`, `2mi`, `5k`, `5mi`, `10k`, `half`, `marathon`) with `meters`, then durations (`30min`, `1h`) with `seconds`, each with its `kind`
- `GET /segments/{name}/best?limit=N` - The user's best efforts at that segment across workouts (default 10, max 100): fastest for a distance, farthest for a duration

### Routes API (requires JWT token)
- `GET /routes` - The user's routes, most recently run first: `{id, name, representative_workout_id, run_count, distance_miles, best_time_seconds, best_workout_id, latest_start_time}`
- `GET /routes/{id}` - A route with its `workouts` (newest first) and `track`, the representative run's resampled `[lat, lon]` points
- `PUT /routes/{id}` - Rename with `{"name": "..."}` (trimmed, at most 100 characters); an empty or null name clears it
- `GET /workouts/{id}/route` - The route a workout is on; `404` when it has none (no GPS, or nothing else matches it yet)

### Jobs API (requires JWT token)
- `GET /jobs?state=queued|running|done|dead` - The user's 100 newest jobs, optionally in one state; done jobs are pruned after a week
- `GET /jobs/{id}` - One job, with `attempts` and `last_error`
- `POST /jobs/{id}/retry` - Requeue a dead job with fresh attempts; `409` unless it's dead

## Development Workflow

1. **Set up development environment**: `nix develop` (or use existing Nix shell)
2. **Generate keys for JWT**: `just generate_keypair` (first time only)
3. **Create migrations** if changing database schema
4. **Run migrations**: `go run cmd/migrate/main.go -command=up` or use auto-migration
5. **Modify SQL queries** in `queries/`
6. **Run `just chores`** to regenerate sqlc code and update dependencies
7. **Update routes/handlers** to use new database operations
8. **Test with `just serve`** (auto-runs migrations)
9. **Run tests**: `just test` or `just full_test`
10. **Format code**: `just fmt`

## Database Switching

**SQLite (Default)**:
- File-based: `./data.db`
- No external dependencies
- Good for development/testing

**PostgreSQL**:
- Production-ready
- Better concurrency and features
- Use Docker Compose for local development

Switch by updating `config.yaml` or setting `TAIGA_DB_DRIVER` environment variable.

## Special Features

### JWT Authentication
- RSA key-based JWT tokens for stateless authentication
- Middleware protection for routes requiring authentication
- Keys stored in `cert/` directory (generated with `just generate_keypair`)

### Content-Addressable File Storage
- Files stored by SHA-256 hash in `local/filerepo/`
- Configurable directory prefix length for performance
- Deduplication through content addressing

### FIT File Processing
- Garmin FIT workout file parsing with `muktihari/fit` library
- Workout data extraction and segment analysis
- Utility tool (`cmd/fit_view`) for inspecting FIT files

### Dual Database Support
- Seamless switching between SQLite (development) and PostgreSQL (production)
- Separate migration paths for each database type
- Configuration-driven database selection

### Nix Integration
- Full Nix flake for reproducible development environment
- All dependencies declared in `flake.nix`
- Supported systems: `x86_64-linux`, `aarch64-linux`, `aarch64-darwin`, `x86_64-darwin`
- Flake `checks`: `formatting` (treefmt), `golangci-lint`, `go-test` — all run by `nix flake check`
- Consistent formatting and building across machines

### CI / GitHub Actions
- `ci.yml` runs `nix flake check`, builds the binary and Docker image, and
  publishes the image to `ghcr.io/<owner>/taiga` on every push. Tag fan-out is
  delegated to `hanko version docker tags --latest-on-release-tag`:
  - Tagged release (`vX.Y.Z`) → `:X.Y.Z`, `:X.Y`, `:X`, `:latest`, `:master-<sha>`
  - `master` between releases → `:0.Y.0-master.N.<sha>` + `:master-<sha>` (prerelease, no moving tags)
  - Feature branches → `:<semver-prerelease>` + `:<branch>-<sha>` (prerelease, no moving tags)

  Auth uses the workflow's `GITHUB_TOKEN` with `packages: write`. Images are
  pushed directly from the Nix-built `docker-archive` via `skopeo copy` — no
  Docker daemon needed in CI.
- `seal.yml` is a manually-triggered hanko release workflow.

### Docker Deployment
- Layered Docker image built with Nix for minimal size and reproducibility
- Image includes compiled Go binary, migration files, and minimal runtime dependencies
- Tagged as `taiga:latest` by default
- Supports volume mounting for configuration, certificates, database, and file repository
- The image runs `/bin/server` by default, with migrations at `/migrations` (postgres) and `/migrations-sqlite`
- Example run command with volume mounts: `docker run -v ./config.yaml:/workspace/config.yaml -v ./cert:/cert -v ./data.db:/workspace/data.db -v ./filerepo:/filerepo taiga`

## Activity Files API

The application provides a comprehensive API for managing activity files (primarily Garmin FIT files) with secure upload, download, and metadata management.

### Key Features
- **Secure file upload** with 32MB size limit and multipart form support
- **Content-addressable storage** using SHA-256 hashes for deduplication
- **Metadata management** with RFC3339 timestamps and user ownership
- **User isolation** - users can only access their own files

### File Processing
- FIT files are processed using the `muktihari/fit` library for workout data extraction
- `POST /workouts/from-activity-file/{id}` parses the file once, storing the workout summary and its fastest segments (`workout_segments` table) for each standard distance in `workouts.StandardDistances`, plus the farthest stretch for each duration in `workouts.StandardDurations` (`kind = duration`, same table), in one transaction
- Segments are measured between recorded points, so a segment can overshoot its distance (or duration) by up to one record interval
- Download returns the stored bytes untouched
- File content is immutable once stored, ensuring data integrity

### Reprocessing and the job queue
- The FIT file is the source of truth; a workout's summary, segments and series (`workout_series`, one JSON document per workout) are derived from it and stamped with `workouts.ProcessorVersion`
- The series keeps every Nth record (at most `MaxSeriesPoints`, rounded per column) and is the only copy of per-record data in the database; segments and totals use every record from the file. ROADMAP M6 lists better sampling methods and when each would be needed
- `ProcessorVersion` combines `extractorVersion` (bump it in `internal/workouts/processor.go` whenever extraction changes) with a hash of `StandardDistances` and `StandardDurations`, so adding or removing one reprocesses old workouts on its own
- Processing also stores the run's GPS track resampled every 20m by distance (`workout_tracks`, `workouts.BuildTrack`); a run without GPS gets none
- Routes (`routes`, `workout_routes`) group a user's runs on the same route, never across users. Two runs match when starts and ends are within 150m, lengths within 10%, and the discrete Fréchet distance between tracks is under 50m (`workouts.SameRoute`). Fréchet follows the points in order, so a loop run the other way round is a separate route
- Routes are derived across workouts, so they aren't part of per-workout reprocessing: a new, reprocessed or deleted workout queues one `detect_routes` job per user, which regroups every tracked run from scratch (`workouts.GroupRoutes`, oldest first) and keeps the id and name of the old route each group overlaps most. A run matching nothing isn't a route until a second run matches it
- `internal/jobs` runs a worker goroutine in the server: on boot it queues a `process_workout` job for every workout whose version isn't current, then polls the `jobs` table
- The claim query runs unchanged on both databases (no `SKIP LOCKED`); a claimed job holds a 5-minute lease, so a crashed worker's job is picked up again
- Failures retry with exponential backoff from 30s, up to 5 attempts; errors that can't succeed on retry (unreadable file, missing workout) go straight to `dead`
- Store writes that must land together go through `store.InTx`; SQLite opens transactions `IMMEDIATE` so concurrent writers wait on `busy_timeout` instead of failing

### Testing
- `internal/testsupport` provides real dependencies for integration tests: a migrated file-backed SQLite model, a local file repo, a throwaway RSA toker, and `EncodeFIT` / `EncodeMultiSessionFIT` for synthetic activity files