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

const mockTestEventsResponse = `{
	"data": [{
		"id": "test-1",
		"type": "ciTestEvent",
		"attributes": {
			"ci_level": "test",
			"attributes": {
				"@timestamp": "2024-01-15T10:30:00.000Z",
				"test.name": "TestSomething",
				"test.suite": "pkg/foo",
				"test.status": "pass",
				"duration": 500000000,
				"service": "my-service"
			},
			"tags": []
		}
	}],
	"meta": {"status": "done", "elapsed": 50, "request_id": "req-2"}
}`

func newTestTestsAPI(srv *httptest.Server) func() (*testsAPI, error) {
	return newTestTestsAPIWithCtx(srv, context.Background())
}

func newTestTestsAPIWithCtx(srv *httptest.Server, ctx context.Context) func() (*testsAPI, error) {
	return func() (*testsAPI, error) {
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
		return &testsAPI{api: datadogV2.NewCIVisibilityTestsApi(c), ctx: apiCtx}, nil
	}
}

func buildCITestSearchCmd(mkAPI func() (*testsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	ci := &cobra.Command{Use: "ci"}
	test := &cobra.Command{Use: "test"}
	test.AddCommand(newCITestSearchCmd(mkAPI))
	ci.AddCommand(test)
	root.AddCommand(ci)
	return root, buf
}

func TestCITestSearchFlagQuery(t *testing.T) {
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

	root, _ := buildCITestSearchCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "search", "--query", "test.status:pass", "--limit", "100"})
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
	if got := q.Get("filter[query]"); got != "test.status:pass" {
		t.Errorf("filter[query] = %q, want %q", got, "test.status:pass")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestCITestSearchFlagFromTo(t *testing.T) {
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

	root, _ := buildCITestSearchCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "search", "--from", "now-1h", "--to", "now"})
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

func TestCITestSearchFlagSort(t *testing.T) {
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

	root, _ := buildCITestSearchCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "search", "--sort", "-timestamp"})
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

func TestCITestSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTestEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCITestSearchCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "TEST", "SUITE", "STATUS", "DURATION", "SERVICE"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing header %q:\n%s", want, out)
		}
	}
	for _, want := range []string{"TestSomething", "pkg/foo", "pass", "my-service"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing value %q:\n%s", want, out)
		}
	}
}

func TestCITestSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTestEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCITestSearchCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "search", "--json"})
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

func TestCITestSearchDefaultFrom(t *testing.T) {
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

	root, _ := buildCITestSearchCmd(newTestTestsAPI(srv))
	// no --from flag - should default to now-1h
	root.SetArgs([]string{"ci", "test", "search"})
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

func buildCITestTailCmd(mkAPI func() (*testsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	ci := &cobra.Command{Use: "ci"}
	test := &cobra.Command{Use: "test"}
	test.AddCommand(newCITestTailCmd(mkAPI))
	ci.AddCommand(test)
	root.AddCommand(ci)
	return root, buf
}

func TestCITestTailFlagQuery(t *testing.T) {
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
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCITestTailCmd(newTestTestsAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"ci", "test", "tail", "--query", "test.status:pass"})
	_ = root.Execute()

	mu.Lock()
	reqs := capturedReqs
	mu.Unlock()
	if len(reqs) == 0 {
		t.Fatal("no requests made to mock server")
	}
	if got := reqs[0].URL.Query().Get("filter[query]"); got != "test.status:pass" {
		t.Errorf("filter[query] = %q, want %q", got, "test.status:pass")
	}
}

func TestCITestTailPollsAPIAndPrintsNewEvents(t *testing.T) {
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
			fmt.Fprint(w, mockTestEventsResponse) //nolint:errcheck
		} else {
			cancel()
			fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildCITestTailCmd(newTestTestsAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"ci", "test", "tail"})
	_ = root.Execute()

	out := buf.String()
	for _, want := range []string{"TestSomething", "pkg/foo", "pass", "my-service"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
