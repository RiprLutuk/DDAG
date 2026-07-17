// Package store contains the metadata repositories. Every method takes a
// context and uses the shared pgx pool; reads use scany for struct scanning.
package store

import (
	"context"
	"errors"
	"strings"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// Store is the metadata repository facade. All entity methods hang off it.
type Store struct {
	pool *pgxpool.Pool
}

// New builds a Store over the metadata pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the underlying pool for advanced/transactional use.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// get scans a single row into dest, mapping pgx.ErrNoRows to ErrNotFound.
func (s *Store) get(ctx context.Context, dest interface{}, sql string, args ...interface{}) error {
	err := pgxscan.Get(ctx, s.pool, dest, sql, args...)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// selectRows scans many rows into dest (a pointer to a slice).
func (s *Store) selectRows(ctx context.Context, dest interface{}, sql string, args ...interface{}) error {
	return pgxscan.Select(ctx, s.pool, dest, sql, args...)
}

// ListParams holds common pagination/search inputs.
type ListParams struct {
	Page    int
	Limit   int
	Search  string
	SortBy  string
	SortDir string
}

// Normalize clamps page/limit to sane defaults.
func (p *ListParams) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 25
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
}

// Offset returns the SQL OFFSET for the params.
func (p ListParams) Offset() int { return (p.Page - 1) * p.Limit }

// OrderBy returns a safe ORDER BY expression from a whitelist.
func (p ListParams) OrderBy(allowed map[string]string, fallback string) string {
	col := allowed[p.SortBy]
	if col == "" {
		col = fallback
	}
	dir := strings.ToUpper(p.SortDir)
	if dir != "ASC" {
		dir = "DESC"
	}
	return col + " " + dir
}
