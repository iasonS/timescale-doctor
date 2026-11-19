package findings

import (
	"reflect"
	"testing"

	"github.com/iasonS/timescale-doctor/internal/catalog"
)

func TestContinuousAggregateWithoutRefreshPolicyIsAFinding(t *testing.T) {
	caggs := []catalog.Cagg{
		{Name: "public.metrics_hourly", Policy: &catalog.RefreshPolicy{StartOffset: "3 days"}},
		{Name: "public.metrics_daily"},
	}
	got := Caggs(caggs)
	want := []string{"public.metrics_daily has no refresh policy, so it is only refreshed by hand."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
