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

const mockLogsResponse = `{
	"data": [{
		"id": "log-1",
		"type": "log",
		"attributes": {
			"timestamp": "2024-01-15T10:30:00.000Z",
			"service": "web-service",
			"host": "web-01",
			"status": "info",
			"message": "Test log message"
		}
	}],
	"meta": {"status": "done", "elapsed": 100, "request_id": "req-1"}
}`

func newTestLogsAPI(srv *httptest.Server) func() (*logsAPI, error) {
	return newTestLogsAPIWithCtx(srv, context.Background())
}

func newTestLogsAPIWithCtx(srv *httptest.Server, ctx context.Context) func() (*logsAPI, error) {
	return func() (*logsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			ctx,
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &logsAPI{api: datadogV2.NewLogsApi(c), ctx: apiCtx}, nil
	}
}

// buildSearchCmd sets up a root -> logs -> search command tree for testing.
func buildSearchCmd(mkAPI func() (*logsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	logs := &cobra.Command{Use: "logs"}
	search := newLogsSearchCmd(mkAPI)
	logs.AddCommand(search)
	root.AddCommand(logs)
	return root, buf
}

func TestLogsSearchFlagQuery(t *testing.T) {
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

	root, _ := buildSearchCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{"logs", "search", "--query", "service:web", "--limit", "100"})
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
	if got := q.Get("filter[query]"); got != "service:web" {
		t.Errorf("filter[query] = %q, want %q", got, "service:web")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestLogsSearchFlagFromTo(t *testing.T) {
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

	root, _ := buildSearchCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{"logs", "search", "--from", "now-1h", "--to", "now"})
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

func TestLogsSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockLogsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSearchCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{"logs", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "SERVICE", "HOST", "STATUS", "MESSAGE",
		"web-service", "web-01", "info", "Test log message"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockLogsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSearchCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{"logs", "search", "--json"})
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

func TestLogsSearchDefaultFrom(t *testing.T) {
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

	root, _ := buildSearchCmd(newTestLogsAPI(srv))
	// no --from flag - should default to now-15m
	root.SetArgs([]string{"logs", "search"})
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

// buildTailCmd sets up a root -> logs -> tail command tree for testing.
func buildTailCmd(mkAPI func() (*logsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	logs := &cobra.Command{Use: "logs"}
	tail := newLogsTailCmd(mkAPI)
	logs.AddCommand(tail)
	root.AddCommand(logs)
	return root, buf
}

func TestLogsTailFlagQuery(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		cancel()
	}))
	defer srv.Close()

	root, _ := buildTailCmd(newTestLogsAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"logs", "tail", "--query", "service:web", "--interval", "1ms"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("filter[query]"); got != "service:web" {
		t.Errorf("filter[query] = %q, want %q", got, "service:web")
	}
}

func TestLogsTailFlagService(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		cancel()
	}))
	defer srv.Close()

	root, _ := buildTailCmd(newTestLogsAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"logs", "tail", "--service", "my-svc", "--interval", "1ms"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("filter[query]"); !strings.Contains(got, `service:"my-svc"`) {
		t.Errorf("filter[query] = %q, want it to contain %q", got, `service:"my-svc"`)
	}
}

func TestLogsTailPolling(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		cc := callCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if cc == 1 {
			fmt.Fprint(w, mockLogsResponse) //nolint:errcheck
		} else {
			fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
			cancel()
		}
	}))
	defer srv.Close()

	root, buf := buildTailCmd(newTestLogsAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"logs", "tail", "--interval", "1ms"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	calls := callCount
	mu.Unlock()
	if calls < 2 {
		t.Errorf("expected at least 2 API calls for polling, got %d", calls)
	}

	out := buf.String()
	for _, want := range []string{"web-service", "info", "Test log message"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
