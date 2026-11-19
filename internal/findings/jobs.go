package findings

import (
	"fmt"
	"strings"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/format"
)

// Jobs finds jobs whose last run failed, and scheduled jobs that have not succeeded for more
// than StaleRuns schedule intervals.
func Jobs(jobs []catalog.Job, now time.Time) []string {
	var found []string
	for _, job := range jobs {
		if finding := jobFinding(job, now); finding != "" {
			found = append(found, finding)
		}
	}
	return found
}

func jobFinding(job catalog.Job, now time.Time) string {
	if job.LastRunStatus == "Failed" {
		return fmt.Sprintf("job %d (%s) %s; last success: %s.",
			job.ID, JobName(job), failedRuns(job), format.Relative(job.LastSuccess, now, "never"))
	}
	if !job.Scheduled || job.LastSuccess == nil || job.ScheduleEvery <= 0 {
		return ""
	}
	since := now.Sub(*job.LastSuccess)
	if since <= StaleRuns*job.ScheduleEvery {
		return ""
	}
	return fmt.Sprintf("job %d (%s) last succeeded %s ago but should run every %s.",
		job.ID, JobName(job), format.Duration(since), job.Schedule)
}

func failedRuns(job catalog.Job) string {
	if job.ConsecutiveFailures == nil || *job.ConsecutiveFailures <= 1 {
		return "failed its last run"
	}
	return fmt.Sprintf("failed its last %d runs", *job.ConsecutiveFailures)
}

// JobName says what a job does: the policy and what it works on, or the procedure of a
// user-defined job.
func JobName(job catalog.Job) string {
	if job.Target == "" {
		return job.Proc
	}
	_, proc, _ := strings.Cut(job.Proc, ".")
	return strings.TrimPrefix(proc, "policy_") + " " + job.Target
}
