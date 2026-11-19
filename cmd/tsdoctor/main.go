// tsdoctor prints a health report of a TimescaleDB database. It only reads.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iasonS/timescale-doctor/internal/doctor"
	"github.com/iasonS/timescale-doctor/internal/report"
)

const usage = `Usage: tsdoctor [flags] [connection string]

Prints a health report of a TimescaleDB database. It only reads.

The connection string is a URL or key=value string. Without one, tsdoctor uses
DATABASE_URL, or else the standard PG* environment variables (PGHOST, PGUSER, ...).

Flags:
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tsdoctor:", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}
	asJSON := flag.Bool("json", false, "print the report as JSON")
	only := flag.String("only", "", "comma-separated sections to print: "+strings.Join(doctor.Names(), ","))
	timeout := flag.Duration("timeout", 30*time.Second, "statement timeout of each query")
	flag.Parse()

	if flag.NArg() > 1 {
		return errors.New("give at most one connection string")
	}
	names, err := doctor.Select(*only)
	if err != nil {
		return err
	}
	config, err := connectionConfig(flag.Arg(0))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	r, err := doctor.Run(ctx, conn, names, *timeout)
	if err != nil {
		return err
	}
	if *asJSON {
		return report.WriteJSON(os.Stdout, r)
	}
	return report.WriteText(os.Stdout, r)
}

// connectionConfig reads the connection string, or DATABASE_URL when there is none. pgx
// fills in what neither gives from the PG* environment variables, as psql does.
func connectionConfig(connString string) (*pgx.ConnConfig, error) {
	if connString == "" {
		connString = os.Getenv("DATABASE_URL")
	}
	config, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if _, set := config.RuntimeParams["application_name"]; !set {
		config.RuntimeParams["application_name"] = "tsdoctor"
	}
	return config, nil
}
