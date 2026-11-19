# timescale-doctor

`tsdoctor` prints a health report of a TimescaleDB database in one command, instead of the dozen
catalog queries you would otherwise run by hand: versions, hypertables, background jobs,
continuous aggregates, top statements, and connections and locks. When something looks wrong,
a one-line finding starting with `!` sits above the section's table.

## Example, cut from `make demo`

    == Hypertables ==
    ! public.conditions has 29 chunks older than its retention of 30 days; check retention job 1001.
    ! public.page_views has 1201 chunks; a chunk interval longer than 01:00:00 would make fewer.
    hypertable           size     compressed  uncompressed  ratio  chunks              chunk interval  compression  retention         compress after
    public.conditions    17.7 MB  6.8 MB      10.9 MB       2.9x   61 (40 compressed)  1 day           on           30 days (paused)  7 days (paused)
    public.page_views    37.5 MB  -           -             -      1201                01:00:00        off          none              -

    == Continuous aggregates ==
    ! public.conditions_daily has no refresh policy, so it is only refreshed by hand.
    continuous aggregate      on                 real-time  refresh from  refresh to    every     materialised up to
    public.conditions_daily   public.conditions  on         no policy     -             -         20d 11h ago
    public.conditions_hourly  public.conditions  off        3 days ago    01:00:00 ago  01:00:00  1h 34m ago

## Run

    tsdoctor "postgres://user@host:5432/db"   # or DATABASE_URL, or the PG* variables as for psql
    tsdoctor --json ...                       # the same report as JSON
    tsdoctor --only hypertables,jobs ...      # versions, hypertables, jobs, caggs, statements, connections

It only reads: each section runs in its own read-only transaction with a statement timeout
(`--timeout`, 30s by default), and one that fails or that your version lacks shows a note.
Findings fire at over 1000 chunks, a job without success for 3 schedule intervals, a
transaction open over an hour, and connections above 80% of the maximum. Top statements need
`pg_stat_statements`; without the `pg_monitor` role, other users' queries show as
`<insufficient privilege>`.

## Tested on

The `timescale/timescaledb` images 2.32.0-pg17, 2.17.2-pg16 and 2.14.2-pg15, as a superuser,
as a `pg_monitor` user and as a user with no grants. Other versions are untested; where a view
or column the report needs is missing, that part is skipped with a note.

## Build

Go runs in Docker, nothing to install:

    make test    # go vet and go test
    make build   # bin/tsdoctor for macOS on Apple silicon
    make demo    # start TimescaleDB with testdata/seed.sql, print the report, remove the database
