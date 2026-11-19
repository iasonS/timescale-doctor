package catalog

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Hypertable is one hypertable with its size and policies.
type Hypertable struct {
	Schema             string
	Name               string
	Chunks             int64
	CompressionEnabled bool
	ChunkInterval      string // "1 day", or a number for an integer dimension
	TotalBytes         int64
	Compression        CompressionStats
	RetentionPolicy    *Policy
	CompressionPolicy  *Policy
}

// FullName is schema.name.
func (h Hypertable) FullName() string {
	return h.Schema + "." + h.Name
}

// CompressionStats describe the chunks that are compressed now.
type CompressionStats struct {
	Known       bool // false when the stats could not be read
	Chunks      int64
	BytesBefore int64
	BytesAfter  int64
}

// Policy is a retention or compression policy job.
type Policy struct {
	JobID     int32
	Scheduled bool
	Schedule  time.Duration
	After     string         // as configured, for example "30 days"
	AfterAge  *time.Duration // nil when After is not a time interval
}

// Chunk is one chunk on a time dimension.
type Chunk struct {
	Hypertable string // schema.name
	End        time.Time
	Compressed bool
}

var hypertableColumns = []string{"hypertable_schema", "hypertable_name", "num_chunks", "compression_enabled"}

var dimensionColumns = []string{
	"hypertable_schema", "hypertable_name", "dimension_number", "column_type", "time_interval", "integer_interval",
}

const hypertablesSQL = `
select h.hypertable_schema,
       h.hypertable_name,
       h.num_chunks,
       h.compression_enabled,
       coalesce(d.time_interval::text, d.integer_interval::text, ''),
       coalesce({schema}.hypertable_size(
           (quote_ident(h.hypertable_schema) || '.' || quote_ident(h.hypertable_name))::regclass), 0)
from timescaledb_information.hypertables h
left join timescaledb_information.dimensions d
       on d.hypertable_schema = h.hypertable_schema
      and d.hypertable_name = h.hypertable_name
      and d.dimension_number = 1
order by h.hypertable_schema, h.hypertable_name`

// ReadHypertables reads every hypertable with its size and policies. timescale is the
// schema of the timescaledb extension.
func ReadHypertables(ctx context.Context, q Querier, timescale string) ([]Hypertable, error) {
	if err := require(ctx, q, "timescaledb_information", "hypertables", hypertableColumns...); err != nil {
		return nil, err
	}
	if err := require(ctx, q, "timescaledb_information", "dimensions", dimensionColumns...); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, withSchema(hypertablesSQL, timescale))
	if err != nil {
		return nil, err
	}
	tables, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Hypertable, error) {
		var h Hypertable
		err := row.Scan(&h.Schema, &h.Name, &h.Chunks, &h.CompressionEnabled, &h.ChunkInterval, &h.TotalBytes)
		return h, err
	})
	if err != nil {
		return nil, err
	}
	if err := addPolicies(ctx, q, tables); err != nil {
		return nil, err
	}
	return tables, nil
}

var jobColumns = []string{
	"job_id", "proc_schema", "proc_name", "schedule_interval", "scheduled", "config", "next_start",
	"hypertable_schema", "hypertable_name",
}

// The *_created_before settings age chunks by creation time instead of by their time range.
const policiesSQL = `
select hypertable_schema,
       hypertable_name,
       proc_name,
       job_id,
       scheduled,
       extract(epoch from schedule_interval)::float8,
       coalesce(config->>'drop_after', config->>'compress_after',
                (config->>'drop_created_before') || ' after creation',
                (config->>'compress_created_before') || ' after creation', ''),
       case when jsonb_typeof(coalesce(config->'drop_after', config->'compress_after')) = 'string'
            then extract(epoch from coalesce(config->>'drop_after', config->>'compress_after')::interval)::float8
       end
from timescaledb_information.jobs
where proc_name in ('policy_retention', 'policy_compression')
  and hypertable_name is not null`

func addPolicies(ctx context.Context, q Querier, tables []Hypertable) error {
	if err := require(ctx, q, "timescaledb_information", "jobs", jobColumns...); err != nil {
		return err
	}
	rows, err := q.Query(ctx, policiesSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	byName := byFullName(tables)
	for rows.Next() {
		var schema, name, proc string
		var policy Policy
		var schedule float64
		var after *float64
		if err := rows.Scan(&schema, &name, &proc, &policy.JobID, &policy.Scheduled, &schedule, &policy.After, &after); err != nil {
			return err
		}
		table := byName[schema+"."+name]
		if table == nil {
			continue
		}
		policy.Schedule = seconds(schedule)
		if after != nil {
			age := seconds(*after)
			policy.AfterAge = &age
		}
		if proc == "policy_retention" {
			table.RetentionPolicy = &policy
		} else {
			table.CompressionPolicy = &policy
		}
	}
	return rows.Err()
}

const compressionStatsSQL = `
select h.hypertable_schema,
       h.hypertable_name,
       coalesce(sum(s.number_compressed_chunks), 0)::bigint,
       coalesce(sum(s.before_compression_total_bytes), 0)::bigint,
       coalesce(sum(s.after_compression_total_bytes), 0)::bigint
from timescaledb_information.hypertables h
cross join lateral {schema}.{function}(
    (quote_ident(h.hypertable_schema) || '.' || quote_ident(h.hypertable_name))::regclass) s
where h.compression_enabled
group by h.hypertable_schema, h.hypertable_name`

// AddCompressionStats fills in the compression stats of the hypertables. 2.17 has only
// hypertable_compression_stats, 2.32 also hypertable_columnstore_stats; this reads the
// columnstore one when it exists, and returns an *Unsupported error when neither does.
func AddCompressionStats(ctx context.Context, q Querier, timescale string, tables []Hypertable) error {
	function, err := statsFunction(ctx, q, timescale)
	if err != nil {
		return err
	}
	sql := strings.ReplaceAll(withSchema(compressionStatsSQL, timescale), "{function}", function)
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	byName := byFullName(tables)
	for rows.Next() {
		var schema, name string
		stats := CompressionStats{Known: true}
		if err := rows.Scan(&schema, &name, &stats.Chunks, &stats.BytesBefore, &stats.BytesAfter); err != nil {
			return err
		}
		if table := byName[schema+"."+name]; table != nil {
			table.Compression = stats
		}
	}
	return rows.Err()
}

func statsFunction(ctx context.Context, q Querier, timescale string) (string, error) {
	for _, name := range []string{"hypertable_columnstore_stats", "hypertable_compression_stats"} {
		exists, err := functionExists(ctx, q, timescale, name)
		if err != nil {
			return "", err
		}
		if exists {
			return name, nil
		}
	}
	return "", &Unsupported{Missing: []string{timescale + ".hypertable_compression_stats"}}
}

const chunksSQL = `
select hypertable_schema || '.' || hypertable_name,
       range_end,
       is_compressed
from timescaledb_information.chunks
where isfinite(range_end)`

// ReadChunks reads the chunks of time dimensions; integer dimensions have no time range.
func ReadChunks(ctx context.Context, q Querier) ([]Chunk, error) {
	needed := []string{"hypertable_schema", "hypertable_name", "range_end", "is_compressed"}
	if err := require(ctx, q, "timescaledb_information", "chunks", needed...); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, chunksSQL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Chunk, error) {
		var c Chunk
		err := row.Scan(&c.Hypertable, &c.End, &c.Compressed)
		return c, err
	})
}

func byFullName(tables []Hypertable) map[string]*Hypertable {
	byName := map[string]*Hypertable{}
	for i := range tables {
		byName[tables[i].FullName()] = &tables[i]
	}
	return byName
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}
