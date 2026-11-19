package findings

import (
	"fmt"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/format"
)

// Connections finds a server close to max_connections, a long open transaction and
// sessions waiting on a lock.
func Connections(c catalog.Connections) []string {
	var found []string
	if used := inUse(c); c.Max > 0 && float64(used) > BusyConnections*float64(c.Max) {
		found = append(found, fmt.Sprintf("%d of %d connections are in use.", used, c.Max))
	}
	if c.Longest != nil && c.Longest.Age > LongTransaction {
		found = append(found, fmt.Sprintf("pid %d (%s) has had a transaction open for %s.",
			c.Longest.PID, c.Longest.State, format.Duration(c.Longest.Age)))
	}
	if len(c.Waiting) == 1 {
		found = append(found, "1 session is waiting on a lock.")
	}
	if len(c.Waiting) > 1 {
		found = append(found, fmt.Sprintf("%d sessions are waiting on a lock.", len(c.Waiting)))
	}
	return found
}

func inUse(c catalog.Connections) int64 {
	used := int64(0)
	for _, state := range c.ByState {
		used += state.Count
	}
	return used
}
