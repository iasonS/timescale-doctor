package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Connections are the client sessions of the whole server.
type Connections struct {
	Max     int64
	ByState []StateCount
	Longest *Session // session with the oldest open transaction, nil when none
	Waiting []Session
}

// StateCount is the number of client sessions in one state.
type StateCount struct {
	State string
	Count int64
}

// Session is one client session.
type Session struct {
	PID       int32
	User      string
	State     string
	Age       time.Duration // how long its transaction (or, when waiting, its query) has run
	Query     string
	BlockedBy []int32
}

// tsdoctor's own session is left out everywhere.
const byStateSQL = `
select coalesce(state, 'unknown'), count(*)
from pg_stat_activity
where backend_type = 'client backend' and pid <> pg_backend_pid()
group by 1
order by 2 desc, 1`

const longestSQL = `
select pid,
       coalesce(usename::text, ''),
       coalesce(state, ''),
       extract(epoch from now() - xact_start)::float8,
       coalesce(query, '')
from pg_stat_activity
where backend_type = 'client backend' and pid <> pg_backend_pid() and xact_start is not null
order by xact_start
limit 1`

const waitingSQL = `
select pid,
       coalesce(usename::text, ''),
       coalesce(state, ''),
       extract(epoch from now() - coalesce(query_start, now()))::float8,
       coalesce(query, ''),
       pg_blocking_pids(pid)
from pg_stat_activity
where wait_event_type = 'Lock' and pid <> pg_backend_pid()
order by query_start`

// ReadConnections reads the client sessions by state, the oldest open transaction and the
// sessions waiting on a lock.
func ReadConnections(ctx context.Context, q Querier) (Connections, error) {
	var c Connections
	if err := q.QueryRow(ctx, "select current_setting('max_connections')::bigint").Scan(&c.Max); err != nil {
		return c, err
	}
	byState, err := readByState(ctx, q)
	if err != nil {
		return c, err
	}
	c.ByState = byState
	if c.Longest, err = readLongest(ctx, q); err != nil {
		return c, err
	}
	c.Waiting, err = readWaiting(ctx, q)
	return c, err
}

func readByState(ctx context.Context, q Querier) ([]StateCount, error) {
	rows, err := q.Query(ctx, byStateSQL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (StateCount, error) {
		var s StateCount
		err := row.Scan(&s.State, &s.Count)
		return s, err
	})
}

func readLongest(ctx context.Context, q Querier) (*Session, error) {
	var s Session
	var age float64
	err := q.QueryRow(ctx, longestSQL).Scan(&s.PID, &s.User, &s.State, &age, &s.Query)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Age = seconds(age)
	return &s, nil
}

func readWaiting(ctx context.Context, q Querier) ([]Session, error) {
	rows, err := q.Query(ctx, waitingSQL)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		var s Session
		var age float64
		err := row.Scan(&s.PID, &s.User, &s.State, &age, &s.Query, &s.BlockedBy)
		s.Age = seconds(age)
		return s, err
	})
}
