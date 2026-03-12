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
	"sync"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

const mockRUMEventsResponse = `{
	"data": [{
		"id": "rum-event-1",
		"type": "rum",
		"attributes": {
			"timestamp": "2024-01-15T10:30:00.000Z",
			"service": "web-store",
			"attributes": {
				"type": "view",
				"application.id": "app-123",
				"view.url": "https://example.com/home",
				"duration": "1234000000"
			}
		}
	}],
	"meta": {"status": "done", "elapsed": 100, "request_id": "req-1"}
}`

func newTestRUMAPI(srv *httptest.Server) func() (*rumAPI, error) {
	return func() (*rumAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &rumAPI{api: datadogV2.NewRUMApi(c), ctx: apiCtx}, nil
	}
}

func buildRUMSearchCmd(mkAPI func() (*rumAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	rum := &cobra.Command{Use: "rum"}
	rum.AddCommand(newRUMSearchCmd(mkAPI))
	root.AddCommand(rum)
	return root, buf
}

func TestRUMSearchFlagQuery(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMSearchCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "search", "--query", "type:view", "--limit", "100"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	q := req.URL.Query()
	if got := q.Get("filter[query]"); got != "type:view" {
		t.Errorf("filter[query] = %q, want %q", got, "type:view")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestRUMSearchFlagFromTo(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMSearchCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "search", "--from", "now-1h", "--to", "now"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made")
	}
	q := req.URL.Query()
	if q.Get("filter[from]") == "" {
		t.Error("filter[from] should be set")
	}
	if q.Get("filter[to]") == "" {
		t.Error("filter[to] should be set")
	}
}

func TestRUMSearchFlagSort(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMSearchCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "search", "--sort", "-timestamp"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made")
	}
	if got := req.URL.Query().Get("sort"); got != "-timestamp" {
		t.Errorf("sort = %q, want %q", got, "-timestamp")
	}
}

func TestRUMSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMSearchCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "TYPE", "APPLICATION", "VIEW", "DURATION",
		"view", "app-123", "https://example.com/home"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMSearchCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "search", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestRUMSearchDefaultFrom(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMSearchCmd(newTestRUMAPI(srv))
	// no --from flag - should default to now-15m
	root.SetArgs([]string{"rum", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made")
	}
	if q := req.URL.Query().Get("filter[from]"); q == "" {
		t.Error("filter[from] should be set when --from is omitted")
	}
}

const mockRUMAggregateResponse = `{
	"data": {
		"buckets": [
			{
				"by": {"@view.url": "https://example.com/home"},
				"computes": {"c0": 5}
			},
			{
				"by": {"@view.url": "https://example.com/about"},
				"computes": {"c0": 2}
			}
		]
	}
}`

func buildRUMAggregateCmd(mkAPI func() (*rumAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	rum := &cobra.Command{Use: "rum"}
	rum.AddCommand(newRUMAggregateCmd(mkAPI))
	root.AddCommand(rum)
	return root, buf
}

func TestRUMAggregateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"buckets":[]}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMAggregateCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{
		"rum", "aggregate",
		"--query", "type:error",
		"--from", "now-1h",
		"--to", "now",
		"--group-by", "@view.url",
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
	if got := filter["query"]; got != "type:error" {
		t.Errorf("filter.query = %v, want type:error", got)
	}
	if filter["from"] == nil {
		t.Error("filter.from should be set")
	}
	if filter["to"] == nil {
		t.Error("filter.to should be set")
	}
	groupBy, ok := req["group_by"].([]interface{})
	if !ok || len(groupBy) != 1 {
		t.Errorf("group_by should have 1 entry, got %v", req["group_by"])
	}
	compute, ok := req["compute"].([]interface{})
	if !ok || len(compute) != 1 {
		t.Errorf("compute should have 1 entry, got %v", req["compute"])
	}
}

func TestRUMAggregateTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMAggregateCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{
		"rum", "aggregate",
		"--group-by", "@view.url",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"@VIEW.URL", "C0", "https://example.com/home", "5"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMAggregateJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMAggregateCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{
		"rum", "aggregate",
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

// rum app tests

const mockRUMAppListResponse = `{
	"data": [{
		"id": "app-id-1",
		"type": "rum_application",
		"attributes": {
			"application_id": "app-id-1",
			"name": "Web Store",
			"type": "browser",
			"created_at": 1700000000000,
			"created_by_handle": "admin@example.com",
			"updated_at": 1700000001000,
			"updated_by_handle": "admin@example.com",
			"org_id": 12345
		}
	}]
}`

const mockRUMAppResponse = `{
	"data": {
		"id": "app-id-1",
		"type": "rum_application",
		"attributes": {
			"application_id": "app-id-1",
			"name": "Web Store",
			"type": "browser",
			"client_token": "pub123abc",
			"created_at": 1700000000000,
			"created_by_handle": "admin@example.com",
			"updated_at": 1700000001000,
			"updated_by_handle": "admin@example.com",
			"org_id": 12345
		}
	}
}`

func buildRUMAppCmd(mkAPI func() (*rumAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	rum := &cobra.Command{Use: "rum"}
	rum.AddCommand(newRUMAppCmd(mkAPI))
	root.AddCommand(rum)
	return root, buf
}

func TestRUMAppListTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMAppListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMAppCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "app", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "TYPE", "app-id-1", "Web Store", "browser"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMAppShowTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMAppResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMAppCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "app", "show", "app-id-1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"app-id-1", "Web Store", "browser", "pub123abc"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMAppCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMAppResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMAppCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "app", "create", "--name", "My App", "--type", "ios"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := req["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "My App" {
		t.Errorf("name = %v, want My App", got)
	}
	if got := attrs["type"]; got != "ios" {
		t.Errorf("type = %v, want ios", got)
	}
}

func TestRUMAppUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMAppResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMAppCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "app", "update", "app-id-1", "--name", "Updated App"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := req["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "Updated App" {
		t.Errorf("name = %v, want Updated App", got)
	}
	if got := data["id"]; got != "app-id-1" {
		t.Errorf("id = %v, want app-id-1", got)
	}
}

func TestRUMAppDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMAppCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "app", "delete", "app-id-1"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestRUMAppDeleteWithYes(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMAppCmd(newTestRUMAPI(srv))
	root.SetArgs([]string{"rum", "app", "delete", "app-id-1", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("expected DELETE request to be made")
	}
}

// rum metric tests

const mockRUMMetricsListResponse = `{
	"data": [{
		"id": "my.rum.metric",
		"type": "rum_metrics",
		"attributes": {
			"event_type": "view",
			"compute": {
				"aggregation_type": "count"
			},
			"filter": {
				"query": "@view.url:*"
			},
			"group_by": [{"path": "@view.url"}]
		}
	}]
}`

const mockRUMMetricResponse = `{
	"data": {
		"id": "my.rum.metric",
		"type": "rum_metrics",
		"attributes": {
			"event_type": "view",
			"compute": {
				"aggregation_type": "count"
			},
			"filter": {
				"query": "@view.url:*"
			},
			"group_by": [{"path": "@view.url"}]
		}
	}
}`

func newTestRUMMetricsAPI(srv *httptest.Server) func() (*rumMetricsAPI, error) {
	return func() (*rumMetricsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &rumMetricsAPI{api: datadogV2.NewRumMetricsApi(c), ctx: apiCtx}, nil
	}
}

func buildRUMMetricCmd(mkAPI func() (*rumMetricsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	rum := &cobra.Command{Use: "rum"}
	rum.AddCommand(newRUMMetricCmd(mkAPI))
	root.AddCommand(rum)
	return root, buf
}

func TestRUMMetricListTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMMetricsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMMetricCmd(newTestRUMMetricsAPI(srv))
	root.SetArgs([]string{"rum", "metric", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "EVENT TYPE", "COMPUTE", "FILTER", "my.rum.metric", "view", "count", "@view.url:*"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMMetricShowTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMMetricCmd(newTestRUMMetricsAPI(srv))
	root.SetArgs([]string{"rum", "metric", "show", "my.rum.metric"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"my.rum.metric", "view", "count", "@view.url:*"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMMetricCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMMetricCmd(newTestRUMMetricsAPI(srv))
	root.SetArgs([]string{
		"rum", "metric", "create",
		"--id", "my.rum.metric",
		"--compute", "count",
		"--event-type", "view",
		"--filter", "@view.url:*",
		"--group-by", "@view.url",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := req["data"].(map[string]interface{})
	if got := data["id"]; got != "my.rum.metric" {
		t.Errorf("id = %v, want my.rum.metric", got)
	}
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["event_type"]; got != "view" {
		t.Errorf("event_type = %v, want view", got)
	}
	compute, _ := attrs["compute"].(map[string]interface{})
	if got := compute["aggregation_type"]; got != "count" {
		t.Errorf("aggregation_type = %v, want count", got)
	}
	filter, _ := attrs["filter"].(map[string]interface{})
	if got := filter["query"]; got != "@view.url:*" {
		t.Errorf("filter.query = %v, want @view.url:*", got)
	}
}

func TestRUMMetricUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMMetricResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMMetricCmd(newTestRUMMetricsAPI(srv))
	root.SetArgs([]string{
		"rum", "metric", "update", "my.rum.metric",
		"--filter", "@view.url:/checkout",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := req["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	filter, _ := attrs["filter"].(map[string]interface{})
	if got := filter["query"]; got != "@view.url:/checkout" {
		t.Errorf("filter.query = %v, want @view.url:/checkout", got)
	}
}

func TestRUMMetricDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMMetricCmd(newTestRUMMetricsAPI(srv))
	root.SetArgs([]string{"rum", "metric", "delete", "my.rum.metric"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestRUMMetricDeleteWithYes(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMMetricCmd(newTestRUMMetricsAPI(srv))
	root.SetArgs([]string{"rum", "metric", "delete", "my.rum.metric", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("expected DELETE request to be made")
	}
}

// rum retention-filter tests

const mockRUMRetentionFiltersListResponse = `{
	"data": [{
		"id": "rf-id-1",
		"type": "retention_filters",
		"attributes": {
			"name": "Keep Errors",
			"event_type": "error",
			"sample_rate": 100,
			"enabled": true,
			"query": "@error.type:*"
		}
	}]
}`

const mockRUMRetentionFilterResponse = `{
	"data": {
		"id": "rf-id-1",
		"type": "retention_filters",
		"attributes": {
			"name": "Keep Errors",
			"event_type": "error",
			"sample_rate": 100,
			"enabled": true,
			"query": "@error.type:*"
		}
	}
}`

func newTestRUMRetentionFiltersAPI(srv *httptest.Server) func() (*rumRetentionFiltersAPI, error) {
	return func() (*rumRetentionFiltersAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &rumRetentionFiltersAPI{api: datadogV2.NewRumRetentionFiltersApi(c), ctx: apiCtx}, nil
	}
}

func buildRUMRetentionFilterCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	rum := &cobra.Command{Use: "rum"}
	rum.AddCommand(newRUMRetentionFilterCmd(mkAPI))
	root.AddCommand(rum)
	return root, buf
}

func TestRUMRetentionFilterListTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMRetentionFiltersListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{"rum", "retention-filter", "list", "--app", "app-id-1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "EVENT TYPE", "SAMPLE RATE", "ENABLED",
		"rf-id-1", "Keep Errors", "error", "100", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMRetentionFilterListRequiresApp(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMRetentionFiltersListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{"rum", "retention-filter", "list"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --app flag")
	}
}

func TestRUMRetentionFilterShowTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMRetentionFilterResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{"rum", "retention-filter", "show", "--app", "app-id-1", "rf-id-1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"rf-id-1", "Keep Errors", "error", "100", "true", "@error.type:*"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMRetentionFilterCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMRetentionFilterResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{
		"rum", "retention-filter", "create",
		"--app", "app-id-1",
		"--name", "Keep Errors",
		"--event-type", "error",
		"--sample-rate", "100",
		"--query", "@error.type:*",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := req["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "Keep Errors" {
		t.Errorf("name = %v, want Keep Errors", got)
	}
	if got := attrs["event_type"]; got != "error" {
		t.Errorf("event_type = %v, want error", got)
	}
	if got := attrs["sample_rate"]; got != float64(100) {
		t.Errorf("sample_rate = %v, want 100", got)
	}
	if got := attrs["query"]; got != "@error.type:*" {
		t.Errorf("query = %v, want @error.type:*", got)
	}
}

func TestRUMRetentionFilterUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRUMRetentionFilterResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{
		"rum", "retention-filter", "update", "rf-id-1",
		"--app", "app-id-1",
		"--name", "Updated Filter",
		"--sample-rate", "50",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := req["data"].(map[string]interface{})
	if got := data["id"]; got != "rf-id-1" {
		t.Errorf("id = %v, want rf-id-1", got)
	}
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "Updated Filter" {
		t.Errorf("name = %v, want Updated Filter", got)
	}
	if got := attrs["sample_rate"]; got != float64(50) {
		t.Errorf("sample_rate = %v, want 50", got)
	}
}

func TestRUMRetentionFilterDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{"rum", "retention-filter", "delete", "--app", "app-id-1", "rf-id-1"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestRUMRetentionFilterDeleteWithYes(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMRetentionFilterCmd(newTestRUMRetentionFiltersAPI(srv))
	root.SetArgs([]string{"rum", "retention-filter", "delete", "--app", "app-id-1", "rf-id-1", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("expected DELETE request to be made")
	}
}

// --- playlist tests ---

const mockPlaylistListResponse = `{
	"data": [
		{
			"id": "42",
			"type": "rum_replay_playlist",
			"attributes": {
				"name": "My Playlist",
				"description": "test desc",
				"session_count": 3,
				"created_at": "2024-01-15T10:00:00Z"
			}
		}
	]
}`

const mockPlaylistResponse = `{
	"data": {
		"id": "42",
		"type": "rum_replay_playlist",
		"attributes": {
			"name": "My Playlist",
			"description": "test desc",
			"session_count": 3,
			"created_at": "2024-01-15T10:00:00Z"
		}
	}
}`

const mockPlaylistSessionsResponse = `{
	"data": [
		{
			"id": "sess-abc",
			"type": "rum_replay_session"
		}
	]
}`

func newTestRUMPlaylistsAPI(srv *httptest.Server) func() (*rumPlaylistsAPI, error) {
	return func() (*rumPlaylistsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &rumPlaylistsAPI{api: datadogV2.NewRumReplayPlaylistsApi(c), ctx: apiCtx}, nil
	}
}

func buildRUMPlaylistCmd(mkAPI func() (*rumPlaylistsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	rum := &cobra.Command{Use: "rum"}
	rum.AddCommand(newRUMPlaylistCmd(mkAPI))
	root.AddCommand(rum)
	return root, buf
}

func TestRUMPlaylistListTable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPlaylistListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "SESSIONS", "CREATED", "My Playlist", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMPlaylistListJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPlaylistListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestRUMPlaylistShow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPlaylistResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "show", "42"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"42", "My Playlist", "test desc"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRUMPlaylistShowInvalidID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "show", "not-an-int"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-integer playlist id")
	}
}

func TestRUMPlaylistCreate(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPlaylistResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "create", "--name", "My Playlist"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(string(capturedBody), "My Playlist") {
		t.Errorf("request body missing name: %s", capturedBody)
	}
	if !strings.Contains(buf.String(), "My Playlist") {
		t.Errorf("output missing name:\n%s", buf.String())
	}
}

func TestRUMPlaylistCreateRequiresName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --name flag")
	}
}

func TestRUMPlaylistUpdate(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPlaylistResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "update", "42", "--name", "Updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// should make GET then PUT (2 calls)
	if callCount < 2 {
		t.Errorf("expected at least 2 API calls, got %d", callCount)
	}
	_ = buf.String()
}

func TestRUMPlaylistDelete(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "delete", "42", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("expected DELETE request to be made")
	}
}

func TestRUMPlaylistDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "delete", "42"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestRUMPlaylistSessions(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPlaylistSessionsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "sessions", "42"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "sess-abc") {
		t.Errorf("output missing session id:\n%s", out)
	}
}

func TestRUMPlaylistAddSession(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":"sess-abc","type":"rum_replay_session"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "add-session", "42", "sess-abc"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()

	if req == nil {
		t.Fatal("no request made")
	}
	if !strings.Contains(req.URL.Path, "42") {
		t.Errorf("path missing playlist id: %s", req.URL.Path)
	}
	if !strings.Contains(req.URL.Path, "sess-abc") {
		t.Errorf("path missing session id: %s", req.URL.Path)
	}
	if !strings.Contains(buf.String(), "added") {
		t.Errorf("output missing 'added':\n%s", buf.String())
	}
}

func TestRUMPlaylistRemoveSession(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildRUMPlaylistCmd(newTestRUMPlaylistsAPI(srv))
	root.SetArgs([]string{"rum", "playlist", "remove-session", "42", "sess-abc"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("expected DELETE request to be made")
	}
	if !strings.Contains(buf.String(), "removed") {
		t.Errorf("output missing 'removed':\n%s", buf.String())
	}
}
