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

const mockDashboardsListResponse = `{
	"dashboards": [
		{
			"id": "abc-123",
			"title": "My Dashboard",
			"layout_type": "ordered",
			"url": "/dashboard/abc-123/my-dashboard",
			"created_at": "2024-01-15T10:00:00.000Z",
			"modified_at": "2024-01-20T15:30:00.000Z"
		}
	]
}`

func newTestDashboardsAPI(srv *httptest.Server) func() (*dashboardsAPI, error) {
	return func() (*dashboardsAPI, error) {
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
		return &dashboardsAPI{api: datadogV1.NewDashboardsApi(c), ctx: ctx}, nil
	}
}

func buildDashboardsListCmd(mkAPI func() (*dashboardsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	dashboards.AddCommand(newDashboardsListCmd(mkAPI))
	root.AddCommand(dashboards)
	return root, buf
}

func TestNewDashboardsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewDashboardsCommand()
	if cmd.Use != "dashboards" {
		t.Errorf("Use = %q, want %q", cmd.Use, "dashboards")
	}
}

func TestNewTestDashboardsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestDashboardsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil api")
	}
	if api.api == nil {
		t.Fatal("expected non-nil api.api")
	}
}

func TestDashboardsListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsListCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "TITLE", "LAYOUT", "URL", "CREATED", "MODIFIED", "abc-123", "My Dashboard", "ordered", "/dashboard/abc-123"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDashboardsListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsListCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "list", "--json"})
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

func TestDashboardsListEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"dashboards":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsListCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	// table headers should still appear
	if !strings.Contains(out, "ID") {
		t.Errorf("output missing headers:\n%s", out)
	}
}
