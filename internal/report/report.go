// Package report holds the health report and writes it as text or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Report is the whole health report.
type Report struct {
	Sections []Section `json:"sections"`
}

// Section is one part of the report: findings first, then a table.
type Section struct {
	Name     string     `json:"name"`
	Title    string     `json:"title"`
	Findings []string   `json:"findings"`
	Notes    []string   `json:"notes,omitempty"` // what could not be read, and why
	Columns  []string   `json:"columns,omitempty"`
	Rows     [][]string `json:"rows,omitempty"`
}

// WriteJSON writes the report as indented JSON.
func WriteJSON(w io.Writer, r Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// WriteText writes the report for a terminal: a heading per section, its findings, notes
// and table.
func WriteText(w io.Writer, r Report) error {
	for i, section := range r.Sections {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if err := writeSection(w, section); err != nil {
			return err
		}
	}
	return nil
}

func writeSection(w io.Writer, s Section) error {
	fmt.Fprintf(w, "== %s ==\n", s.Title)
	for _, finding := range s.Findings {
		fmt.Fprintf(w, "! %s\n", finding)
	}
	for _, note := range s.Notes {
		fmt.Fprintf(w, "note: %s\n", note)
	}
	if len(s.Rows) == 0 {
		if len(s.Notes) == 0 {
			fmt.Fprintln(w, "(none)")
		}
		return nil
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, strings.Join(s.Columns, "\t"))
	for _, row := range s.Rows {
		fmt.Fprintln(table, strings.Join(row, "\t"))
	}
	return table.Flush()
}
