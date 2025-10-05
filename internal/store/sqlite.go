package store

import (
	"context"
	"database/sql"
	"fmt"
	"gitea.deepak.science/deepak/trygo/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type sqliteStore struct {
	db *sql.DB
}

type sqliteWrapper struct {
	db *sql.DB
}

// Exec implements db.DBTX interface for SQLite
func (w *sqliteWrapper) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	result, err := w.db.ExecContext(ctx, query, args...)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	rowsAffected, _ := result.RowsAffected()
	return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", rowsAffected)), nil
}

// Query implements db.DBTX interface for SQLite
func (w *sqliteWrapper) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	// This is tricky - we need to return pgx.Rows but we have sql.Rows
	// For now, we'll return an error since this gets complex
	return nil, fmt.Errorf("Query method not implemented for SQLite wrapper - use QueryRow instead")
}

// QueryRow implements db.DBTX interface for SQLite
func (w *sqliteWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	row := w.db.QueryRowContext(ctx, query, args...)
	return &sqliteRowWrapper{row: row}
}

// sqliteRowWrapper wraps a *sql.Row to implement pgx.Row interface
type sqliteRowWrapper struct {
	row *sql.Row
}

// Scan implements pgx.Row interface for SQLite
func (w *sqliteRowWrapper) Scan(dest ...interface{}) error {
	return w.row.Scan(dest...)
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
