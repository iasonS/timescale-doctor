package catalog

import "context"

// Versions are the PostgreSQL and timescaledb versions.
type Versions struct {
	Postgres  string
	Timescale string   // version this database runs; empty when not installed
	Available []string // timescaledb versions the server has files for
	License   string   // timescaledb.license: "timescale" or "apache"
}

const versionsSQL = `
select current_setting('server_version'),
       coalesce((select extversion from pg_extension where extname = 'timescaledb'), ''),
       coalesce((select array_agg(version::text)
                 from pg_available_extension_versions
                 where name = 'timescaledb'), '{}'),
       coalesce(current_setting('timescaledb.license', true), '')`

// ReadVersions reads the server and extension versions.
func ReadVersions(ctx context.Context, q Querier) (Versions, error) {
	var v Versions
	err := q.QueryRow(ctx, versionsSQL).Scan(&v.Postgres, &v.Timescale, &v.Available, &v.License)
	return v, err
}
