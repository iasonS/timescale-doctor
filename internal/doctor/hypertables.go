package doctor

import (
	"context"
	"fmt"

	"github.com/iasonS/timescale-doctor/internal/catalog"
	"github.com/iasonS/timescale-doctor/internal/findings"
	"github.com/iasonS/timescale-doctor/internal/format"
	"github.com/iasonS/timescale-doctor/internal/report"
)

var hypertableColumns = []string{
	"hypertable", "size", "compressed", "uncompressed", "ratio", "chunks", "chunk interval",
	"compression", "retention", "compress after",
}

func hypertables(ctx context.Context, q catalog.Querier, server catalog.Server) (report.Section, error) {
	tables, err := catalog.ReadHypertables(ctx, q, server.Timescale)
	if err != nil {
		return report.Section{}, err
	}
	var section report.Section
	if err := catalog.AddCompressionStats(ctx, q, server.Timescale, tables); err != nil {
		section.Notes = append(section.Notes, "compressed sizes not shown: "+err.Error())
	}
	chunks, err := catalog.ReadChunks(ctx, q)
	if err != nil {
		return report.Section{}, err
	}
	section.Findings = findings.Hypertables(tables, chunks, server.Now)
	section.Columns = hypertableColumns
	for _, table := range tables {
		section.Rows = append(section.Rows, hypertableRow(table))
	}
	return section, nil
}

func hypertableRow(t catalog.Hypertable) []string {
	compressed, uncompressed, ratio := "-", "-", "-"
	chunks := fmt.Sprint(t.Chunks)
	if t.CompressionEnabled && t.Compression.Known {
		compressed = format.Bytes(t.Compression.BytesAfter)
		uncompressed = format.Bytes(max(t.TotalBytes-t.Compression.BytesAfter, 0))
		chunks = fmt.Sprintf("%d (%d compressed)", t.Chunks, t.Compression.Chunks)
	}
	if t.Compression.BytesAfter > 0 {
		ratio = fmt.Sprintf("%.1fx", float64(t.Compression.BytesBefore)/float64(t.Compression.BytesAfter))
	}
	return []string{
		t.FullName(), format.Bytes(t.TotalBytes), compressed, uncompressed, ratio, chunks,
		t.ChunkInterval, onOff(t.CompressionEnabled), policyText(t.RetentionPolicy), compressAfter(t),
	}
}

func compressAfter(t catalog.Hypertable) string {
	if !t.CompressionEnabled && t.CompressionPolicy == nil {
		return "-"
	}
	return policyText(t.CompressionPolicy)
}

func policyText(p *catalog.Policy) string {
	if p == nil {
		return "none"
	}
	if !p.Scheduled {
		return p.After + " (paused)"
	}
	return p.After
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
