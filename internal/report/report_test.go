package report

import (
	"bytes"
	"encoding/json"
	"testing"
)

var sample = Report{Sections: []Section{
	{
		Name: "caggs", Title: "Continuous aggregates",
		Findings: []string{"public.metrics_daily has no refresh policy, so it is only refreshed by hand."},
		Columns:  []string{"continuous aggregate", "real-time"},
		Rows:     [][]string{{"public.metrics_daily", "on"}, {"public.metrics_hourly", "off"}},
	},
	{
		Name: "statements", Title: "Top statements", Findings: []string{},
		Notes: []string{"skipped: pg_stat_statements is not installed in this database"},
	},
	{Name: "jobs", Title: "Background jobs", Findings: []string{}, Columns: []string{"job"}},
}}

func TestTextPutsFindingsAboveAnAlignedTable(t *testing.T) {
	var out bytes.Buffer
	if err := WriteText(&out, sample); err != nil {
		t.Fatal(err)
	}
	want := `== Continuous aggregates ==
! public.metrics_daily has no refresh policy, so it is only refreshed by hand.
continuous aggregate   real-time
public.metrics_daily   on
public.metrics_hourly  off

== Top statements ==
note: skipped: pg_stat_statements is not installed in this database

== Background jobs ==
(none)
`
	if out.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestJSONHasTheSameContent(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, sample); err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(out.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Sections) != 3 || back.Sections[0].Rows[1][0] != "public.metrics_hourly" {
		t.Fatalf("got %+v", back)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"findings": []`)) {
		t.Fatalf("a section without findings should have an empty list:\n%s", out.String())
	}
}
