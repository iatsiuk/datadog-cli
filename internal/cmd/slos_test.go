package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

func newTestSLOsAPI(srv *httptest.Server) func() (*slosAPI, error) {
	return func() (*slosAPI, error) {
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
		return &slosAPI{
			api:         datadogV1.NewServiceLevelObjectivesApi(c),
			corrections: datadogV1.NewServiceLevelObjectiveCorrectionsApi(c),
			ctx:         apiCtx,
		}, nil
	}
}

func TestNewTestSLOsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestSLOsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil slosAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil ServiceLevelObjectivesApi")
	}
	if api.corrections == nil {
		t.Fatal("expected non-nil ServiceLevelObjectiveCorrectionsApi")
	}
}

func buildSLOsListCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	slos := &cobra.Command{Use: "slos"}
	slos.AddCommand(newSLOsListCmd(mkAPI))
	root.AddCommand(slos)
	return root, buf
}

const mockSLOsListResponse = `{
	"data": [
		{
			"id": "abc123",
			"name": "API Availability",
			"type": "metric",
			"thresholds": [{"timeframe": "30d", "target": 99.9}],
			"tags": ["env:prod", "service:api"]
		},
		{
			"id": "def456",
			"name": "Login Success",
			"type": "monitor",
			"thresholds": [{"timeframe": "7d", "target": 99.0}],
			"tags": ["env:prod"]
		}
	]
}`

func TestSLOsList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "TYPE", "TARGET", "TIMEFRAME", "abc123", "API Availability", "metric", "99.9", "30d", "def456", "Login Success", "monitor", "99", "7d"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"id"`) {
		t.Errorf("JSON output missing id field\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "API Availability") {
		t.Errorf("JSON output missing SLO name\nfull output:\n%s", out)
	}
}

func TestSLOsList_WithQueryFilter(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list", "--query", "env:prod"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotQuery != "env:prod" {
		t.Errorf("query param = %q, want %q", gotQuery, "env:prod")
	}
}

func TestSLOsList_WithTagsFilter(t *testing.T) {
	t.Parallel()
	var gotTags string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTags = r.URL.Query().Get("tags_query")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list", "--tags", "service:web"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotTags != "service:web" {
		t.Errorf("tags_query param = %q, want %q", gotTags, "service:web")
	}
}

func TestSLOsList_Empty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("expected headers in empty output, got:\n%s", out)
	}
}

func TestNewSLOsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewSLOsCommand()
	if cmd.Use != "slos" {
		t.Errorf("Use = %q, want %q", cmd.Use, "slos")
	}

	want := []string{"list", "show", "history", "create", "update", "delete", "can-delete", "correction"}
	found := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, name := range want {
		if !found[name] {
			t.Errorf("subcommand %q not found", name)
		}
	}
}
