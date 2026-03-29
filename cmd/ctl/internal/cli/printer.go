package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Printer handles output formatting (table or JSON).
type Printer struct {
	jsonMode bool
}

func NewPrinter(jsonMode bool) *Printer {
	return &Printer{jsonMode: jsonMode}
}

// JSON pretty-prints v as JSON.
func (p *Printer) JSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// Table prints rows with a tab-separated header.
// header: []string{"NAME", "OWNER", ...}
// rows:   [][]string{{"repo1", "admin", ...}, ...}
func (p *Printer) Table(header []string, rows [][]string) {
	if p.jsonMode {
		// Convert table to list of maps
		var out []map[string]string
		for _, row := range rows {
			m := make(map[string]string, len(header))
			for i, h := range header {
				if i < len(row) {
					m[strings.ToLower(h)] = row[i]
				}
			}
			out = append(out, m)
		}
		p.JSON(out)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	fmt.Fprintln(w, strings.Repeat("-", 4*len(header))+"\t") // separator line approximation
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// KV prints a key-value list.
func (p *Printer) KV(pairs [][2]string) {
	if p.jsonMode {
		m := make(map[string]string, len(pairs))
		for _, kv := range pairs {
			m[kv[0]] = kv[1]
		}
		p.JSON(m)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, kv := range pairs {
		fmt.Fprintf(w, "%s:\t%s\n", kv[0], kv[1])
	}
	w.Flush()
}

// Success prints a success message.
func (p *Printer) Success(format string, args ...any) {
	if p.jsonMode {
		p.JSON(map[string]string{"status": "ok", "message": fmt.Sprintf(format, args...)})
		return
	}
	fmt.Printf("OK  "+format+"\n", args...)
}

// Raw prints raw JSON bytes (re-indented if not JSON mode).
func (p *Printer) Raw(data []byte) {
	var v any
	if json.Unmarshal(data, &v) == nil {
		p.JSON(v)
	} else {
		os.Stdout.Write(data)
	}
}
