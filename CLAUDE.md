# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

This is a modern Go web API template with PostgreSQL/SQLite support and type-safe database operations. The application follows clean architecture principles with clear separation of concerns.

### Core Components

- **cmd/server**: Application entrypoint with graceful shutdown
- **internal/config**: Configuration management with Viper
  - YAML config files and environment variable support
  - Database driver switching (PostgreSQL/SQLite)
  - Development/production environment detection

- **internal/handlers**: HTTP request handlers
  - Simple REST endpoints (hello, ping, health)
  - JSON responses with proper error handling
  - Database connectivity health checks

- **internal/server**: HTTP server setup and middleware
  - Chi router with standard middleware
  - CORS support for development
  - Connection pooling and timeouts

- **internal/db**: Generated sqlc code (after running `sqlc generate`)
  - Type-safe database operations
  - Compiled SQL queries with Go interfaces

## Development Commands

### Setup and Code Generation
```bash
go mod tidy              # Install dependencies
sqlc generate           # Generate database code from SQL queries
```

### Running the Application
```bash
go run cmd/server/main.go                    # Run with SQLite (default)
go build -o server cmd/server/main.go       # Build binary
./server                                     # Run binary
```

### Database Development
```bash
docker-compose up -d postgres               # Start PostgreSQL
docker-compose down                         # Stop all services
```

### Testing and Formatting
```bash
just test    # Runs test suite via nix flake check
just fmt     # Formats code using nix fmt
```

## Project Structure

- **Go 1.23** with modern module structure
- **sqlc** for type-safe SQL code generation
- **golang-migrate** for database migrations
- **Chi v5** router with middleware support
- **Viper** for configuration management
- **PostgreSQL** (production) and **SQLite** (development) support

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
port: "8080"
environment: "development"
database:
  driver: "sqlite"        # or "postgres"
  filepath: "./data.db"   # for sqlite

# Migration configuration
migration:
  path: "./migrations"    # path to migration files
  auto_up: true          # automatically run up migrations on startup
  auto_down: false       # automatically run down migrations on shutdown (dev only)
```

### Environment Variables
- Prefix: `TRYGO_`
- Examples: `TRYGO_PORT`, `TRYGO_DB_DRIVER`, `TRYGO_DB_HOST`, `TRYGO_MIGRATION_AUTO_UP`
- Override any config file setting

## API Endpoints

- `GET /` - Hello World with service info
- `GET /ping` - Simple ping/pong response
- `GET /health` - Health check with database connectivity status

## Development Workflow

1. **Create migrations** if changing database schema
2. **Run migrations**: `go run cmd/migrate/main.go -command=up`
3. **Modify SQL queries** in `queries/`
4. **Run `sqlc generate`** to update Go code
5. **Update handlers** to use new database operations
6. **Test with `go run cmd/server/main.go`** (auto-runs migrations)

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