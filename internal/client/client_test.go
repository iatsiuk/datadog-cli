package client_test

import (
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
)

func TestNew_ReturnsClient(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		APIKey: "test-api-key",
		AppKey: "test-app-key",
		Site:   "datadoghq.com",
	}

	c, ctx := client.New(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestNew_AuthKeysInContext(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		APIKey: "test-api-key",
		AppKey: "test-app-key",
		Site:   "datadoghq.com",
	}

	_, ctx := client.New(cfg)

	keys, ok := ctx.Value(datadog.ContextAPIKeys).(map[string]datadog.APIKey)
	if !ok {
		t.Fatal("expected ContextAPIKeys in context")
	}
	if keys["apiKeyAuth"].Key != cfg.APIKey {
		t.Errorf("apiKeyAuth: got %q, want %q", keys["apiKeyAuth"].Key, cfg.APIKey)
	}
	if keys["appKeyAuth"].Key != cfg.AppKey {
		t.Errorf("appKeyAuth: got %q, want %q", keys["appKeyAuth"].Key, cfg.AppKey)
	}
}

func TestNew_SiteInContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		site string
	}{
		{site: "datadoghq.com"},
		{site: "datadoghq.eu"},
		{site: "us3.datadoghq.com"},
		{site: "ap1.datadoghq.com"},
	}

	for _, tt := range tests {
		t.Run(tt.site, func(t *testing.T) {
			t.Parallel()

			cfg := config.Config{
				APIKey: "key",
				AppKey: "key",
				Site:   tt.site,
			}

			_, ctx := client.New(cfg)

			vars, ok := ctx.Value(datadog.ContextServerVariables).(map[string]string)
			if !ok {
				t.Fatal("expected ContextServerVariables in context")
			}
			if vars["site"] != tt.site {
				t.Errorf("site: got %q, want %q", vars["site"], tt.site)
			}
		})
	}
}
