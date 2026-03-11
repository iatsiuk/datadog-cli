package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	return func() (*logsAPI, error) {
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
		return &logsAPI{api: datadogV2.NewLogsApi(c), ctx: ctx}, nil
	}
}

// buildSearchCmd sets up a root -> logs -> search command tree for testing.
func buildSearchCmd(mkAPI func() (*logsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
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

	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSearchCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{"logs", "search", "--query", "service:web", "--limit", "100"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedReq == nil {
		t.Fatal("no request made to mock server")
	}
	q := capturedReq.URL.Query()
	if got := q.Get("filter[query]"); got != "service:web" {
		t.Errorf("filter[query] = %q, want %q", got, "service:web")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestLogsSearchFlagFromTo(t *testing.T) {
	t.Parallel()

	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSearchCmd(newTestLogsAPI(srv))
	root.SetArgs([]string{"logs", "search", "--from", "now-1h", "--to", "now"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedReq == nil {
		t.Fatal("no request made")
	}
	q := capturedReq.URL.Query()
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

	var capturedReq *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
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
	if capturedReq == nil {
		t.Fatal("no request made")
	}
	if q := capturedReq.URL.Query().Get("filter[from]"); q == "" {
		t.Error("filter[from] should be set when --from is omitted")
	}
}
