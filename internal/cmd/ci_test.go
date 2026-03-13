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

const mockPipelineEventsResponse = `{
	"data": [{
		"id": "pipe-1",
		"type": "ciPipelineEvent",
		"attributes": {
			"ci_level": "pipeline",
			"attributes": {
				"@timestamp": "2024-01-15T10:30:00.000Z",
				"ci.pipeline.name": "build-and-test",
				"ci.status": "success",
				"duration": 120000000000,
				"git.branch": "main"
			},
			"tags": []
		}
	}],
	"meta": {"status": "done", "elapsed": 100, "request_id": "req-1"}
}`

func newTestPipelinesAPI(srv *httptest.Server) func() (*pipelinesAPI, error) {
	return newTestPipelinesAPIWithCtx(srv, context.Background())
}

func newTestPipelinesAPIWithCtx(srv *httptest.Server, ctx context.Context) func() (*pipelinesAPI, error) {
	return func() (*pipelinesAPI, error) {
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
		return &pipelinesAPI{api: datadogV2.NewCIVisibilityPipelinesApi(c), ctx: apiCtx}, nil
	}
}

func buildCIPipelineSearchCmd(mkAPI func() (*pipelinesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	ci := &cobra.Command{Use: "ci"}
	pipeline := &cobra.Command{Use: "pipeline"}
	pipeline.AddCommand(newCIPipelineSearchCmd(mkAPI))
	ci.AddCommand(pipeline)
	root.AddCommand(ci)
	return root, buf
}

func TestCIPipelineSearchFlagQuery(t *testing.T) {
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

	root, _ := buildCIPipelineSearchCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{"ci", "pipeline", "search", "--query", "ci.status:success", "--limit", "100"})
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
	if got := q.Get("filter[query]"); got != "ci.status:success" {
		t.Errorf("filter[query] = %q, want %q", got, "ci.status:success")
	}
	if got := q.Get("page[limit]"); got != "100" {
		t.Errorf("page[limit] = %q, want %q", got, "100")
	}
}

func TestCIPipelineSearchFlagFromTo(t *testing.T) {
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

	root, _ := buildCIPipelineSearchCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{"ci", "pipeline", "search", "--from", "now-1h", "--to", "now"})
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

func TestCIPipelineSearchFlagSort(t *testing.T) {
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

	root, _ := buildCIPipelineSearchCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{"ci", "pipeline", "search", "--sort", "-timestamp"})
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

func TestCIPipelineSearchTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCIPipelineSearchCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{"ci", "pipeline", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "PIPELINE", "STATUS", "DURATION", "BRANCH"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing header %q:\n%s", want, out)
		}
	}
}

func TestCIPipelineSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineEventsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCIPipelineSearchCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{"ci", "pipeline", "search", "--json"})
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

func buildCIPipelineTailCmd(mkAPI func() (*pipelinesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	ci := &cobra.Command{Use: "ci"}
	pipeline := &cobra.Command{Use: "pipeline"}
	pipeline.AddCommand(newCIPipelineTailCmd(mkAPI))
	ci.AddCommand(pipeline)
	root.AddCommand(ci)
	return root, buf
}

func TestCIPipelineTailFlagQuery(t *testing.T) {
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

	root, _ := buildCIPipelineTailCmd(newTestPipelinesAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"ci", "pipeline", "tail", "--query", "ci.status:success"})
	_ = root.Execute()

	mu.Lock()
	reqs := capturedReqs
	mu.Unlock()
	if len(reqs) == 0 {
		t.Fatal("no requests made to mock server")
	}
	if got := reqs[0].URL.Query().Get("filter[query]"); got != "ci.status:success" {
		t.Errorf("filter[query] = %q, want %q", got, "ci.status:success")
	}
}

func TestCIPipelineTailPollsAPIAndPrintsNewEvents(t *testing.T) {
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
			fmt.Fprint(w, mockPipelineEventsResponse) //nolint:errcheck
		} else {
			cancel()
			fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildCIPipelineTailCmd(newTestPipelinesAPIWithCtx(srv, ctx))
	root.SetArgs([]string{"ci", "pipeline", "tail"})
	_ = root.Execute()

	out := buf.String()
	for _, want := range []string{"build-and-test", "success", "main"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCIPipelineSearchDefaultFrom(t *testing.T) {
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

	root, _ := buildCIPipelineSearchCmd(newTestPipelinesAPI(srv))
	// no --from flag - should default to now-1h
	root.SetArgs([]string{"ci", "pipeline", "search"})
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

const mockPipelineAggregateResponse = `{
	"data": {
		"buckets": [{
			"by": {"ci.status": "success"},
			"computes": {"c0": 42.0}
		}]
	}
}`

func buildCIPipelineAggregateCmd(mkAPI func() (*pipelinesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	ci := &cobra.Command{Use: "ci"}
	pipeline := &cobra.Command{Use: "pipeline"}
	pipeline.AddCommand(newCIPipelineAggregateCmd(mkAPI))
	ci.AddCommand(pipeline)
	root.AddCommand(ci)
	return root, buf
}

func TestCIPipelineAggregateFlags(t *testing.T) {
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

	root, _ := buildCIPipelineAggregateCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{
		"ci", "pipeline", "aggregate",
		"--query", "ci.status:success",
		"--from", "now-1h",
		"--to", "now",
		"--group-by", "ci.status",
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
	if got := filter["query"]; got != "ci.status:success" {
		t.Errorf("filter.query = %v, want ci.status:success", got)
	}
	if got := filter["from"]; got != "now-1h" {
		t.Errorf("filter.from = %v, want now-1h", got)
	}

	groups, _ := parsed["group_by"].([]interface{})
	if len(groups) != 1 {
		t.Fatalf("group_by len = %d, want 1", len(groups))
	}
	g, _ := groups[0].(map[string]interface{})
	if got := g["facet"]; got != "ci.status" {
		t.Errorf("group_by[0].facet = %v, want ci.status", got)
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

func TestCIPipelineAggregateTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCIPipelineAggregateCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{
		"ci", "pipeline", "aggregate",
		"--group-by", "ci.status",
		"--compute", "count",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ci.status", "success", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCIPipelineAggregateJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineAggregateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCIPipelineAggregateCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{
		"ci", "pipeline", "aggregate",
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

func TestCIPipelineAggregateRequiresCompute(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCIPipelineAggregateCmd(newTestPipelinesAPI(srv))
	root.SetArgs([]string{"ci", "pipeline", "aggregate"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --compute is missing")
	}
}
