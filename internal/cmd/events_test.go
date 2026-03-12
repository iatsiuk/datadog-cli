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
	"time"

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
		cfg.OperationServers["v2.EventsApi.CreateEvent"] = datadog.ServerConfigurations{{URL: srv.URL}}
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

func buildEventsCreateCmd(mkAPI func() (*eventsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	events := &cobra.Command{Use: "events"}
	events.AddCommand(newEventsCreateCmd(mkAPI))
	root.AddCommand(events)
	return root, buf
}

const mockEventCreateResponse = `{
	"data": {
		"attributes": {
			"attributes": {
				"evt": {
					"id": "created-event-123"
				}
			}
		},
		"type": "event"
	}
}`

func TestEventsCreateFlags(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedReq  *http.Request
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildEventsCreateCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{
		"events", "create",
		"--title", "Deploy v2.1",
		"--text", "Released to prod",
		"--tags", "env:prod,service:web",
		"--alert-type", "info",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	body := capturedBody
	mu.Unlock()

	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	data := payload["data"].(map[string]interface{})
	attrs := data["attributes"].(map[string]interface{})
	if attrs["title"] != "Deploy v2.1" {
		t.Errorf("title = %v, want Deploy v2.1", attrs["title"])
	}
	if attrs["message"] != "Released to prod" {
		t.Errorf("message = %v, want Released to prod", attrs["message"])
	}

	out := buf.String()
	if !strings.Contains(out, "created-event-123") {
		t.Errorf("output missing event id:\n%s", out)
	}
}

func TestEventsCreateTitleRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildEventsCreateCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --title is missing")
	}
}

func TestEventsCreateAlertTypeValidation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockEventCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	for _, validType := range []string{"info", "warning", "error", "success"} {
		root, _ := buildEventsCreateCmd(newTestEventsAPI(srv))
		root.SetArgs([]string{"events", "create", "--title", "test", "--alert-type", validType})
		if err := root.Execute(); err != nil {
			t.Errorf("alert-type %q should be valid, got error: %v", validType, err)
		}
	}

	root, _ := buildEventsCreateCmd(newTestEventsAPI(srv))
	root.SetArgs([]string{"events", "create", "--title", "test", "--alert-type", "invalid"})
	if err := root.Execute(); err == nil {
		t.Error("expected error for invalid --alert-type value")
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

func buildEventsTailCmd(mkAPI func() (*eventsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	events := &cobra.Command{Use: "events"}
	events.AddCommand(newEventsTailCmd(mkAPI))
	root.AddCommand(events)
	return root, buf
}

func TestEventsTailFlagQuery(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		reqCount    int
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqCount++
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mkAPI := func() (*eventsAPI, error) {
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
		return &eventsAPI{api: datadogV2.NewEventsApi(c), ctx: apiCtx}, nil
	}

	root, _ := buildEventsTailCmd(mkAPI)
	root.SetArgs([]string{"events", "tail", "--query", "source:github", "--interval", "50ms"})

	done := make(chan error, 1)
	go func() {
		done <- root.Execute()
	}()

	// wait for at least one poll then cancel
	for i := 0; i < 100; i++ {
		mu.Lock()
		count := reqCount
		mu.Unlock()
		if count >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	mu.Lock()
	req := capturedReq
	mu.Unlock()

	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("filter[query]"); got != "source:github" {
		t.Errorf("filter[query] = %q, want source:github", got)
	}
}

func TestEventsTailPrintsNewEvents(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		reqCount int
	)
	// first poll returns one event, subsequent polls return empty
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		count := reqCount
		reqCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if count == 0 {
			fmt.Fprint(w, mockEventsResponse) //nolint:errcheck
		} else {
			fmt.Fprint(w, `{"data":[],"meta":{"status":"done"}}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mkAPI := func() (*eventsAPI, error) {
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
		return &eventsAPI{api: datadogV2.NewEventsApi(c), ctx: apiCtx}, nil
	}

	root, buf := buildEventsTailCmd(mkAPI)
	root.SetArgs([]string{"events", "tail", "--interval", "50ms"})

	done := make(chan error, 1)
	go func() {
		done <- root.Execute()
	}()

	// wait for at least 2 polls to ensure first event was printed
	for i := 0; i < 100; i++ {
		mu.Lock()
		count := reqCount
		mu.Unlock()
		if count >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	out := buf.String()
	if !strings.Contains(out, "Deploy v2.1") {
		t.Errorf("output missing event title:\n%s", out)
	}
}
