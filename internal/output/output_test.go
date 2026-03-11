package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

func TestPrintTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		headers  []string
		rows     [][]string
		contains []string
	}{
		{
			name:     "headers only",
			headers:  []string{"NAME", "VALUE"},
			rows:     [][]string{},
			contains: []string{"NAME", "VALUE"},
		},
		{
			name:    "aligned columns",
			headers: []string{"NAME", "VALUE"},
			rows: [][]string{
				{"foo", "bar"},
				{"longer-name", "longer-value"},
			},
			contains: []string{"NAME", "VALUE", "foo", "bar", "longer-name", "longer-value"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			output.PrintTable(&buf, tc.headers, tc.rows)
			result := buf.String()
			for _, s := range tc.contains {
				if !strings.Contains(result, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    any
		contains string
	}{
		{
			name:     "indented json",
			value:    map[string]string{"key": "value"},
			contains: `"key": "value"`,
		},
		{
			name:     "nil value",
			value:    nil,
			contains: "null",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := output.PrintJSON(&buf, tc.value); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			result := buf.String()
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected output to contain %q, got:\n%s", tc.contains, result)
			}
		})
	}
}
