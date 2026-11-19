package doctor

import (
	"context"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/findings"
	"github.com/iasonS/timescale-doctor/internal/format"
	"github.com/iasonS/timescale-doctor/internal/report"
)

var caggColumns = []string{
	"continuous aggregate", "on", "real-time", "refresh from", "refresh to", "every", "materialised up to",
}

func caggs(ctx context.Context, q catalog.Querier, server catalog.Server) (report.Section, error) {
	all, err := catalog.ReadCaggs(ctx, q)
	if err != nil {
		return report.Section{}, err
	}
	var section report.Section
	if err := catalog.AddMaterializedTo(ctx, q, all); err != nil {
		section.Notes = append(section.Notes, "materialised up to not shown: "+err.Error())
	}
	section.Findings = findings.Caggs(all)
	section.Columns = caggColumns
	for _, cagg := range all {
		section.Rows = append(section.Rows, caggRow(cagg, server.Now))
	}
	return section, nil
}

func caggRow(c catalog.Cagg, now time.Time) []string {
	from, to, every := "no policy", "-", "-"
	if c.Policy != nil {
		from = offset(c.Policy.StartOffset, "the beginning")
		to = offset(c.Policy.EndOffset, "the end")
		every = c.Policy.Schedule
	}
	return []string{c.Name, c.Hypertable, onOff(c.RealTime), from, to, every, materializedTo(c, now)}
}

func materializedTo(c catalog.Cagg, now time.Time) string {
	if c.NeverRefreshed {
		return "never refreshed"
	}
	return format.Relative(c.MaterializedTo, now, "-")
}

// offset writes a refresh window's edge; an empty offset leaves that side of the window open.
func offset(value, open string) string {
	if value == "" {
		return open
	}
	return value + " ago"
}
