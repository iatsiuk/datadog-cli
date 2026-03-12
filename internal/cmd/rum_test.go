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
