package findings

import "github.com/iasonS/timescale-doctor/internal/catalog"

// Caggs finds continuous aggregates that nothing refreshes.
func Caggs(caggs []catalog.Cagg) []string {
	var found []string
	for _, cagg := range caggs {
		if cagg.Policy == nil {
			found = append(found, cagg.Name+" has no refresh policy, so it is only refreshed by hand.")
		}
	}
	return found
}
