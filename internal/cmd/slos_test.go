package cmd

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
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
