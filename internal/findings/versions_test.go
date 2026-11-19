package findings

import (
	"strings"
	"testing"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

func TestCompareVersionsComparesNumbersNotText(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.14.2", "2.9.3", 1},
		{"2.9.3", "2.14.2", -1},
		{"2.17.2", "2.17.2", 0},
		{"2.18", "2.18.0", 0},
		{"2.18.0-dev", "2.18.0", 0},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNewestPicksTheHighestVersion(t *testing.T) {
	// pg_available_extension_versions lists them in no useful order.
	if got := Newest([]string{"2.10.0", "2.14.2", "2.8.1", "2.9.3"}); got != "2.14.2" {
		t.Fatalf("got %q, want 2.14.2", got)
	}
}

func TestNewerVersionOnTheServerIsAFinding(t *testing.T) {
	got := Versions(catalog.Versions{Timescale: "2.31.0", Available: []string{"2.31.0", "2.32.0"}})
	if len(got) != 1 || !strings.Contains(got[0], "2.32.0") || !strings.Contains(got[0], "2.31.0") {
		t.Fatalf("got %q", got)
	}
}

func TestUpToDateExtensionIsFine(t *testing.T) {
	if got := Versions(catalog.Versions{Timescale: "2.32.0", Available: []string{"2.31.0", "2.32.0"}}); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}

func TestMissingExtensionHasNoVersionFinding(t *testing.T) {
	if got := Versions(catalog.Versions{Available: []string{"2.32.0"}}); got != nil {
		t.Fatalf("got %q, want nothing", got)
	}
}
