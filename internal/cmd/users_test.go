package cmd

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func newTestUsersAPI(srv *httptest.Server) func() (*usersAPI, error) {
	return func() (*usersAPI, error) {
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
		return &usersAPI{api: datadogV2.NewUsersApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestUsersAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestUsersAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil usersAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV2.UsersApi")
	}
}

func TestNewUsersCommand_Use(t *testing.T) {
	t.Parallel()
	cmd := NewUsersCommand()
	if cmd.Use != "users" {
		t.Errorf("Use = %q, want %q", cmd.Use, "users")
	}
}
