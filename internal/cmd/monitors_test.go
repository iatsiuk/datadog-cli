package cmd

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func newTestMonitorsAPI(srv *httptest.Server) func() (*monitorsAPI, error) {
	return func() (*monitorsAPI, error) {
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
		return &monitorsAPI{api: datadogV1.NewMonitorsApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestMonitorsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestMonitorsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil monitorsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV1.MonitorsApi")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestNewMonitorsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewMonitorsCommand()
	if cmd.Use != "monitors" {
		t.Errorf("Use = %q, want %q", cmd.Use, "monitors")
	}
}
