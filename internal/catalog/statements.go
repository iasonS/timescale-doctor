package catalog

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Statement is one row of pg_stat_statements.
type Statement struct {
	Query       string
	Calls       int64
	TotalMillis float64
	MeanMillis  float64
	Rows        int64
}

const statementsSchemaSQL = `
select coalesce((select n.nspname
                 from pg_extension e
                 join pg_namespace n on n.oid = e.extnamespace
                 where e.extname = 'pg_stat_statements'), '')`

var statementColumns = []string{"dbid", "query", "calls", "total_exec_time", "mean_exec_time", "rows"}

const statementsSQL = `
select coalesce(query, ''),
       calls,
       total_exec_time::float8,
       mean_exec_time::float8,
       rows
from {schema}.pg_stat_statements
where dbid = (select oid from pg_database where datname = current_database())
order by total_exec_time desc
limit $1`

// ReadStatements reads the statements of this database that took the most time in total.
// It returns ErrNotInstalled when pg_stat_statements is not installed here.
func ReadStatements(ctx context.Context, q Querier, limit int) ([]Statement, error) {
	var schema string
	if err := q.QueryRow(ctx, statementsSchemaSQL).Scan(&schema); err != nil {
		return nil, err
	}
	if schema == "" {
		return nil, ErrNotInstalled
	}
	if err := require(ctx, q, schema, "pg_stat_statements", statementColumns...); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, withSchema(statementsSQL, schema), limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Statement, error) {
		var s Statement
		err := row.Scan(&s.Query, &s.Calls, &s.TotalMillis, &s.MeanMillis, &s.Rows)
		return s, err
	})
}
