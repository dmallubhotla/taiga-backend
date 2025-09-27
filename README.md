# TryGo - Modern Go Web API Template

A clean, modern Go web API template with PostgreSQL/SQLite support, built with Chi router and sqlc for type-safe database operations.

## Features

- 🚀 Modern Go (1.23+) with clean architecture
- 🌐 Chi router with middleware support
- 🗄️ Database support: PostgreSQL & SQLite
- 🔒 Type-safe SQL with sqlc
- 🐳 Docker & Docker Compose ready
- ⚡ Hot reload support
- 🔧 Environment-based configuration
- 🏥 Health check endpoints

## Quick Start

### Prerequisites

- Go 1.23+
- Docker & Docker Compose (for PostgreSQL)
- sqlc (for generating database code)

### Installation

1. **Clone and setup:**
   ```bash
   git clone <this-repo> my-project
   cd my-project
   go mod tidy
   ```

2. **Install sqlc** (if not already installed):
   ```bash
   go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
   ```

3. **Generate database code:**
   ```bash
   sqlc generate
   ```

### Running with SQLite (Default)

```bash
# Run directly
go run cmd/server/main.go

# Or build and run
go build -o server cmd/server/main.go
./server
```

The server will start on `http://localhost:8080` using SQLite (`./data.db`).

### Running with PostgreSQL

1. **Start PostgreSQL:**
   ```bash
   docker-compose up -d postgres
   ```

2. **Update configuration:**
   Create `config.yaml`:
   ```yaml
   port: "8080"
   environment: "development"
   database:
     driver: "postgres"
     host: "localhost"
     port: "5432"
     user: "trygo"
     password: "password"
     name: "trygo_db"
     sslmode: "disable"
   ```

3. **Run the application:**
   ```bash
   go run cmd/server/main.go
   ```

### Using Environment Variables

You can override any config with environment variables:

```bash
export TRYGO_PORT=3000
export TRYGO_DB_DRIVER=postgres
export TRYGO_DB_HOST=localhost
export TRYGO_DB_USER=myuser
export TRYGO_DB_PASSWORD=mypass
export TRYGO_DB_NAME=mydb
go run cmd/server/main.go
```

## API Endpoints

- `GET /` - Hello World message
- `GET /ping` - Simple ping/pong
- `GET /health` - Health check with database status

Example responses:

```bash
# Hello endpoint
curl http://localhost:8080/
{"message":"Hello, World!","service":"trygo-template","status":"success"}

# Health check
curl http://localhost:8080/health
{"database":{"healthy":true,"status":"healthy"},"service":"trygo-template","status":"ok"}
```

## Project Structure

```
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── config/          # Configuration management
│   ├── db/              # Generated sqlc code (after running sqlc generate)
│   ├── handlers/        # HTTP handlers
│   └── server/          # Server setup and middleware
├── migrations/          # SQL migrations
├── queries/             # SQL queries for sqlc
├── docker-compose.yml   # Local development setup
├── sqlc.yaml           # sqlc configuration
└── config.yaml         # Application configuration
```

## Development

### Adding Database Operations

1. **Add SQL queries** in `queries/`:
   ```sql
   -- queries/users.sql
   -- name: GetUser :one
   SELECT * FROM users WHERE id = $1;
   ```

2. **Generate Go code:**
   ```bash
   sqlc generate
   ```

3. **Use in handlers:**
   ```go
   user, err := queries.GetUser(ctx, userID)
   ```

### Database Migrations

Add new migrations in `migrations/` following the naming pattern:
- `001_initial.sql`
- `002_add_users_table.sql`
- etc.

For PostgreSQL with Docker Compose, migrations in `migrations/` are automatically run on container startup.

### Hot Reload (Optional)

Install `air` for hot reload during development:

```bash
go install github.com/cosmtrek/air@latest
air
```

## Deployment

### Docker

```bash
# Build image
docker build -t trygo .

# Run with environment variables
docker run -p 8080:8080 \
  -e TRYGO_DB_DRIVER=postgres \
  -e TRYGO_DB_HOST=your-db-host \
  trygo
```

### Docker Compose (Full Stack)

Uncomment the `app` service in `docker-compose.yml` and run:

```bash
docker-compose up
```

## Configuration

The application supports configuration via:

1. **Config file** (`config.yaml`)
2. **Environment variables** (prefixed with `TRYGO_`)
3. **Command line flags** (via environment)

See `internal/config/config.go` for all available options.

## Contributing

1. Fork the project
2. Create your feature branch
3. Add tests for new functionality
4. Run `go test ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
