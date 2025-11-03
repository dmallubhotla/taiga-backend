package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/migration"
)

func main() {
	var (
		command = flag.String("command", "up", "Migration command: up, down, version, steps, goto")
		steps   = flag.Int("steps", 0, "Number of steps for 'steps' command")
		version = flag.Uint("version", 0, "Target version for 'goto' command")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load("config")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	dsn := cfg.Db.DSN()
	if dsn == "" {
		log.Fatal("Invalid database configuration")
	}

	db, err := sql.Open(cfg.Db.Driver, dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize migrator
	migrator, err := migration.New(db, cfg.Db.Driver, "./migrations")
	if err != nil {
		log.Fatalf("Failed to initialize migrator: %v", err)
	}
	defer migrator.Close()

	// Execute command
	switch *command {
	case "up":
		log.Println("Running up migrations...")
		err = migrator.Up()
	case "down":
		log.Println("Running down migrations...")
		err = migrator.Down()
	case "steps":
		if *steps == 0 {
			log.Fatal("Steps must be non-zero for 'steps' command")
		}
		log.Printf("Running %d migration steps...", *steps)
		err = migrator.Steps(*steps)
	case "goto":
		if *version == 0 {
			log.Fatal("Version must be non-zero for 'goto' command")
		}
		log.Printf("Migrating to version %d...", *version)
		err = migrator.Migrate(*version)
	case "version":
		ver, dirty, err := migrator.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %v)\n", ver, dirty)
		return
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *command)
		fmt.Fprintln(os.Stderr, "Available commands: up, down, version, steps, goto")
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Show final version
	ver, dirty, err := migrator.Version()
	if err != nil {
		log.Printf("Warning: failed to get final version: %v", err)
	} else {
		log.Printf("Migration completed. Current version: %d (dirty: %v)", ver, dirty)
	}
}
