// Package findings turns catalog rows into one-line findings in plain words. Every function
// here is pure: it reads nothing and takes the current time as an argument.
package findings

import "time"

const (
	// ManyChunks is the chunk count above which a hypertable has too many chunks: query
	// planning and the background jobs slow down with every chunk.
	ManyChunks = 1000

	// StaleRuns is how many schedule intervals a job may go without succeeding.
	StaleRuns = 3

	// LongTransaction is the age above which an open transaction is worth a look: it holds
	// back vacuum and can block compression and retention.
	LongTransaction = time.Hour

	// BusyConnections is the share of max_connections above which a server is close to
	// refusing new connections.
	BusyConnections = 0.8
)
