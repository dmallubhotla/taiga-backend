package store

// Registered here, where connections are opened, so every binary using the store gets them without remembering the imports.
import (
	_ "github.com/lib/pq"  // PostgreSQL driver
	_ "modernc.org/sqlite" // SQLite driver
)
