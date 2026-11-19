// Package catalog reads the PostgreSQL and TimescaleDB catalogs. It only runs SELECTs; the
// caller decides which transaction they run in. Each section's SQL lives in its own file.
package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Querier runs queries. A pgx.Tx is one.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Unsupported means a view or column a query needs does not exist, usually because the
// TimescaleDB or PostgreSQL version is older or newer than the query.
type Unsupported struct {
	Missing []string // "schema.view.column"
}

func (u *Unsupported) Error() string {
	return "this version has no " + strings.Join(u.Missing, ", ")
}

// ErrNotInstalled means an extension a section reads is not installed in this database.
var ErrNotInstalled = errors.New("extension not installed")

// Server is what every section needs to know before it starts.
type Server struct {
	Now       time.Time // the database's clock, so ages do not depend on the client's
	Timescale string    // schema of the timescaledb extension; empty when it is not installed
}

const serverSQL = `
select now(),
       coalesce((select n.nspname
                 from pg_extension e
                 join pg_namespace n on n.oid = e.extnamespace
                 where e.extname = 'timescaledb'), '')`

// ReadServer reads the database clock and where timescaledb is installed.
func ReadServer(ctx context.Context, q Querier) (Server, error) {
	var server Server
	err := q.QueryRow(ctx, serverSQL).Scan(&server.Now, &server.Timescale)
	return server, err
}

const columnsSQL = `
select column_name::text
from information_schema.columns
where table_schema = $1 and table_name = $2`

// columns returns the columns of a table or view that this user can see.
func columns(ctx context.Context, q Querier, schema, relation string) ([]string, error) {
	rows, err := q.Query(ctx, columnsSQL, schema, relation)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// require returns an *Unsupported error when the table or view lacks any of the columns.
func require(ctx context.Context, q Querier, schema, relation string, needed ...string) error {
	found, err := columns(ctx, q, schema, relation)
	if err != nil {
		return err
	}
	var missing []string
	for _, column := range needed {
		if !slices.Contains(found, column) {
			missing = append(missing, schema+"."+relation+"."+column)
		}
	}
	if len(missing) > 0 {
		return &Unsupported{Missing: missing}
	}
	return nil
}

const functionSQL = `
select exists (select 1
               from pg_proc p
               join pg_namespace n on n.oid = p.pronamespace
               where n.nspname = $1 and p.proname = $2)`

func functionExists(ctx context.Context, q Querier, schema, name string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, functionSQL, schema, name).Scan(&exists)
	return exists, err
}

// withSchema puts a quoted schema name wherever the SQL says {schema}, for functions of an
// extension that may not be installed in public.
func withSchema(sql, schema string) string {
	return strings.ReplaceAll(sql, "{schema}", pgx.Identifier{schema}.Sanitize())
}
