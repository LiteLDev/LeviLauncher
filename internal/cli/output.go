package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
	"time"
)

// Render writes a command's result to stdout in the requested form:
//
//   - --json: data encoded as one JSON document (a nil slice becomes []);
//   - -q: ids, one per line (names, IDs, paths, PIDs);
//   - otherwise human(w), which writes the human form to w.
//
// Give data a typed struct (or slice of structs) with camelCase json tags.
// ids may be nil for commands without identifiers; -q then prints nothing.
func (c *Ctx) Render(data any, ids []string, human func(w io.Writer) error) error {
	switch {
	case c.G.JSON:
		return c.JSON(data)
	case c.G.Quiet:
		for _, id := range ids {
			fmt.Fprintln(c.Stdout, id)
		}
		return nil
	case human != nil:
		return human(c.Stdout)
	}
	return nil
}

// JSON writes v to stdout as indented JSON.
func (c *Ctx) JSON(v any) error {
	if v == nil {
		v = struct{}{}
	} else if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		v = []struct{}{}
	}
	enc := json.NewEncoder(c.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Table writes aligned columns with a header row to w. Cells must not
// contain tabs or newlines.
func Table(w io.Writer, header []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(header) > 0 {
		fmt.Fprintln(tw, strings.Join(header, "\t"))
	}
	for _, r := range rows {
		cells := make([]string, len(r))
		for i, cell := range r {
			cells[i] = cleanCell(cell)
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// KV writes a key: value block with aligned values to w. pairs alternate
// key and value.
func KV(w io.Writer, pairs ...string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(tw, "%s:\t%s\n", pairs[i], cleanCell(pairs[i+1]))
	}
	return tw.Flush()
}

func cleanCell(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

// Dash returns s, or "-" when s is empty (for table cells).
func Dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// YesNo renders a boolean table cell.
func YesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Size formats a byte count with IEC units: 512 B, 1.5 KiB, 3.2 GiB.
func Size(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	f := float64(n)
	for _, u := range units {
		f /= 1024
		if f < 1024 || u == units[len(units)-1] {
			if f >= 100 {
				return fmt.Sprintf("%.0f %s", f, u)
			}
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return fmt.Sprintf("%d B", n)
}

// Ago renders a time as local time with a relative hint:
// "2024-05-01 14:03 (2 h ago)". The zero time is "-".
func Ago(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04") + " (" + relative(time.Since(t)) + ")"
}

func relative(d time.Duration) string {
	future := d < 0
	if future {
		d = -d
	}
	var s string
	switch {
	case d < time.Minute:
		s = "just now"
		return s
	case d < time.Hour:
		s = fmt.Sprintf("%d min", int(d/time.Minute))
	case d < 48*time.Hour:
		s = fmt.Sprintf("%d h", int(d/time.Hour))
	case d < 60*24*time.Hour:
		s = fmt.Sprintf("%d days", int(d/(24*time.Hour)))
	case d < 2*365*24*time.Hour:
		s = fmt.Sprintf("%d months", int(d/(30*24*time.Hour)))
	default:
		s = fmt.Sprintf("%d years", int(d/(365*24*time.Hour)))
	}
	if future {
		return "in " + s
	}
	return s + " ago"
}
