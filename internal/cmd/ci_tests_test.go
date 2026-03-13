package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		mu              sync.Mutex
		capturedQueries []string
	)
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedQueries = append(capturedQueries, r.URL.RawQuery)
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
	queries := capturedQueries
	mu.Unlock()
	if len(queries) == 0 {
		t.Fatal("no requests made to mock server")
	}
	parsed, _ := url.ParseQuery(queries[0])
	if got := parsed.Get("filter[query]"); got != "test.status:pass" {
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

const mockTestAggregateResponse = `{
	"data": {
		"buckets": [{
			"by": {"test.status": "pass"},
			"computes": {"c0": 99.0}
		}]
	}
}`

func buildCITestAggregateCmd(mkAPI func() (*testsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	ci := &cobra.Command{Use: "ci"}
	test := &cobra.Command{Use: "test"}
	test.AddCommand(newCITestAggregateCmd(mkAPI))
	ci.AddCommand(test)
	root.AddCommand(ci)
	return root, buf
}

func TestCITestAggregateFlags(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		reqBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqBody = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"buckets":[]}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCITestAggregateCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{
		"ci", "test", "aggregate",
		"--query", "test.status:pass",
		"--from", "now-1h",
		"--to", "now",
		"--group-by", "test.status",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := reqBody
	mu.Unlock()

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("invalid request body: %v\n%s", err, body)
	}

	filter, _ := parsed["filter"].(map[string]interface{})
	if filter == nil {
		t.Fatal("request missing filter field")
	}
	if got := filter["query"]; got != "test.status:pass" {
		t.Errorf("filter.query = %v, want test.status:pass", got)
	}
	if got := filter["from"]; got != "now-1h" {
		t.Errorf("filter.from = %v, want now-1h", got)
	}

	groups, _ := parsed["group_by"].([]interface{})
	if len(groups) != 1 {
		t.Fatalf("group_by len = %d, want 1", len(groups))
	}
	g, _ := groups[0].(map[string]interface{})
	if got := g["facet"]; got != "test.status" {
		t.Errorf("group_by[0].facet = %v, want test.status", got)
	}

	computes, _ := parsed["compute"].([]interface{})
	if len(computes) != 1 {
		t.Fatalf("compute len = %d, want 1", len(computes))
	}
	c, _ := computes[0].(map[string]interface{})
	if got := c["aggregation"]; got != "count" {
		t.Errorf("compute[0].aggregation = %v, want count", got)
	}
}

func TestCITestAggregateTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTestAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCITestAggregateCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{
		"ci", "test", "aggregate",
		"--group-by", "test.status",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"test.status", "pass", "99"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCITestAggregateJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTestAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCITestAggregateCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{
		"ci", "test", "aggregate",
		"--compute", "count",
		"--json",
	})
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

func TestCITestAggregateRequiresCompute(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCITestAggregateCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "aggregate"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --compute is missing")
	}
}

func TestCITestSearchInvalidSort(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCITestSearchCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "search", "--sort", "invalid"})
	if err := root.Execute(); err == nil {
		t.Error("expected error for invalid --sort value")
	}
}

func TestCITestAggregateInvalidCompute(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"buckets":[]}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCITestAggregateCmd(newTestTestsAPI(srv))
	root.SetArgs([]string{"ci", "test", "aggregate", "--compute", "notafunc"})
	if err := root.Execute(); err == nil {
		t.Error("expected error for invalid --compute aggregation function")
	}
}
