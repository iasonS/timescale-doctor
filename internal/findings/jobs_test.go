package findings

import (
	"reflect"
	"testing"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

func ago(d time.Duration) *time.Time {
	t := now.Add(-d)
	return &t
}

func failures(n int32) *int32 {
	return &n
}

func TestFailingJobIsAFinding(t *testing.T) {
	job := catalog.Job{
		ID: 1004, Proc: "public.export_totals", Schedule: "00:01:00", ScheduleEvery: time.Minute,
		Scheduled: true, LastRunStatus: "Failed", ConsecutiveFailures: failures(11),
	}
	got := Jobs([]catalog.Job{job}, now)
	want := []string{"job 1004 (public.export_totals) failed its last 11 runs; last success: never."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFailingJobWithoutFailureCountSaysLastRun(t *testing.T) {
	job := catalog.Job{
		ID: 1000, Proc: "_timescaledb_functions.policy_compression", Target: "public.metrics",
		Scheduled: true, LastRunStatus: "Failed", LastSuccess: ago(2 * day),
	}
	got := Jobs([]catalog.Job{job}, now)
	want := []string{"job 1000 (compression public.metrics) failed its last run; last success: 2d ago."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJobThatHasNotSucceededForThreeIntervalsIsAFinding(t *testing.T) {
	job := catalog.Job{
		ID: 1002, Proc: "_timescaledb_functions.policy_retention", Target: "public.metrics",
		Schedule: "1 day", ScheduleEvery: day, Scheduled: true, LastRunStatus: "Success",
		LastSuccess: ago(3*day + time.Hour),
	}
	got := Jobs([]catalog.Job{job}, now)
	want := []string{"job 1002 (retention public.metrics) last succeeded 3d 1h ago but should run every 1 day."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJobWithinThreeIntervalsIsFine(t *testing.T) {
	job := catalog.Job{
		ID: 1002, Proc: "public.export_totals", ScheduleEvery: day, Scheduled: true,
		LastRunStatus: "Success", LastSuccess: ago(3 * day),
	}
	if got := Jobs([]catalog.Job{job}, now); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestPausedJobIsNotStale(t *testing.T) {
	job := catalog.Job{
		ID: 1001, Proc: "public.export_totals", ScheduleEvery: time.Hour, Scheduled: false,
		LastRunStatus: "Success", LastSuccess: ago(30 * day),
	}
	if got := Jobs([]catalog.Job{job}, now); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestJobThatNeverRanIsFine(t *testing.T) {
	job := catalog.Job{ID: 1003, Proc: "public.export_totals", ScheduleEvery: time.Hour, Scheduled: true}
	if got := Jobs([]catalog.Job{job}, now); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}
