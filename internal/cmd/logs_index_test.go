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

func newTestLogsIndexAPI(srv *httptest.Server) func() (*logsIndexAPI, error) {
	return func() (*logsIndexAPI, error) {
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
		return &logsIndexAPI{api: datadogV1.NewLogsIndexesApi(c), ctx: ctx}, nil
	}
}

func buildIndexCmd(mkAPI func() (*logsIndexAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(newLogsIndexCmd(mkAPI))
	return root, buf
}

const mockIndexListResponse = `{
	"indexes": [
		{
			"name": "main",
			"filter": {"query": "service:web"},
			"num_retention_days": 15
		},
		{
			"name": "archive",
			"filter": {"query": "*"},
			"num_retention_days": 30
		}
	]
}`

const mockIndexResponse = `{
	"name": "main",
	"filter": {"query": "service:web"},
	"num_retention_days": 15
}`

func TestLogsIndexListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"NAME", "FILTER", "RETENTION", "main", "service:web", "15", "archive", "30"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsIndexListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "list", "--json"})
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

func TestLogsIndexShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "show", "main"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"NAME", "FILTER", "RETENTION", "main", "service:web", "15"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsIndexShowRequiresName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestLogsIndexCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(capturedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "create", "--name", "main", "--filter", "service:web", "--retention", "15"})
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
	if got := req["name"]; got != "main" {
		t.Errorf("name = %v, want main", got)
	}
	filter, ok := req["filter"].(map[string]interface{})
	if !ok {
		t.Fatal("missing filter in request")
	}
	if got := filter["query"]; got != "service:web" {
		t.Errorf("filter.query = %v, want service:web", got)
	}
	if got := req["num_retention_days"]; got == nil {
		t.Error("num_retention_days should be set")
	}
}

func TestLogsIndexCreateRequiresName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "create", "--filter", "service:web"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --name flag")
	}
}

func TestLogsIndexUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(capturedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIndexResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "update", "main", "--filter", "service:api", "--retention", "30"})
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
	if got := req["num_retention_days"]; got == nil {
		t.Error("num_retention_days should be set")
	}
}

func TestLogsIndexDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "delete", "main"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestLogsIndexDeleteWithYes(t *testing.T) {
	t.Parallel()

	var deletedName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// extract index name from path: /api/v1/logs/config/indexes/{name}
		parts := strings.Split(r.URL.Path, "/")
		deletedName = parts[len(parts)-1]
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIndexCmd(newTestLogsIndexAPI(srv))
	root.SetArgs([]string{"index", "delete", "main", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if deletedName != "main" {
		t.Errorf("deleted index = %q, want %q", deletedName, "main")
	}
	if !strings.Contains(buf.String(), "main") {
		t.Errorf("output should mention deleted index name:\n%s", buf.String())
	}
}
