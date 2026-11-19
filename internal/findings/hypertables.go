package findings

import (
	"fmt"
	"time"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/format"
)

// Hypertables finds missing policies, too many chunks, and chunks that the policies should
// already have dropped or compressed.
func Hypertables(tables []catalog.Hypertable, chunks []catalog.Chunk, now time.Time) []string {
	byTable := map[string][]catalog.Chunk{}
	for _, chunk := range chunks {
		byTable[chunk.Hypertable] = append(byTable[chunk.Hypertable], chunk)
	}
	var found []string
	for _, table := range tables {
		found = append(found, hypertable(table, byTable[table.FullName()], now)...)
	}
	return found
}

func hypertable(table catalog.Hypertable, chunks []catalog.Chunk, now time.Time) []string {
	name := table.FullName()
	var found []string
	if table.RetentionPolicy == nil {
		found = append(found, name+" has no retention policy, so its data is never dropped.")
	}
	if table.CompressionEnabled && table.CompressionPolicy == nil {
		found = append(found, name+" has compression on but no compression policy, so new chunks stay uncompressed.")
	}
	if table.Chunks > ManyChunks {
		found = append(found, fmt.Sprintf("%s has %d chunks; a chunk interval longer than %s would make fewer.",
			name, table.Chunks, table.ChunkInterval))
	}
	if table.RetentionPolicy != nil {
		if old := overdue(*table.RetentionPolicy, chunks, now); len(old) > 0 {
			found = append(found, fmt.Sprintf("%s has %s older than its retention of %s; check retention job %d.",
				name, format.Count(int64(len(old)), "chunk"), table.RetentionPolicy.After, table.RetentionPolicy.JobID))
		}
	}
	if table.CompressionPolicy != nil {
		if uncompressed := countUncompressed(overdue(*table.CompressionPolicy, chunks, now)); uncompressed > 0 {
			found = append(found, fmt.Sprintf("%s has %s older than its compression policy's %s; check compression job %d.",
				name, format.Count(uncompressed, "uncompressed chunk"), table.CompressionPolicy.After, table.CompressionPolicy.JobID))
		}
	}
	return found
}

// overdue returns the chunks that ended longer ago than the policy's age plus one schedule
// interval: the policy only acts when it runs, so a chunk may wait that long.
func overdue(policy catalog.Policy, chunks []catalog.Chunk, now time.Time) []catalog.Chunk {
	if policy.AfterAge == nil {
		return nil
	}
	cutoff := now.Add(-*policy.AfterAge - policy.Schedule)
	var old []catalog.Chunk
	for _, chunk := range chunks {
		if chunk.End.Before(cutoff) {
			old = append(old, chunk)
		}
	}
	return old
}

func countUncompressed(chunks []catalog.Chunk) int64 {
	count := int64(0)
	for _, chunk := range chunks {
		if !chunk.Compressed {
			count++
		}
	}
	return count
}
