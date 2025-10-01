# TryGo - go template

template app to use with db + http

clone and change as needed, push 

routing with chi, sqlc for db code gen

## Quick Start

nix dev shell for development,
run `just chores` to regen sqlc, gomod2nix etc.

The server will start on `http://localhost:8080` using SQLite (`./data.db`).

### Running with PostgreSQL


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
├── sqlc.yaml           # sqlc configuration
└── config.yaml         # Application configuration
```

