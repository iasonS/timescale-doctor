package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/findings"
	"github.com/iasonS/timescale-doctor/internal/format"
	"github.com/iasonS/timescale-doctor/internal/report"
)

func connections(ctx context.Context, q catalog.Querier, _ catalog.Server) (report.Section, error) {
	c, err := catalog.ReadConnections(ctx, q)
	if err != nil {
		return report.Section{}, err
	}
	section := report.Section{
		Findings: findings.Connections(c),
		Columns:  []string{"what", "value"},
	}
	for _, state := range c.ByState {
		section.Rows = append(section.Rows, []string{"sessions " + state.State, fmt.Sprint(state.Count)})
	}
	section.Rows = append(section.Rows, []string{"max_connections", fmt.Sprint(c.Max)})
	section.Rows = append(section.Rows, []string{"oldest open transaction", longest(c.Longest)})
	for _, waiting := range c.Waiting {
		section.Rows = append(section.Rows, []string{"waiting on a lock", waiter(waiting)})
	}
	return section, nil
}

func longest(s *catalog.Session) string {
	if s == nil {
		return "none"
	}
	return fmt.Sprintf("pid %d, %s, %s for %s: %s",
		s.PID, s.User, s.State, format.Duration(s.Age), format.OneLine(s.Query, queryWidth))
}

func waiter(s catalog.Session) string {
	return fmt.Sprintf("pid %d, %s, for %s, blocked by pid %s: %s",
		s.PID, s.User, format.Duration(s.Age), joinPIDs(s.BlockedBy), format.OneLine(s.Query, queryWidth))
}

func joinPIDs(pids []int32) string {
	texts := make([]string, len(pids))
	for i, pid := range pids {
		texts[i] = fmt.Sprint(pid)
	}
	return strings.Join(texts, ", ")
}
