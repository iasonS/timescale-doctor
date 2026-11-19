// Package format writes sizes, durations and query text short enough for one table cell.
package format

import (
	"fmt"
	"strings"
	"time"
)

// Bytes writes a size with binary units, as pg_size_pretty does: "812 bytes", "7.7 MB".
func Bytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d bytes", n)
	}
	units := []string{"kB", "MB", "GB", "TB", "PB"}
	value, i := float64(n)/1024, 0
	for value >= 1024 && i < len(units)-1 {
		value /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", value, units[i])
}

// Duration writes the two largest units of a duration: "45s", "12m", "3h 5m", "2d 4h".
func Duration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	seconds := int64(d / time.Second)
	days, hours := seconds/86400, seconds%86400/3600
	minutes, rest := seconds%3600/60, seconds%60
	switch {
	case days > 0:
		return pair(days, "d", hours, "h")
	case hours > 0:
		return pair(hours, "h", minutes, "m")
	case minutes > 0:
		return pair(minutes, "m", rest, "s")
	}
	return fmt.Sprintf("%ds", rest)
}

func pair(big int64, bigUnit string, small int64, smallUnit string) string {
	if small == 0 {
		return fmt.Sprintf("%d%s", big, bigUnit)
	}
	return fmt.Sprintf("%d%s %d%s", big, bigUnit, small, smallUnit)
}

// Relative writes a time relative to now, "3m ago" or "in 5m", and ifNil when there is none.
func Relative(t *time.Time, now time.Time, ifNil string) string {
	if t == nil {
		return ifNil
	}
	if t.After(now) {
		return "in " + Duration(t.Sub(now))
	}
	return Duration(now.Sub(*t)) + " ago"
}

// Millis writes a time in milliseconds with a unit that fits: "0.42 ms", "12.3 s".
func Millis(ms float64) string {
	switch {
	case ms < 1:
		return fmt.Sprintf("%.2f ms", ms)
	case ms < 1000:
		return fmt.Sprintf("%.1f ms", ms)
	case ms < 60_000:
		return fmt.Sprintf("%.1f s", ms/1000)
	}
	return Duration(time.Duration(ms * float64(time.Millisecond)))
}

// OneLine puts a query on one line and cuts it to at most width characters.
func OneLine(query string, width int) string {
	line := strings.Join(strings.Fields(query), " ")
	runes := []rune(line)
	if len(runes) <= width {
		return line
	}
	return string(runes[:width-3]) + "..."
}

// Count writes a number with a noun that agrees with it: "1 chunk", "5 chunks".
func Count(n int64, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
