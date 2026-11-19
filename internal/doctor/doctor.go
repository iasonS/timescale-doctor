// Package doctor builds the health report: each section reads the catalog in its own
// read-only transaction, so one that fails or is not supported does not stop the others.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/report"
)

type builder func(ctx context.Context, q catalog.Querier, server catalog.Server) (report.Section, error)

type section struct {
	name           string
	title          string
	needsTimescale bool
	build          builder
}

var sections = []section{
	{name: "versions", title: "Versions", build: versions},
	{name: "hypertables", title: "Hypertables", needsTimescale: true, build: hypertables},
	{name: "jobs", title: "Background jobs", needsTimescale: true, build: jobs},
	{name: "caggs", title: "Continuous aggregates", needsTimescale: true, build: caggs},
	{name: "statements", title: "Top statements", build: statements},
	{name: "connections", title: "Connections and locks", build: connections},
}

// Names lists every section in report order.
func Names() []string {
	names := make([]string, len(sections))
	for i, s := range sections {
		names[i] = s.name
	}
	return names
}

// Select turns a comma-separated list such as "jobs,caggs" into section names in report
// order. An empty list selects every section.
func Select(only string) ([]string, error) {
	if strings.TrimSpace(only) == "" {
		return Names(), nil
	}
	wanted := map[string]bool{}
	for _, name := range strings.Split(only, ",") {
		name = strings.TrimSpace(name)
		if !slices.Contains(Names(), name) {
			return nil, fmt.Errorf("unknown section %q; sections are %s", name, strings.Join(Names(), ", "))
		}
		wanted[name] = true
	}
	var selected []string
	for _, name := range Names() {
		if wanted[name] {
			selected = append(selected, name)
		}
	}
	return selected, nil
}

// Run builds the report of the named sections. timeout is the statement timeout of every
// query. It returns an error only when it cannot read the database clock, which means
// nothing else would work either.
func Run(ctx context.Context, conn *pgx.Conn, names []string, timeout time.Duration) (report.Report, error) {
	r := runner{conn: conn, timeout: timeout}
	var server catalog.Server
	err := r.readOnly(ctx, func(tx pgx.Tx) error {
		var err error
		server, err = catalog.ReadServer(ctx, tx)
		return err
	})
	if err != nil {
		return report.Report{}, err
	}

	var result report.Report
	for _, s := range sections {
		if slices.Contains(names, s.name) {
			result.Sections = append(result.Sections, r.section(ctx, s, server))
		}
	}
	return result, nil
}

type runner struct {
	conn    *pgx.Conn
	timeout time.Duration
}

func (r runner) section(ctx context.Context, s section, server catalog.Server) report.Section {
	result := r.build(ctx, s, server)
	result.Name, result.Title = s.name, s.title
	if result.Findings == nil {
		result.Findings = []string{}
	}
	return result
}

func (r runner) build(ctx context.Context, s section, server catalog.Server) report.Section {
	if s.needsTimescale && server.Timescale == "" {
		return report.Section{Notes: []string{"skipped: the timescaledb extension is not installed in this database"}}
	}
	var result report.Section
	err := r.readOnly(ctx, func(tx pgx.Tx) error {
		var err error
		result, err = s.build(ctx, tx, server)
		return err
	})
	if err != nil {
		result.Notes = append(result.Notes, failureNote(err))
	}
	return result
}

func failureNote(err error) string {
	var unsupported *catalog.Unsupported
	if errors.As(err, &unsupported) {
		return "skipped: " + unsupported.Error()
	}
	return "could not read: " + err.Error()
}

// readOnly runs fn in a read-only transaction with the statement timeout, then rolls it back.
func (r runner) readOnly(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	millis := strconv.FormatInt(r.timeout.Milliseconds(), 10)
	if _, err := tx.Exec(ctx, "select set_config('statement_timeout', $1, true)", millis); err != nil {
		return err
	}
	return fn(tx)
}
