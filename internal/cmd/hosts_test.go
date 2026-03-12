package cmd

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
)

func newTestHostsAPI(srv *httptest.Server) func() (*hostsAPI, error) {
	return func() (*hostsAPI, error) {
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
		return &hostsAPI{api: datadogV1.NewHostsApi(c), ctx: apiCtx}, nil
	}
}

func newTestTagsAPI(srv *httptest.Server) func() (*tagsAPI, error) {
	return func() (*tagsAPI, error) {
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
		return &tagsAPI{api: datadogV1.NewTagsApi(c), ctx: apiCtx}, nil
	}
}

func TestNewHostsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewHostsCommand()
	if cmd.Use != "hosts" {
		t.Errorf("Use = %q, want %q", cmd.Use, "hosts")
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	if !subNames["tags"] {
		t.Error("expected 'tags' subcommand")
	}
}

func TestNewTestHostsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestHostsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil hostsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil api.api")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil api.ctx")
	}
}

func TestNewTestTagsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestTagsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil tagsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil api.api")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil api.ctx")
	}
}
