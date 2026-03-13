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

const mockSignalsResponse = `{
	"data": [{
		"id": "signal-abc123",
		"type": "signal",
		"attributes": {
			"timestamp": "2024-01-15T10:30:00.000Z",
			"message": "Unauthorized access detected",
			"tags": ["severity:high", "source:cloudtrail"],
			"custom": {
				"status": "open",
				"workflow.rule.name": "AWS CloudTrail Rule"
			}
		}
	}]
}`

func newTestSecurityAPI(srv *httptest.Server) func() (*securityAPI, error) {
	return newTestSecurityAPIWithCtx(srv, context.Background())
}

func newTestSecurityAPIWithCtx(srv *httptest.Server, ctx context.Context) func() (*securityAPI, error) {
	return func() (*securityAPI, error) {
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
		return &securityAPI{api: datadogV2.NewSecurityMonitoringApi(c), ctx: apiCtx}, nil
	}
}

func buildSignalSearchCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sig := &cobra.Command{Use: "signal"}
	sig.AddCommand(newSecuritySignalSearchCmd(mkAPI))
	sec.AddCommand(sig)
	root.AddCommand(sec)
	return root, buf
}

func TestSecuritySignalSearchFlagQuery(t *testing.T) {
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
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalSearchCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "search", "--query", "source:cloudtrail", "--limit", "100"})
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
	if got := q.Get("filter[query]"); got != "source:cloudtrail" {
		t.Errorf("filter[query] = %q, want %q", got, "source:cloudtrail")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestSecuritySignalSearchFlagFromTo(t *testing.T) {
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
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalSearchCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "search", "--from", "now-1h", "--to", "now"})
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

func TestSecuritySignalSearchFlagSort(t *testing.T) {
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
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalSearchCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "search", "--sort", "timestamp"})
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

func TestSecuritySignalSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSignalsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSignalSearchCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"TIMESTAMP", "ID", "RULE_NAME", "SEVERITY", "STATUS",
		"signal-abc123", "high", "open", "AWS CloudTrail Rule",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecuritySignalSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSignalsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSignalSearchCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "search", "--json"})
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

func TestSecuritySignalSearchDefaultFrom(t *testing.T) {
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
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalSearchCmd(newTestSecurityAPI(srv))
	// no --from flag - should default to now-1h
	root.SetArgs([]string{"security", "signal", "search"})
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
