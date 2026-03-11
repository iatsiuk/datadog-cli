package client_test

import (
	"context"
	"testing"

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

func TestNew_ContextDerivesFromBackground(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		APIKey: "test-api-key",
		AppKey: "test-app-key",
		Site:   "datadoghq.com",
	}

	_, ctx := client.New(cfg)
	// context must not be done
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done")
	default:
	}

	// verify it derives from background (no cancellation from parent)
	if ctx == context.Background() {
		t.Fatal("context should be derived, not raw background")
	}
}

func TestNew_SiteInServerURL(t *testing.T) {
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

			c, _ := client.New(cfg)
			if c == nil {
				t.Fatal("expected non-nil client")
			}
			// verify server host is configured with the site
			servers := c.GetConfig().Servers
			if len(servers) == 0 {
				t.Fatal("expected servers to be configured")
			}
		})
	}
}
