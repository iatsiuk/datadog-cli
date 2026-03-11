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

const mockSpansResponse = `{
	"data": [{
		"id": "span-1",
		"type": "spans",
		"attributes": {
			"service": "web-store",
			"resource_name": "/api/users",
			"start_timestamp": "2024-01-15T10:30:00.000Z",
			"end_timestamp": "2024-01-15T10:30:00.100Z"
		}
	}],
	"meta": {"status": "done", "elapsed": 100, "request_id": "req-1"}
}`

func newTestSpansAPI(srv *httptest.Server) func() (*spansAPI, error) {
	return newTestSpansAPIWithCtx(srv, context.Background())
}

func newTestSpansAPIWithCtx(srv *httptest.Server, ctx context.Context) func() (*spansAPI, error) {
	return func() (*spansAPI, error) {
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
		return &spansAPI{api: datadogV2.NewSpansApi(c), ctx: apiCtx}, nil
	}
}

func buildAPMSearchCmd(mkAPI func() (*spansAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	apm := &cobra.Command{Use: "apm"}
	search := newAPMSearchCmd(mkAPI)
	apm.AddCommand(search)
	root.AddCommand(apm)
	return root, buf
}

func TestAPMSearchFlagQuery(t *testing.T) {
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

	root, _ := buildAPMSearchCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{"apm", "search", "--query", "service:web-store", "--limit", "100"})
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
	if got := q.Get("filter[query]"); got != "service:web-store" {
		t.Errorf("filter[query] = %q, want %q", got, "service:web-store")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestAPMSearchFlagFromTo(t *testing.T) {
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

	root, _ := buildAPMSearchCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{"apm", "search", "--from", "now-1h", "--to", "now"})
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

func TestAPMSearchFlagSort(t *testing.T) {
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

	root, _ := buildAPMSearchCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{"apm", "search", "--sort", "-timestamp"})
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

func TestAPMSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSpansResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMSearchCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{"apm", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "SERVICE", "RESOURCE", "DURATION", "STATUS",
		"web-store", "/api/users"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSpansResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMSearchCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{"apm", "search", "--json"})
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

func TestAPMSearchDefaultFrom(t *testing.T) {
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

	root, _ := buildAPMSearchCmd(newTestSpansAPI(srv))
	// no --from flag - should default to now-15m
	root.SetArgs([]string{"apm", "search"})
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

func buildAPMTailCmd(mkAPI func() (*spansAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	apm := &cobra.Command{Use: "apm"}
	tail := newAPMTailCmd(mkAPI)
	apm.AddCommand(tail)
	root.AddCommand(apm)
	return root, buf
}

func TestAPMTailFlagQuery(t *testing.T) {
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

	root, _ := buildAPMTailCmd(newTestSpansAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"apm", "tail", "--query", "service:web-store"})
	// tail exits when context is cancelled; ignore the context error
	_ = root.Execute()

	mu.Lock()
	reqs := capturedReqs
	mu.Unlock()
	if len(reqs) == 0 {
		t.Fatal("no requests made to mock server")
	}
	if got := reqs[0].URL.Query().Get("filter[query]"); got != "service:web-store" {
		t.Errorf("filter[query] = %q, want %q", got, "service:web-store")
	}
}

func TestAPMTailServiceFlag(t *testing.T) {
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

	root, _ := buildAPMTailCmd(newTestSpansAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"apm", "tail", "--service", "checkout"})
	_ = root.Execute()

	mu.Lock()
	reqs := capturedReqs
	mu.Unlock()
	if len(reqs) == 0 {
		t.Fatal("no requests made")
	}
	// --service checkout should produce a filter query containing service:checkout
	if got := reqs[0].URL.Query().Get("filter[query]"); !strings.Contains(got, "service:checkout") {
		t.Errorf("filter[query] = %q, want it to contain service:checkout", got)
	}
}

func TestAPMTailPollsAPIAndPrintsNewSpans(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			fmt.Fprint(w, mockSpansResponse) //nolint:errcheck
		} else {
			cancel()
			fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildAPMTailCmd(newTestSpansAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"apm", "tail"})
	_ = root.Execute()

	out := buf.String()
	for _, want := range []string{"web-store", "/api/users"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

const mockSpansAggregateResponse = `{
	"data": [{
		"type": "bucket",
		"attributes": {
			"by": {"@http.status_code": "200"},
			"computes": {"c0": 42.0}
		}
	}],
	"meta": {"status": "done", "elapsed": 10, "request_id": "req-2"}
}`

func buildAPMAggregateCmd(mkAPI func() (*spansAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	apm := &cobra.Command{Use: "apm"}
	apm.AddCommand(newAPMAggregateCmd(mkAPI))
	root.AddCommand(apm)
	return root, buf
}

func TestAPMAggregateFlags(t *testing.T) {
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
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMAggregateCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{
		"apm", "aggregate",
		"--query", "service:web",
		"--from", "now-1h",
		"--to", "now",
		"--group-by", "@http.status_code",
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

	data, _ := parsed["data"].(map[string]interface{})
	if data == nil {
		t.Fatal("request missing data field")
	}
	attrs, _ := data["attributes"].(map[string]interface{})
	if attrs == nil {
		t.Fatal("request missing data.attributes field")
	}

	filter, _ := attrs["filter"].(map[string]interface{})
	if filter == nil {
		t.Fatal("request missing filter")
	}
	if got := filter["query"]; got != "service:web" {
		t.Errorf("filter.query = %v, want service:web", got)
	}
	if got := filter["from"]; got != "now-1h" {
		t.Errorf("filter.from = %v, want now-1h", got)
	}

	groups, _ := attrs["group_by"].([]interface{})
	if len(groups) != 1 {
		t.Fatalf("group_by len = %d, want 1", len(groups))
	}
	g, _ := groups[0].(map[string]interface{})
	if got := g["facet"]; got != "@http.status_code" {
		t.Errorf("group_by[0].facet = %v, want @http.status_code", got)
	}

	computes, _ := attrs["compute"].([]interface{})
	if len(computes) != 1 {
		t.Fatalf("compute len = %d, want 1", len(computes))
	}
	c, _ := computes[0].(map[string]interface{})
	if got := c["aggregation"]; got != "count" {
		t.Errorf("compute[0].aggregation = %v, want count", got)
	}
}

func TestAPMAggregateTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSpansAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMAggregateCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{
		"apm", "aggregate",
		"--group-by", "@http.status_code",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"@http.status_code", "200", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMAggregateRequiresCompute(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMAggregateCmd(newTestSpansAPI(srv))
	root.SetArgs([]string{"apm", "aggregate"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --compute is missing")
	}
}
