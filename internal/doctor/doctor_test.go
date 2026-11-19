package doctor

import (
	"reflect"
	"testing"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

var now = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func TestSelectKeepsReportOrder(t *testing.T) {
	got, err := Select("jobs, versions")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"versions", "jobs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSelectNothingMeansEverything(t *testing.T) {
	got, err := Select("")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, Names()) {
		t.Fatalf("got %v, want %v", got, Names())
	}
}

func TestSelectRejectsUnknownSections(t *testing.T) {
	if _, err := Select("jobs,chunks"); err == nil {
		t.Fatal("want an error for chunks")
	}
}

func TestHypertableRowWithCompression(t *testing.T) {
	table := catalog.Hypertable{
		Schema: "public", Name: "conditions", Chunks: 61, CompressionEnabled: true, ChunkInterval: "1 day",
		TotalBytes:        8 << 20,
		Compression:       catalog.CompressionStats{Known: true, Chunks: 40, BytesBefore: 12 << 20, BytesAfter: 3 << 20},
		RetentionPolicy:   &catalog.Policy{After: "30 days", Scheduled: false},
		CompressionPolicy: &catalog.Policy{After: "7 days", Scheduled: true},
	}
	want := []string{
		"public.conditions", "8.0 MB", "3.0 MB", "5.0 MB", "4.0x", "61 (40 compressed)", "1 day",
		"on", "30 days (paused)", "7 days",
	}
	if got := hypertableRow(table); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHypertableRowWithoutCompression(t *testing.T) {
	table := catalog.Hypertable{Schema: "public", Name: "page_views", Chunks: 1201, ChunkInterval: "01:00:00", TotalBytes: 1 << 30}
	want := []string{"public.page_views", "1.0 GB", "-", "-", "-", "1201", "01:00:00", "off", "none", "-"}
	if got := hypertableRow(table); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJobRow(t *testing.T) {
	last, next := now.Add(-10*time.Minute), now.Add(50*time.Minute)
	job := catalog.Job{
		ID: 1003, Proc: "_timescaledb_functions.policy_refresh_continuous_aggregate", Target: "public.metrics_hourly",
		Schedule: "01:00:00", State: "Scheduled", LastRunStatus: "Success", LastSuccess: &last, NextStart: &next,
		TotalRuns: 3,
	}
	want := []string{
		"1003", "refresh_continuous_aggregate public.metrics_hourly", "01:00:00", "Scheduled", "Success",
		"10m ago", "-", "in 50m",
	}
	if got := jobRow(job, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCaggRows(t *testing.T) {
	materialized := now.Add(-20 * 24 * time.Hour)
	withPolicy := catalog.Cagg{
		Name: "public.metrics_hourly", Hypertable: "public.metrics",
		Policy:         &catalog.RefreshPolicy{StartOffset: "3 days", Schedule: "01:00:00"},
		MaterializedTo: &materialized,
	}
	want := []string{
		"public.metrics_hourly", "public.metrics", "off", "3 days ago", "the end", "01:00:00", "20d ago",
	}
	if got := caggRow(withPolicy, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	never := catalog.Cagg{Name: "public.metrics_daily", Hypertable: "public.metrics", RealTime: true, NeverRefreshed: true}
	want = []string{"public.metrics_daily", "public.metrics", "on", "no policy", "-", "-", "never refreshed"}
	if got := caggRow(never, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
