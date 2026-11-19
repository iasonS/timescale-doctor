package format

import (
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	cases := map[int64]string{
		0:                  "0 bytes",
		812:                "812 bytes",
		8192:               "8.0 kB",
		8118272:            "7.7 MB",
		3 * 1024 * 1 << 30: "3.0 TB",
	}
	for n, want := range cases {
		if got := Bytes(n); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDurationShowsTwoUnits(t *testing.T) {
	cases := map[time.Duration]string{
		45 * time.Second:                 "45s",
		12 * time.Minute:                 "12m",
		12*time.Minute + 15*time.Second:  "12m 15s",
		3*time.Hour + 5*time.Minute:      "3h 5m",
		52*time.Hour + 30*time.Minute:    "2d 4h",
		20*24*time.Hour + 10*time.Minute: "20d",
	}
	for d, want := range cases {
		if got := Duration(d); got != want {
			t.Errorf("Duration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestRelative(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	before, after := now.Add(-3*time.Minute), now.Add(5*time.Minute)
	if got := Relative(&before, now, "never"); got != "3m ago" {
		t.Errorf("got %q, want 3m ago", got)
	}
	if got := Relative(&after, now, "never"); got != "in 5m" {
		t.Errorf("got %q, want in 5m", got)
	}
	if got := Relative(nil, now, "never"); got != "never" {
		t.Errorf("got %q, want never", got)
	}
}

func TestMillis(t *testing.T) {
	cases := map[float64]string{
		0.4167:  "0.42 ms",
		508.9:   "508.9 ms",
		1612:    "1.6 s",
		125_000: "2m 5s",
	}
	for ms, want := range cases {
		if got := Millis(ms); got != want {
			t.Errorf("Millis(%v) = %q, want %q", ms, got, want)
		}
	}
}

func TestOneLinePutsAQueryOnOneShortLine(t *testing.T) {
	query := "select city,\n\tavg(temperature)\nfrom conditions\n  group by city"
	if got := OneLine(query, 80); got != "select city, avg(temperature) from conditions group by city" {
		t.Errorf("got %q", got)
	}
	if got := OneLine(query, 20); got != "select city, avg(..." {
		t.Errorf("got %q", got)
	}
}

func TestCount(t *testing.T) {
	if got := Count(1, "chunk"); got != "1 chunk" {
		t.Errorf("got %q", got)
	}
	if got := Count(4, "chunk"); got != "4 chunks" {
		t.Errorf("got %q", got)
	}
}
