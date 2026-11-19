package doctor

import (
	"context"
	"fmt"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/findings"
	"github.com/iasonS/timescale-doctor/internal/format"
	"github.com/iasonS/timescale-doctor/internal/report"
)

var jobColumns = []string{
	"job", "what", "schedule", "state", "last run", "last success", "failures in a row", "next start",
}

func jobs(ctx context.Context, q catalog.Querier, server catalog.Server) (report.Section, error) {
	all, err := catalog.ReadJobs(ctx, q)
	if err != nil {
		return report.Section{}, err
	}
	var section report.Section
	if err := catalog.AddConsecutiveFailures(ctx, q, all); err != nil {
		section.Notes = append(section.Notes, "failures in a row not shown: "+err.Error())
	}
	section.Findings = findings.Jobs(all, server.Now)
	section.Columns = jobColumns
	for _, job := range all {
		section.Rows = append(section.Rows, jobRow(job, server.Now))
	}
	return section, nil
}

func jobRow(job catalog.Job, now time.Time) []string {
	return []string{
		fmt.Sprint(job.ID),
		findings.JobName(job),
		job.Schedule,
		job.State,
		lastRun(job),
		format.Relative(job.LastSuccess, now, "never"),
		failuresInARow(job),
		format.Relative(job.NextStart, now, "-"),
	}
}

func lastRun(job catalog.Job) string {
	if job.LastRunStatus != "" {
		return job.LastRunStatus
	}
	if job.TotalRuns == 0 {
		return "never ran"
	}
	return "-"
}

func failuresInARow(job catalog.Job) string {
	if job.ConsecutiveFailures == nil {
		return "-"
	}
	return fmt.Sprint(*job.ConsecutiveFailures)
}
