package findings

import (
	"reflect"
	"testing"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

var now = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

const day = 24 * time.Hour

func policy(jobID int32, after time.Duration, text string, schedule time.Duration) *catalog.Policy {
	return &catalog.Policy{JobID: jobID, Scheduled: true, Schedule: schedule, After: text, AfterAge: &after}
}

// chunks returns one-day chunks of public.metrics that ended 1, 2, ... days ago.
func chunks(days int, compressedFrom int) []catalog.Chunk {
	var all []catalog.Chunk
	for i := 1; i <= days; i++ {
		all = append(all, catalog.Chunk{
			Hypertable: "public.metrics",
			End:        now.Add(-time.Duration(i) * day),
			Compressed: i >= compressedFrom,
		})
	}
	return all
}

func healthy() catalog.Hypertable {
	return catalog.Hypertable{
		Schema: "public", Name: "metrics", Chunks: 40, CompressionEnabled: true, ChunkInterval: "1 day",
		RetentionPolicy:   policy(1001, 30*day, "30 days", day),
		CompressionPolicy: policy(1000, 7*day, "7 days", 12*time.Hour),
	}
}

func TestHealthyHypertableHasNoFindings(t *testing.T) {
	got := Hypertables([]catalog.Hypertable{healthy()}, chunks(30, 8), now)
	if got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestMissingPoliciesAreFindings(t *testing.T) {
	table := healthy()
	table.RetentionPolicy, table.CompressionPolicy = nil, nil
	got := Hypertables([]catalog.Hypertable{table}, nil, now)
	want := []string{
		"public.metrics has no retention policy, so its data is never dropped.",
		"public.metrics has compression on but no compression policy, so new chunks stay uncompressed.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNoCompressionPolicyIsFineWithoutCompression(t *testing.T) {
	table := healthy()
	table.CompressionEnabled, table.CompressionPolicy = false, nil
	if got := Hypertables([]catalog.Hypertable{table}, nil, now); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestTooManyChunksIsAFinding(t *testing.T) {
	table := healthy()
	table.Chunks, table.ChunkInterval = 1201, "01:00:00"
	got := Hypertables([]catalog.Hypertable{table}, nil, now)
	want := []string{"public.metrics has 1201 chunks; a chunk interval longer than 01:00:00 would make fewer."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestChunksPastRetentionAreFound(t *testing.T) {
	// Retention is 30 days and runs daily: chunks that ended 32 to 35 days ago are overdue,
	// the one that ended 31 days ago may still wait for the next run.
	got := Hypertables([]catalog.Hypertable{healthy()}, chunks(35, 8), now)
	want := []string{"public.metrics has 4 chunks older than its retention of 30 days; check retention job 1001."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestUncompressedChunksPastThePolicyAreFound(t *testing.T) {
	// Compression after 7 days, run every 12 hours: chunks that ended 8 to 10 days ago
	// should be compressed by now.
	got := Hypertables([]catalog.Hypertable{healthy()}, chunks(30, 11), now)
	want := []string{
		"public.metrics has 3 uncompressed chunks older than its compression policy's 7 days; check compression job 1000.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPolicyOnAnIntegerColumnIsNotAged(t *testing.T) {
	table := healthy()
	table.RetentionPolicy = &catalog.Policy{Scheduled: true, Schedule: day, After: "100000"}
	if got := Hypertables([]catalog.Hypertable{table}, chunks(60, 8), now); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestChunksOfAnotherHypertableDoNotCount(t *testing.T) {
	other := chunks(60, 100)
	for i := range other {
		other[i].Hypertable = "public.other"
	}
	if got := Hypertables([]catalog.Hypertable{healthy()}, other, now); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}
