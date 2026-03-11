package output

import (
	"encoding/json"
	"io"
	"text/tabwriter"
)

// PrintTable writes headers and rows as aligned tab-separated columns.
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	writeRow(tw, headers)
	for _, row := range rows {
		writeRow(tw, row)
	}
	tw.Flush() //nolint:errcheck
}

func writeRow(w io.Writer, cols []string) {
	for i, col := range cols {
		if i > 0 {
			io.WriteString(w, "\t") //nolint:errcheck
		}
		io.WriteString(w, col) //nolint:errcheck
	}
	io.WriteString(w, "\n") //nolint:errcheck
}

// PrintJSON writes v as indented JSON to w.
func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
