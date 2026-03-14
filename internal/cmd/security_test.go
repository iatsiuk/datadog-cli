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
		cfg.SetUnstableOperationEnabled("v2.ListFindings", true)
		cfg.SetUnstableOperationEnabled("v2.GetFinding", true)
		cfg.SetUnstableOperationEnabled("v2.MuteFindings", true)
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
		"TIMESTAMP", "ID", "RULE_NAME", "SEVERITY", "STATUS", "SOURCE",
		"signal-abc123", "high", "open", "AWS CloudTrail Rule", "cloudtrail",
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

const mockSignalShowResponse = `{
	"data": {
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
	}
}`

func buildSignalTailCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sig := &cobra.Command{Use: "signal"}
	sig.AddCommand(newSecuritySignalTailCmd(mkAPI))
	sec.AddCommand(sig)
	root.AddCommand(sec)
	return root, buf
}

func buildSignalShowCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sig := &cobra.Command{Use: "signal"}
	sig.AddCommand(newSecuritySignalShowCmd(mkAPI))
	sec.AddCommand(sig)
	root.AddCommand(sec)
	return root, buf
}

func TestSecuritySignalTailFlagQuery(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedReqs []*http.Request
	)
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReqs = append(capturedReqs, r)
		callCount++
		if callCount >= 2 {
			cancel()
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalTailCmd(newTestSecurityAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"security", "signal", "tail", "--query", "source:cloudtrail"})
	_ = root.Execute()

	mu.Lock()
	reqs := capturedReqs
	mu.Unlock()
	if len(reqs) == 0 {
		t.Fatal("no requests made to mock server")
	}
	if got := reqs[0].URL.Query().Get("filter[query]"); got != "source:cloudtrail" {
		t.Errorf("filter[query] = %q, want %q", got, "source:cloudtrail")
	}
}

func TestSecuritySignalTailPollsAPIAndPrintsNewSignals(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n == 1 {
			fmt.Fprint(w, mockSignalsResponse) //nolint:errcheck
		} else {
			cancel()
			fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildSignalTailCmd(newTestSecurityAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"security", "signal", "tail"})
	_ = root.Execute()

	mu.Lock()
	calls := callCount
	mu.Unlock()
	if calls < 2 {
		t.Errorf("expected at least 2 API calls for polling, got %d", calls)
	}

	out := buf.String()
	for _, want := range []string{"signal-abc123", "high", "open", "AWS CloudTrail Rule"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecuritySignalShowDetail(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/signal-abc123") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSignalShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSignalShowCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "show", "signal-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"signal-abc123", "AWS CloudTrail Rule", "high", "open",
		"Unauthorized access detected", "2024-01-15",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecuritySignalShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSignalShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSignalShowCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "show", "signal-abc123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["id"] != "signal-abc123" {
		t.Errorf("id = %v, want signal-abc123", result["id"])
	}
}

func buildSignalTriageCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sig := &cobra.Command{Use: "signal"}
	sig.AddCommand(newSecuritySignalSetStateCmd(mkAPI))
	sig.AddCommand(newSecuritySignalAssignCmd(mkAPI))
	sig.AddCommand(newSecuritySignalAddIncidentCmd(mkAPI))
	sec.AddCommand(sig)
	root.AddCommand(sec)
	return root, buf
}

const mockTriageUpdateResponse = `{"data":{}}`

func TestSecuritySignalSetState(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTriageUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalTriageCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "set-state", "signal-abc123", "--state", "archived"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	mu.Lock()
	body := capturedBody
	mu.Unlock()
	if !strings.Contains(string(body), `"archived"`) {
		t.Errorf("request body missing state value: %s", body)
	}
}

func TestSecuritySignalSetStateInvalidState(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTriageUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalTriageCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "set-state", "signal-abc123", "--state", "invalid"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestSecuritySignalAssign(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTriageUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalTriageCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "assign", "signal-abc123", "--assignee", "user@example.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	mu.Lock()
	body := capturedBody
	mu.Unlock()
	if !strings.Contains(string(body), "user@example.com") {
		t.Errorf("request body missing assignee handle: %s", body)
	}
}

func TestSecuritySignalAddIncident(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTriageUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSignalTriageCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "signal", "add-incident", "signal-abc123", "--incident-id", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	mu.Lock()
	body := capturedBody
	mu.Unlock()
	if !strings.Contains(string(body), "12345") {
		t.Errorf("request body missing incident id: %s", body)
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
