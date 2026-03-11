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
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

func newTestLogsPipelineAPI(srv *httptest.Server) func() (*logsPipelineAPI, error) {
	return func() (*logsPipelineAPI, error) {
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
		return &logsPipelineAPI{api: datadogV1.NewLogsPipelinesApi(c), ctx: ctx}, nil
	}
}

func buildPipelineCmd(mkAPI func() (*logsPipelineAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(newLogsPipelineCmd(mkAPI))
	return root, buf
}

const mockPipelineListResponse = `[
	{
		"id": "abc123",
		"name": "web-pipeline",
		"is_enabled": true,
		"filter": {"query": "service:web"}
	},
	{
		"id": "def456",
		"name": "api-pipeline",
		"is_enabled": false,
		"filter": {"query": "service:api"}
	}
]`

const mockPipelineResponse = `{
	"id": "abc123",
	"name": "web-pipeline",
	"is_enabled": true,
	"filter": {"query": "service:web"}
}`

func TestLogsPipelineListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "ENABLED", "FILTER", "abc123", "web-pipeline", "true", "service:web", "def456", "api-pipeline", "false"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsPipelineListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 2 {
		t.Errorf("got %d entries, want 2", len(result))
	}
}

func TestLogsPipelineShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "show", "abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "ENABLED", "FILTER", "abc123", "web-pipeline", "true", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsPipelineShowRequiresID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestLogsPipelineCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(capturedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "create", "--name", "web-pipeline", "--filter", "service:web", "--enabled"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	if got := req["name"]; got != "web-pipeline" {
		t.Errorf("name = %v, want web-pipeline", got)
	}
	filter, ok := req["filter"].(map[string]interface{})
	if !ok {
		t.Fatal("missing filter in request")
	}
	if got := filter["query"]; got != "service:web" {
		t.Errorf("filter.query = %v, want service:web", got)
	}
}

func TestLogsPipelineCreateRequiresName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "create", "--filter", "service:web"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --name flag")
	}
}

func TestLogsPipelineUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(capturedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPipelineResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "update", "abc123", "--name", "web-pipeline", "--filter", "service:api"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	filter, ok := req["filter"].(map[string]interface{})
	if !ok {
		t.Fatal("missing filter in request")
	}
	if got := filter["query"]; got != "service:api" {
		t.Errorf("filter.query = %v, want service:api", got)
	}
}

func TestLogsPipelineDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "delete", "abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestLogsPipelineDeleteWithYes(t *testing.T) {
	t.Parallel()

	var deletedID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		deletedID = parts[len(parts)-1]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, buf := buildPipelineCmd(newTestLogsPipelineAPI(srv))
	root.SetArgs([]string{"pipeline", "delete", "abc123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if deletedID != "abc123" {
		t.Errorf("deleted pipeline id = %q, want %q", deletedID, "abc123")
	}
	if !strings.Contains(buf.String(), "abc123") {
		t.Errorf("output should mention deleted pipeline id:\n%s", buf.String())
	}
}
