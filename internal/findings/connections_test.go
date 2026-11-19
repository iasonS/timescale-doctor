package findings

import (
	"reflect"
	"testing"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

func TestQuietServerHasNoFindings(t *testing.T) {
	c := catalog.Connections{
		Max:     100,
		ByState: []catalog.StateCount{{State: "idle", Count: 10}, {State: "active", Count: 2}},
		Longest: &catalog.Session{PID: 42, State: "active", Age: time.Minute},
	}
	if got := Connections(c); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestBusyServerLongTransactionAndLockWaitsAreFindings(t *testing.T) {
	c := catalog.Connections{
		Max:     100,
		ByState: []catalog.StateCount{{State: "idle", Count: 70}, {State: "idle in transaction", Count: 11}},
		Longest: &catalog.Session{PID: 42, State: "idle in transaction", Age: 2*time.Hour + 5*time.Minute},
		Waiting: []catalog.Session{{PID: 43, BlockedBy: []int32{42}}, {PID: 44, BlockedBy: []int32{42}}},
	}
	got := Connections(c)
	want := []string{
		"81 of 100 connections are in use.",
		"pid 42 (idle in transaction) has had a transaction open for 2h 5m.",
		"2 sessions are waiting on a lock.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
