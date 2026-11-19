package doctor

import (
	"context"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/findings"
	"github.com/iasonS/timescale-doctor/internal/report"
)

func versions(ctx context.Context, q catalog.Querier, _ catalog.Server) (report.Section, error) {
	v, err := catalog.ReadVersions(ctx, q)
	if err != nil {
		return report.Section{}, err
	}
	return report.Section{
		Findings: findings.Versions(v),
		Columns:  []string{"component", "version"},
		Rows: [][]string{
			{"PostgreSQL", v.Postgres},
			{"timescaledb in this database", orElse(v.Timescale, "not installed")},
			{"newest timescaledb on the server", orElse(findings.Newest(v.Available), "none")},
			{"timescaledb license", orElse(v.License, "-")},
		},
	}, nil
}

func orElse(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
