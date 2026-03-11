package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestLogsMetricAPI(srv *httptest.Server) func() (*logsMetricAPI, error) {
	return func() (*logsMetricAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		ctx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &logsMetricAPI{api: datadogV2.NewLogsMetricsApi(c), ctx: ctx}, nil
	}
}

func buildMetricCmd(mkAPI func() (*logsMetricAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(newLogsMetricCmd(mkAPI))
	return root, buf
}

const mockMetricListResponse = `{
	"data": [
		{
			"type": "logs_metrics",
			"id": "web.requests",
			"attributes": {
				"compute": {"aggregation_type": "count"},
				"filter": {"query": "service:web"},
				"group_by": [{"path": "@env", "tag_name": "env"}]
			}
		},
		{
			"type": "logs_metrics",
			"id": "api.errors",
			"attributes": {
				"compute": {"aggregation_type": "count"},
				"filter": {"query": "service:api status:error"}
			}
		}
	]
}`

const mockMetricResponse = `{
	"data": {
		"type": "logs_metrics",
		"id": "web.requests",
		"attributes": {
			"compute": {"aggregation_type": "count"},
			"filter": {"query": "service:web"},
			"group_by": [{"path": "@env", "tag_name": "env"}]
		}
	}
}`

func TestLogsMetricListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "COMPUTE", "FILTER", "GROUP-BY", "web.requests", "api.errors", "count", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsMetricListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 2 {
		t.Errorf("got %d entries, want 2", len(result))
	}
}

func TestLogsMetricShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "show", "web.requests"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "COMPUTE", "FILTER", "web.requests", "count", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsMetricShowRequiresID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestLogsMetricCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{
		"metric", "create",
		"--id", "web.requests",
		"--compute-type", "count",
		"--filter", "service:web",
		"--group-by", "@env",
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
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data in request")
	}
	if got := data["id"]; got != "web.requests" {
		t.Errorf("id = %v, want web.requests", got)
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("missing attributes in request")
	}
	compute, ok := attrs["compute"].(map[string]interface{})
	if !ok {
		t.Fatal("missing compute in attributes")
	}
	if got := compute["aggregation_type"]; got != "count" {
		t.Errorf("aggregation_type = %v, want count", got)
	}
}

func TestLogsMetricCreateRequiresID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "create", "--compute-type", "count"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestLogsMetricUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{
		"metric", "update", "web.requests",
		"--filter", "service:web env:prod",
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
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data in request")
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("missing attributes in request")
	}
	filter, ok := attrs["filter"].(map[string]interface{})
	if !ok {
		t.Fatal("missing filter in attributes")
	}
	if got := filter["query"]; got != "service:web env:prod" {
		t.Errorf("filter.query = %v, want 'service:web env:prod'", got)
	}
}

func TestLogsMetricDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "delete", "web.requests"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestLogsMetricDeleteWithYes(t *testing.T) {
	t.Parallel()

	var deletedID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		deletedID = parts[len(parts)-1]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildMetricCmd(newTestLogsMetricAPI(srv))
	root.SetArgs([]string{"metric", "delete", "web.requests", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if deletedID != "web.requests" {
		t.Errorf("deleted metric id = %q, want %q", deletedID, "web.requests")
	}
	if !strings.Contains(buf.String(), "web.requests") {
		t.Errorf("output should mention deleted metric id:\n%s", buf.String())
	}
}
