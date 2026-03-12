package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

const mockEventsResponse = `{
	"data": [{
		"id": "event-1",
		"type": "event",
		"attributes": {
			"timestamp": "2024-01-15T10:30:00.000Z",
			"tags": ["env:prod", "service:web"],
			"attributes": {
				"title": "Deploy v2.1",
				"source_type_name": "github",
				"tags": ["env:prod", "service:web"]
			}
		}
	}],
	"meta": {"status": "done", "elapsed": 100, "request_id": "req-1"}
}`

func newTestEventsAPI(srv *httptest.Server) func() (*eventsAPI, error) {
	return func() (*eventsAPI, error) {
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
		return &eventsAPI{api: datadogV2.NewEventsApi(c), ctx: apiCtx}, nil
	}
}

func buildEventsListCmd(mkAPI func() (*eventsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	events := &cobra.Command{Use: "events"}
	events.AddCommand(newEventsListCmd(mkAPI))
	root.AddCommand(events)
	return root, buf
}

func TestEventsListFlagQuery(t *testing.T) {
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

	root, _ := buildEventsListCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "list", "--query", "source:github", "--limit", "100"})
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
	if got := q.Get("filter[query]"); got != "source:github" {
		t.Errorf("filter[query] = %q, want %q", got, "source:github")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestEventsListFlagFromTo(t *testing.T) {
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

	root, _ := buildEventsListCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "list", "--from", "now-1h", "--to", "now"})
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

func TestEventsListFlagSort(t *testing.T) {
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

	root, _ := buildEventsListCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "list", "--sort", "timestamp"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made")
	}
	if got := req.URL.Query().Get("sort"); got != "timestamp" {
		t.Errorf("sort = %q, want %q", got, "timestamp")
	}
}

func TestEventsListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsListCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "TITLE", "SOURCE", "TAGS", "Deploy v2.1", "github", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestEventsListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsListCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "list", "--json"})
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

func buildEventsSearchCmd(mkAPI func() (*eventsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	events := &cobra.Command{Use: "events"}
	events.AddCommand(newEventsSearchCmd(mkAPI))
	root.AddCommand(events)
	return root, buf
}

func TestEventsSearchFlagQuery(t *testing.T) {
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

	root, _ := buildEventsSearchCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "search", "--query", "deployment", "--limit", "20"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

func TestEventsSearchQueryRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildEventsSearchCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "search"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --query is missing")
	}
}

func TestEventsSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsSearchCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "search", "--query", "deployment"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "TITLE", "SOURCE", "TAGS", "Deploy v2.1", "github", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestEventsSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsSearchCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "search", "--query", "deployment", "--json"})
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

func buildEventsShowCmd(mkAPI func() (*eventsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	events := &cobra.Command{Use: "events"}
	events.AddCommand(newEventsShowCmd(mkAPI))
	root.AddCommand(events)
	return root, buf
}

const mockEventShowResponse = `{
	"data": {
		"id": "event-123",
		"type": "event",
		"attributes": {
			"timestamp": "2024-01-15T10:30:00.000Z",
			"tags": ["env:prod", "service:web"],
			"message": "Deploy completed successfully"
		}
	}
}`

func TestEventsShowDetail(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/event-123") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsShowCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "show", "event-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"event-123", "2024-01-15", "env:prod", "Deploy completed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestEventsShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsShowCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "show", "event-123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["id"] != "event-123" {
		t.Errorf("id = %v, want event-123", result["id"])
	}
}

func TestEventsShowNotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errors":["Not Found"]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildEventsShowCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "show", "nonexistent-id"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for non-existent event")
	}
}

func TestEventsShowIDRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildEventsShowCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when event ID is missing")
	}
}

func TestEventsListDefaultFrom(t *testing.T) {
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

	root, _ := buildEventsListCmd(newTestEventsAPI(srv))
	// no --from flag - should default to now-24h
	root.SetArgs([]string{"events", "list"})
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
