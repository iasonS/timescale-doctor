package catalog

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Job is one background job with its last runs.
type Job struct {
	ID                  int32
	Proc                string // schema.name of the procedure the job calls
	Target              string // hypertable or continuous aggregate it works on; empty for user jobs
	Schedule            string // as PostgreSQL prints it, for example "1 day"
	ScheduleEvery       time.Duration
	Scheduled           bool   // false when paused
	State               string // Scheduled, Running or Paused
	LastRunStatus       string // Success, Failed, or empty before the first run
	LastSuccess         *time.Time
	NextStart           *time.Time
	TotalRuns           int64
	TotalFailures       int64
	ConsecutiveFailures *int32 // nil when the version does not expose it
}

var jobStatsColumns = []string{
	"job_id", "job_status", "last_run_status", "last_successful_finish", "total_runs", "total_failures",
}

// A refresh policy's job names the materialization hypertable in 2.17 and older and the view
// in 2.32; the join to continuous_aggregates shows the view's name in both cases. In 2.17 and
// older job_stats has no row for a job that never ran, hence the left join.
const jobsSQL = `
select j.job_id,
       j.proc_schema || '.' || j.proc_name,
       coalesce(c.view_schema || '.' || c.view_name, j.hypertable_schema || '.' || j.hypertable_name, ''),
       j.schedule_interval::text,
       extract(epoch from j.schedule_interval)::float8,
       j.scheduled,
       coalesce(s.job_status, case when j.scheduled then 'Scheduled' else 'Paused' end),
       coalesce(s.last_run_status, ''),
       case when isfinite(s.last_successful_finish) then s.last_successful_finish end,
       case when isfinite(j.next_start) then j.next_start end,
       coalesce(s.total_runs, 0),
       coalesce(s.total_failures, 0)
from timescaledb_information.jobs j
left join timescaledb_information.job_stats s on s.job_id = j.job_id
left join timescaledb_information.continuous_aggregates c
       on c.materialization_hypertable_schema = j.hypertable_schema
      and c.materialization_hypertable_name = j.hypertable_name
order by j.job_id`

// ReadJobs reads every background job with its stats.
func ReadJobs(ctx context.Context, q Querier) ([]Job, error) {
	if err := require(ctx, q, "timescaledb_information", "jobs", jobColumns...); err != nil {
		return nil, err
	}
	if err := require(ctx, q, "timescaledb_information", "job_stats", jobStatsColumns...); err != nil {
		return nil, err
	}
	if err := require(ctx, q, "timescaledb_information", "continuous_aggregates", caggColumns...); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, jobsSQL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Job, error) {
		var j Job
		var every float64
		err := row.Scan(&j.ID, &j.Proc, &j.Target, &j.Schedule, &every, &j.Scheduled, &j.State,
			&j.LastRunStatus, &j.LastSuccess, &j.NextStart, &j.TotalRuns, &j.TotalFailures)
		j.ScheduleEvery = seconds(every)
		return j, err
	})
}

const consecutiveFailuresSQL = `
select job_id, consecutive_failures
from _timescaledb_internal.bgw_job_stat`

// AddConsecutiveFailures fills in how often each job failed in a row. No information view
// shows this, so it comes from the internal bgw_job_stat table.
func AddConsecutiveFailures(ctx context.Context, q Querier, jobs []Job) error {
	if err := require(ctx, q, "_timescaledb_internal", "bgw_job_stat", "job_id", "consecutive_failures"); err != nil {
		return err
	}
	rows, err := q.Query(ctx, consecutiveFailuresSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	byID := map[int32]*Job{}
	for i := range jobs {
		byID[jobs[i].ID] = &jobs[i]
	}
	for rows.Next() {
		var id, failures int32
		if err := rows.Scan(&id, &failures); err != nil {
			return err
		}
		if job := byID[id]; job != nil {
			job.ConsecutiveFailures = &failures
		}
	}
	return rows.Err()
}
