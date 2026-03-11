package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// PrintTable writes headers and rows as aligned tab-separated columns.
func PrintTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	writeRow(tw, headers)
	for _, row := range rows {
		writeRow(tw, row)
	}
	return tw.Flush()
}

func writeRow(w io.Writer, cols []string) {
	for i, col := range cols {
		if i > 0 {
			fmt.Fprint(w, "\t") //nolint:errcheck
		}
		// sanitize tab and newline to avoid corrupting tabwriter layout
		col = strings.ReplaceAll(col, "\t", " ")
		col = strings.ReplaceAll(col, "\n", " ")
		fmt.Fprint(w, col) //nolint:errcheck
	}
	fmt.Fprintln(w) //nolint:errcheck
}

// PrintJSON writes v as indented JSON to w.
func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
