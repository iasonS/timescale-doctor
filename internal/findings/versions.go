package findings

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

// Versions finds a timescaledb version on the server that this database has not updated to.
func Versions(v catalog.Versions) []string {
	if v.Timescale == "" {
		return nil
	}
	newest := Newest(v.Available)
	if CompareVersions(newest, v.Timescale) <= 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"timescaledb %s is installed on the server, but this database still runs %s; "+
			"run ALTER EXTENSION timescaledb UPDATE as the first command of a new session.",
		newest, v.Timescale)}
}

// Newest returns the highest of the versions, or "" when there are none.
func Newest(versions []string) string {
	newest := ""
	for _, version := range versions {
		if newest == "" || CompareVersions(version, newest) > 0 {
			newest = version
		}
	}
	return newest
}

// CompareVersions compares dotted versions such as 2.9.3 and 2.14.2 number by number and
// returns -1, 0 or 1. Anything after a dash, as in 2.18.0-dev, is ignored.
func CompareVersions(a, b string) int {
	left, right := versionNumbers(a), versionNumbers(b)
	for i := range max(len(left), len(right)) {
		l, r := numberAt(left, i), numberAt(right, i)
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}
	return 0
}

func versionNumbers(version string) []int {
	version, _, _ = strings.Cut(version, "-")
	var numbers []int
	for _, part := range strings.Split(version, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		numbers = append(numbers, n)
	}
	return numbers
}

func numberAt(numbers []int, i int) int {
	if i >= len(numbers) {
		return 0
	}
	return numbers[i]
}
