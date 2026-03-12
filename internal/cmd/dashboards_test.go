package cmd

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

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
