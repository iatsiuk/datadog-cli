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

	var mu sync.Mutex
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n == 1 {
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

const mockServicesResponse = `{
	"data": {
		"type": "services_list",
		"attributes": {
			"services": ["web-store", "checkout", "payment"]
		}
	}
}`

func newTestAPMAPI(srv *httptest.Server) func() (*apmAPI, error) {
	return func() (*apmAPI, error) {
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
		return &apmAPI{api: datadogV2.NewAPMApi(c), ctx: apiCtx}, nil
	}
}

func buildAPMServicesCmd(mkAPI func() (*apmAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	apm := &cobra.Command{Use: "apm"}
	apm.AddCommand(newAPMServicesCmd(mkAPI))
	root.AddCommand(apm)
	return root, buf
}

func TestAPMServicesEnvFlag(t *testing.T) {
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
		fmt.Fprint(w, mockServicesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMServicesCmd(newTestAPMAPI(srv))
	root.SetArgs([]string{"apm", "services", "--env", "prod"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("filter[env]"); got != "prod" {
		t.Errorf("filter[env] = %q, want %q", got, "prod")
	}
}

func TestAPMServicesTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockServicesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMServicesCmd(newTestAPMAPI(srv))
	root.SetArgs([]string{"apm", "services", "--env", "prod"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"SERVICE", "web-store", "checkout", "payment"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMServicesRequiresEnv(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMServicesCmd(newTestAPMAPI(srv))
	root.SetArgs([]string{"apm", "services"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --env is missing")
	}
}

// retention filter mock responses
const mockRetentionFiltersListResponse = `{
	"data": [
		{
			"id": "rf-1",
			"type": "apm_retention_filter",
			"attributes": {
				"name": "errors filter",
				"filter": {"query": "status:error"},
				"rate": 1.0,
				"enabled": true,
				"filter_type": "spans-errors-sampling-processor"
			}
		},
		{
			"id": "rf-2",
			"type": "apm_retention_filter",
			"attributes": {
				"name": "sample filter",
				"filter": {"query": "*"},
				"rate": 0.1,
				"enabled": false,
				"filter_type": "spans-sampling-processor"
			}
		}
	]
}`

const mockRetentionFilterGetResponse = `{
	"data": {
		"id": "rf-1",
		"type": "apm_retention_filter",
		"attributes": {
			"name": "errors filter",
			"filter": {"query": "status:error"},
			"rate": 1.0,
			"enabled": true,
			"filter_type": "spans-errors-sampling-processor"
		}
	}
}`

const mockRetentionFilterCreateResponse = `{
	"data": {
		"id": "rf-new",
		"type": "apm_retention_filter",
		"attributes": {
			"name": "new filter",
			"filter": {"query": "service:web"},
			"rate": 0.5,
			"enabled": true,
			"filter_type": "spans-sampling-processor"
		}
	}
}`

func newTestRetentionFiltersAPI(srv *httptest.Server) func() (*retentionFiltersAPI, error) {
	return func() (*retentionFiltersAPI, error) {
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
		return &retentionFiltersAPI{api: datadogV2.NewAPMRetentionFiltersApi(c), ctx: apiCtx}, nil
	}
}

func buildAPMRetentionFilterCmd(mkAPI func() (*retentionFiltersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	apm := &cobra.Command{Use: "apm"}
	apm.AddCommand(newAPMRetentionFilterCmd(mkAPI))
	root.AddCommand(apm)
	return root, buf
}

func TestAPMRetentionFilterListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRetentionFiltersListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMRetentionFilterCmd(newTestRetentionFiltersAPI(srv))
	root.SetArgs([]string{"apm", "retention-filter", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"NAME", "FILTER", "RATE", "ENABLED", "errors filter", "status:error", "sample filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMRetentionFilterShowOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRetentionFilterGetResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMRetentionFilterCmd(newTestRetentionFiltersAPI(srv))
	root.SetArgs([]string{"apm", "retention-filter", "show", "rf-1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "rf-1") {
		t.Errorf("request path %q does not contain filter id", capturedPath)
	}

	out := buf.String()
	for _, want := range []string{"errors filter", "status:error", "1", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMRetentionFilterCreateFlags(t *testing.T) {
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
		fmt.Fprint(w, mockRetentionFilterCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMRetentionFilterCmd(newTestRetentionFiltersAPI(srv))
	root.SetArgs([]string{
		"apm", "retention-filter", "create",
		"--name", "new filter",
		"--filter", "service:web",
		"--rate", "0.5",
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
		t.Fatal("missing data field")
	}
	attrs, _ := data["attributes"].(map[string]interface{})
	if attrs == nil {
		t.Fatal("missing data.attributes")
	}
	if got := attrs["name"]; got != "new filter" {
		t.Errorf("name = %v, want new filter", got)
	}
	filter, _ := attrs["filter"].(map[string]interface{})
	if filter == nil {
		t.Fatal("missing filter field")
	}
	if got := filter["query"]; got != "service:web" {
		t.Errorf("filter.query = %v, want service:web", got)
	}
	if got, _ := attrs["rate"].(float64); got != 0.5 {
		t.Errorf("rate = %v, want 0.5", got)
	}
}

func TestAPMRetentionFilterUpdateFlags(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
		reqBody     []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedReq = r
		reqBody = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRetentionFilterGetResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMRetentionFilterCmd(newTestRetentionFiltersAPI(srv))
	root.SetArgs([]string{
		"apm", "retention-filter", "update", "rf-1",
		"--name", "updated filter",
		"--filter", "status:error",
		"--rate", "1.0",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	body := reqBody
	mu.Unlock()

	if req == nil {
		t.Fatal("no request made")
	}
	if !strings.Contains(req.URL.Path, "rf-1") {
		t.Errorf("path %q does not contain rf-1", req.URL.Path)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := parsed["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "updated filter" {
		t.Errorf("name = %v, want updated filter", got)
	}
}

func TestAPMRetentionFilterDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildAPMRetentionFilterCmd(newTestRetentionFiltersAPI(srv))
	root.SetArgs([]string{"apm", "retention-filter", "delete", "rf-1"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --yes is missing")
	}
}

func TestAPMRetentionFilterDeleteWithYes(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildAPMRetentionFilterCmd(newTestRetentionFiltersAPI(srv))
	root.SetArgs([]string{"apm", "retention-filter", "delete", "rf-1", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "rf-1") {
		t.Errorf("path %q does not contain rf-1", capturedPath)
	}
	if out := buf.String(); !strings.Contains(out, "deleted") {
		t.Errorf("output missing 'deleted': %s", out)
	}
}

// span-metric mock responses
const mockSpanMetricsListResponse = `{
	"data": [
		{
			"id": "sm.count",
			"type": "spans_metrics",
			"attributes": {
				"compute": {"aggregation_type": "count"},
				"filter": {"query": "service:web"},
				"group_by": [{"path": "@http.status_code"}]
			}
		},
		{
			"id": "sm.duration",
			"type": "spans_metrics",
			"attributes": {
				"compute": {"aggregation_type": "distribution", "path": "@duration"},
				"filter": {"query": "*"}
			}
		}
	]
}`

const mockSpanMetricGetResponse = `{
	"data": {
		"id": "sm.count",
		"type": "spans_metrics",
		"attributes": {
			"compute": {"aggregation_type": "count"},
			"filter": {"query": "service:web"},
			"group_by": [{"path": "@http.status_code"}]
		}
	}
}`

const mockSpanMetricCreateResponse = `{
	"data": {
		"id": "sm.new",
		"type": "spans_metrics",
		"attributes": {
			"compute": {"aggregation_type": "count"},
			"filter": {"query": "env:prod"}
		}
	}
}`

func newTestSpansMetricsAPI(srv *httptest.Server) func() (*spansMetricsAPI, error) {
	return func() (*spansMetricsAPI, error) {
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
		return &spansMetricsAPI{api: datadogV2.NewSpansMetricsApi(c), ctx: apiCtx}, nil
	}
}

func buildAPMSpanMetricCmd(mkAPI func() (*spansMetricsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	apm := &cobra.Command{Use: "apm"}
	apm.AddCommand(newAPMSpanMetricCmd(mkAPI))
	root.AddCommand(apm)
	return root, buf
}

func TestAPMSpanMetricListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSpanMetricsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMSpanMetricCmd(newTestSpansMetricsAPI(srv))
	root.SetArgs([]string{"apm", "span-metric", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "COMPUTE", "FILTER", "GROUP-BY", "sm.count", "sm.duration", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMSpanMetricShowOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSpanMetricGetResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildAPMSpanMetricCmd(newTestSpansMetricsAPI(srv))
	root.SetArgs([]string{"apm", "span-metric", "show", "sm.count"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "sm.count") {
		t.Errorf("request path %q does not contain metric id", capturedPath)
	}

	out := buf.String()
	for _, want := range []string{"sm.count", "count", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAPMSpanMetricCreateFlags(t *testing.T) {
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
		fmt.Fprint(w, mockSpanMetricCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMSpanMetricCmd(newTestSpansMetricsAPI(srv))
	root.SetArgs([]string{
		"apm", "span-metric", "create",
		"--id", "sm.new",
		"--compute", "count",
		"--filter", "env:prod",
		"--group-by", "@http.status_code",
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
		t.Fatal("missing data field")
	}
	if got := data["id"]; got != "sm.new" {
		t.Errorf("data.id = %v, want sm.new", got)
	}
	attrs, _ := data["attributes"].(map[string]interface{})
	if attrs == nil {
		t.Fatal("missing data.attributes")
	}
	compute, _ := attrs["compute"].(map[string]interface{})
	if compute == nil {
		t.Fatal("missing compute field")
	}
	if got := compute["aggregation_type"]; got != "count" {
		t.Errorf("compute.aggregation_type = %v, want count", got)
	}
	filter, _ := attrs["filter"].(map[string]interface{})
	if filter == nil {
		t.Fatal("missing filter field")
	}
	if got := filter["query"]; got != "env:prod" {
		t.Errorf("filter.query = %v, want env:prod", got)
	}
}

func TestAPMSpanMetricUpdateFlags(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
		reqBody     []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		capturedReq = r
		reqBody = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSpanMetricGetResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildAPMSpanMetricCmd(newTestSpansMetricsAPI(srv))
	root.SetArgs([]string{
		"apm", "span-metric", "update", "sm.count",
		"--filter", "service:checkout",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	body := reqBody
	mu.Unlock()

	if req == nil {
		t.Fatal("no request made")
	}
	if !strings.Contains(req.URL.Path, "sm.count") {
		t.Errorf("path %q does not contain sm.count", req.URL.Path)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	data, _ := parsed["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	filter, _ := attrs["filter"].(map[string]interface{})
	if got := filter["query"]; got != "service:checkout" {
		t.Errorf("filter.query = %v, want service:checkout", got)
	}
}

func TestAPMSpanMetricDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildAPMSpanMetricCmd(newTestSpansMetricsAPI(srv))
	root.SetArgs([]string{"apm", "span-metric", "delete", "sm.count"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --yes is missing")
	}
}

func TestAPMSpanMetricDeleteWithYes(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildAPMSpanMetricCmd(newTestSpansMetricsAPI(srv))
	root.SetArgs([]string{"apm", "span-metric", "delete", "sm.count", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "sm.count") {
		t.Errorf("path %q does not contain sm.count", capturedPath)
	}
	if out := buf.String(); !strings.Contains(out, "deleted") {
		t.Errorf("output missing 'deleted': %s", out)
	}
}
