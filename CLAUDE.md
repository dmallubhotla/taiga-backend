# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

This is a modern Go web API template with PostgreSQL/SQLite support, JWT authentication, file handling, and workout data processing. The application follows clean architecture principles with clear separation of concerns.

### Core Components

- **cmd/server**: Application entrypoint with graceful shutdown and automatic migrations
- **cmd/migrate**: Standalone migration tool with up/down/steps/goto commands
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
```

### JWT Key Generation (Development Only)
```bash
just generate_keypair    # Generate RSA keypair for JWT signing
```

### Utility Commands
```bash
go run cmd/fit_view/main.go [file.fit]  # View FIT workout files
go run cmd/migrate/main.go [options]    # Manual database migrations
```

## Project Structure

- **Go 1.25** with modern module structure
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

## Database Operations

### Adding New Queries

1. **Write SQL in `queries/` directory**:
   ```sql
   -- name: GetUser :one
   SELECT * FROM users WHERE id = $1;
   ```

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
- Located in `migrations/` directory
- Naming convention: `NNNNNN_description.up.sql` and `NNNNNN_description.down.sql`
- Example: `000001_create_users_table.up.sql`, `000001_create_users_table.down.sql`

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
- Migrations run automatically on server startup when `migration.auto_up: true` (default)
- In development, can auto-rollback on shutdown with `migration.auto_down: true`

#### Creating New Migrations

1. **Create up migration:**
   ```sql
   -- migrations/000004_add_posts_comments.up.sql
   CREATE TABLE comments (
       id SERIAL PRIMARY KEY,
       post_id INTEGER REFERENCES posts(id),
       content TEXT NOT NULL,
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
   );
   ```

2. **Create down migration:**
   ```sql
   -- migrations/000004_add_posts_comments.down.sql
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
  token_key: "token"

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
  asset_path: "local/filerepo/"      # content-addressable file storage
  prefix_length: 2                   # directory structure depth
```

### Environment Variables
- Prefix: `TRYGO_`
- Examples: `TRYGO_APP_PORT`, `TRYGO_DB_DRIVER`, `TRYGO_DB_HOST`, `TRYGO_TOKENS_PRIVATE_KEY_PATH`
- Override any config file setting using dot notation converted to underscores

## API Endpoints

### Public Endpoints
- `GET /` - Hello World with service info
- `GET /ping` - Simple ping/pong response
- `GET /health` - Health check with database connectivity status
- `POST /auth/register` - User registration
- `POST /auth/login` - User login (returns JWT token)

### Protected Endpoints (require JWT token)
- `GET /hats` - List all hats
- `POST /hats` - Create new hat
- `GET /hats/{id}` - Get specific hat
- `PUT /hats/{id}` - Update hat
- `DELETE /hats/{id}` - Delete hat

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

Switch by updating `config.yaml` or setting `TRYGO_DB_DRIVER` environment variable.

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
- Consistent formatting and building across machines