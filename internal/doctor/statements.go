package doctor

import (
	"context"
	"errors"
	"fmt"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/format"
	"github.com/iasonS/timescale-doctor/internal/report"
)

const (
	topStatements = 10
	queryWidth    = 80
)

func statements(ctx context.Context, q catalog.Querier, _ catalog.Server) (report.Section, error) {
	top, err := catalog.ReadStatements(ctx, q, topStatements)
	if errors.Is(err, catalog.ErrNotInstalled) {
		return report.Section{Notes: []string{"skipped: pg_stat_statements is not installed in this database"}}, nil
	}
	if err != nil {
		return report.Section{}, err
	}
	section := report.Section{Columns: []string{"total", "calls", "mean", "rows", "query"}}
	for _, s := range top {
		section.Rows = append(section.Rows, []string{
			format.Millis(s.TotalMillis), fmt.Sprint(s.Calls), format.Millis(s.MeanMillis), fmt.Sprint(s.Rows),
			format.OneLine(s.Query, queryWidth),
		})
	}
	return section, nil
}
