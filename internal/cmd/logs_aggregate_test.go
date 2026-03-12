package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const mockAggregateResponse = `{
	"data": {
		"buckets": [
			{
				"by": {"service": "web", "status": "info"},
				"computes": {"c0": 42}
			},
			{
				"by": {"service": "api", "status": "error"},
				"computes": {"c0": 7}
			}
		]
	}
}`

func buildAggregateCmd(mkAPI func() (*logsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	logs := &cobra.Command{Use: "logs"}
	agg := newLogsAggregateCmd(mkAPI)
	logs.AddCommand(agg)
	root.AddCommand(logs)
	return root, buf
}

func TestLogsAggregateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"buckets":[]}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAggregateCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{
		"logs", "aggregate",
		"--query", "service:web",
		"--from", "now-1h",
		"--to", "now",
		"--group-by", "service,status",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	filter, ok := req["filter"].(map[string]interface{})
	if !ok {
		t.Fatal("missing filter in request")
	}
	if got := filter["query"]; got != "service:web" {
		t.Errorf("filter.query = %v, want service:web", got)
	}
	if filter["from"] == nil {
		t.Error("filter.from should be set")
	}
	if filter["to"] == nil {
		t.Error("filter.to should be set")
	}
	groupBy, ok := req["group_by"].([]interface{})
	if !ok || len(groupBy) != 2 {
		t.Errorf("group_by should have 2 entries, got %v", req["group_by"])
	}
	compute, ok := req["compute"].([]interface{})
	if !ok || len(compute) != 1 {
		t.Errorf("compute should have 1 entry, got %v", req["compute"])
	}
}

func TestLogsAggregateTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAggregateCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{
		"logs", "aggregate",
		"--group-by", "service,status",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"SERVICE", "STATUS", "C0", "web", "info", "42", "api", "error", "7"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsAggregateJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAggregateCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{
		"logs", "aggregate",
		"--compute", "count",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
}
