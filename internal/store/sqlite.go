package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type sqliteStore struct {
	db *sql.DB
}

// sqlConn is what the wrapper needs from database/sql; both *sql.DB and *sql.Tx have it, so queries run the same inside a transaction.
type sqlConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqliteWrapper struct {
	db sqlConn
}

// Exec implements db.DBTX interface for SQLite
func (w *sqliteWrapper) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	result, err := w.db.ExecContext(ctx, query, args...)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	rowsAffected, _ := result.RowsAffected()
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", rowsAffected)), nil
}

// Query implements db.DBTX interface for SQLite
// func (w *sqliteWrapper) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
// 	// This is tricky - we need to return pgx.Rows but we have sql.Rows
// 	// For now, we'll return an error since this gets complex
// 	return nil, fmt.Errorf("Query method not implemented for SQLite wrapper - use QueryRow instead")
// }

// QueryRow implements db.DBTX interface for SQLite
func (w *sqliteWrapper) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	row := w.db.QueryRowContext(ctx, query, args...)
	return &sqliteRowWrapper{row: row}
}

// sqliteRowWrapper wraps a *sql.Row to implement pgx.Row interface
type sqliteRowWrapper struct {
	row *sql.Row
}

// Scan implements pgx.Row interface for SQLite
func (w *sqliteRowWrapper) Scan(dest ...any) error {
	return w.row.Scan(dest...)
}

type sqliteRowsWrapper struct {
	rows *sql.Rows
	err  error
	conn *pgx.Conn // or keep nil if you don't have a real pgx connection
}

func (r *sqliteRowsWrapper) Close() {
	_ = r.rows.Close()
}

func (r *sqliteRowsWrapper) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.rows.Err()
}

func (r *sqliteRowsWrapper) Next() bool {
	return r.rows.Next()
}

func (r *sqliteRowsWrapper) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}
func (r *sqliteRowsWrapper) Conn() *pgx.Conn {
	return r.conn // return nil since SQLite doesn't use pgx.Conn
}

// func (w *sqliteWrapper) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
// 	// This is tricky - we need to return pgx.Rows but we have sql.Rows
// 	// For now, we'll return an error since this gets complex
// 	return nil, fmt.Errorf("Query method not implemented for SQLite wrapper - use QueryRow instead")
// }

// QueryRow implements db.DBTX interface for SQLite

// Implement other pgx.Rows methods as needed
func (r *sqliteRowsWrapper) Values() ([]any, error) {
	return nil, fmt.Errorf("Values not implemented")
}

func (r *sqliteRowsWrapper) RawValues() [][]byte {
	return nil
}

func (r *sqliteRowsWrapper) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *sqliteRowsWrapper) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

// Values are scanned via database/sql, so there is no pgtype map; the interface permits nil.
func (r *sqliteRowsWrapper) TypeMap() *pgtype.Map {
	return nil
}

func (w *sqliteWrapper) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	rows, err := w.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &sqliteRowsWrapper{rows: rows}, nil
}

// func (s *sqliteStore) CreateUser(ctx context.Context, params db.CreateUserParams) (int32, error) {
// 	wrapper := &sqliteWrapper{
// 		db: s.db,
// 	}
// 	querier := db.New(wrapper)
// 	user, err := querier.CreateUser(ctx, params)
// 	if err != nil {
// 		return -1, err
// 	}
// 	return user.ID, nil
// }

func GetSqliteStore(cfg *config.Config) (Store, error) {
	dsn := cfg.Db.DSN()
	if dsn == "" {
		return nil, fmt.Errorf("invalid database configuration")
	}
	// Without a busy timeout, concurrent writers across the pool fail immediately with SQLITE_BUSY instead of waiting their turn.
	// Immediate transactions take the write lock up front: a deferred one that reads then writes can deadlock with another and gets SQLITE_BUSY without waiting.
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=busy_timeout(5000)&_txlock=immediate"
	}
	// Use database/sql for SQLite (pgx doesn't support SQLite)
	dbconn, err := sql.Open(cfg.Db.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Test the connection
	if err := dbconn.Ping(); err != nil {
		if closeErr := dbconn.Close(); closeErr != nil {
			log.Printf("Error closing sqlite database after ping failure: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	// Configure connection pool
	dbconn.SetMaxOpenConns(25)
	dbconn.SetMaxIdleConns(25)
	dbconn.SetConnMaxLifetime(5 * time.Minute)

	s := &sqliteStore{
		db: dbconn,
	}
	return s, nil
}

func (s *sqliteStore) GetQuerier() (db.Querier, error) {
	wrapper := &sqliteWrapper{
		db: s.db,
	}
	return db.New(wrapper), nil
}

func (s *sqliteStore) Healthy(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("sqlite connection unavailable")
	}
	return s.db.PingContext(ctx)
}

func (s *sqliteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *sqliteStore) InTx(ctx context.Context, fn func(db.Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning sqlite transaction: %w", err)
	}
	if err := fn(db.New(&sqliteWrapper{db: tx})); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
