package xrayhq

import (
	"context"
	"database/sql"
	"time"
)

// WrappedDB wraps a *sql.DB to capture query metrics.
type WrappedDB struct {
	*sql.DB
}

// WrapDB wraps a *sql.DB for query instrumentation.
func WrapDB(db *sql.DB) *WrappedDB {
	return &WrappedDB{DB: db}
}

func (w *WrappedDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := w.DB.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	q := DBQuery{
		Query:     query,
		Duration:  duration,
		Timestamp: start,
	}
	if err != nil {
		q.Error = err.Error()
	}
	AddDBQuery(ctx, q)
	return rows, err
}

func (w *WrappedDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := w.DB.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	q := DBQuery{
		Query:     query,
		Duration:  duration,
		Timestamp: start,
	}
	AddDBQuery(ctx, q)
	return row
}

func (w *WrappedDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := w.DB.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	q := DBQuery{
		Query:     query,
		Duration:  duration,
		Timestamp: start,
	}
	if err != nil {
		q.Error = err.Error()
	}
	if result != nil {
		q.RowsAffected, _ = result.RowsAffected()
	}
	AddDBQuery(ctx, q)
	return result, err
}

// PoolStats returns the current DB pool statistics.
func (w *WrappedDB) PoolStats() DBPoolStats {
	stats := w.DB.Stats()
	return DBPoolStats{
		OpenConnections:  stats.OpenConnections,
		IdleConnections:  stats.Idle,
		InUseConnections: stats.InUse,
		WaitCount:        stats.WaitCount,
		WaitDuration:     stats.WaitDuration,
	}
}
