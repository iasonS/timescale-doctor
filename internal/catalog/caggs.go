package catalog

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Cagg is one continuous aggregate with its refresh policy.
type Cagg struct {
	Name            string // schema.view
	Hypertable      string // schema.name of the hypertable it aggregates
	Materialization string // schema.name of the hypertable that stores its rows
	RealTime        bool
	TimeType        string
	Policy          *RefreshPolicy
	MaterializedTo  *time.Time // nil when unknown or never refreshed
	NeverRefreshed  bool
}

// RefreshPolicy is a continuous aggregate's refresh job. The offsets are as configured;
// empty means open-ended.
type RefreshPolicy struct {
	StartOffset string
	EndOffset   string
	Schedule    string
}

var caggColumns = []string{
	"view_schema", "view_name", "hypertable_schema", "hypertable_name",
	"materialization_hypertable_schema", "materialization_hypertable_name", "materialized_only",
}

// A refresh policy names the materialization hypertable in older versions and the view in
// newer ones, so the join accepts either.
const caggsSQL = `
select c.view_schema || '.' || c.view_name,
       c.hypertable_schema || '.' || c.hypertable_name,
       c.materialization_hypertable_schema || '.' || c.materialization_hypertable_name,
       not c.materialized_only,
       coalesce(d.column_type::text, ''),
       j.job_id,
       coalesce(j.config->>'start_offset', ''),
       coalesce(j.config->>'end_offset', ''),
       coalesce(j.schedule_interval::text, '')
from timescaledb_information.continuous_aggregates c
left join timescaledb_information.dimensions d
       on d.hypertable_schema = c.materialization_hypertable_schema
      and d.hypertable_name = c.materialization_hypertable_name
      and d.dimension_number = 1
left join timescaledb_information.jobs j
       on j.proc_name = 'policy_refresh_continuous_aggregate'
      and ((j.hypertable_schema = c.view_schema and j.hypertable_name = c.view_name)
        or (j.hypertable_schema = c.materialization_hypertable_schema
            and j.hypertable_name = c.materialization_hypertable_name))
order by c.view_schema, c.view_name, j.job_id`

// ReadCaggs reads every continuous aggregate with its refresh policy. One with several
// policies is listed once per policy.
func ReadCaggs(ctx context.Context, q Querier) ([]Cagg, error) {
	if err := require(ctx, q, "timescaledb_information", "continuous_aggregates", caggColumns...); err != nil {
		return nil, err
	}
	if err := require(ctx, q, "timescaledb_information", "dimensions", dimensionColumns...); err != nil {
		return nil, err
	}
	if err := require(ctx, q, "timescaledb_information", "jobs", jobColumns...); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, caggsSQL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanCagg)
}

func scanCagg(row pgx.CollectableRow) (Cagg, error) {
	var c Cagg
	var jobID *int32
	var policy RefreshPolicy
	err := row.Scan(&c.Name, &c.Hypertable, &c.Materialization, &c.RealTime, &c.TimeType,
		&jobID, &policy.StartOffset, &policy.EndOffset, &policy.Schedule)
	if jobID != nil {
		c.Policy = &policy
	}
	return c, err
}

const watermarksSQL = `
select h.schema_name || '.' || h.table_name,
       w.watermark
from _timescaledb_catalog.continuous_aggs_watermark w
join _timescaledb_catalog.hypertable h on h.id = w.mat_hypertable_id`

// The watermark of a continuous aggregate that was never refreshed is the lowest time
// PostgreSQL can hold (4714 BC); anything before year 1 counts as never.
var neverRefreshed = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)

// AddMaterializedTo fills in how far each continuous aggregate on a time column is
// materialised. No information view shows this; the watermark table is internal, and the
// cagg_watermark() function needs privileges on the view that a monitoring user lacks.
func AddMaterializedTo(ctx context.Context, q Querier, caggs []Cagg) error {
	if err := require(ctx, q, "_timescaledb_catalog", "continuous_aggs_watermark", "mat_hypertable_id", "watermark"); err != nil {
		return err
	}
	if err := require(ctx, q, "_timescaledb_catalog", "hypertable", "id", "schema_name", "table_name"); err != nil {
		return err
	}
	rows, err := q.Query(ctx, watermarksSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	watermarks := map[string]int64{}
	for rows.Next() {
		var materialization string
		var watermark int64
		if err := rows.Scan(&materialization, &watermark); err != nil {
			return err
		}
		watermarks[materialization] = watermark
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range caggs {
		watermark, found := watermarks[caggs[i].Materialization]
		if !found || !isTimeType(caggs[i].TimeType) {
			continue
		}
		// TimescaleDB keeps times internally as microseconds since the Unix epoch.
		materializedTo := time.UnixMicro(watermark).UTC()
		if materializedTo.Before(neverRefreshed) {
			caggs[i].NeverRefreshed = true
			continue
		}
		caggs[i].MaterializedTo = &materializedTo
	}
	return nil
}

func isTimeType(columnType string) bool {
	switch columnType {
	case "timestamp with time zone", "timestamp without time zone", "date":
		return true
	}
	return false
}
