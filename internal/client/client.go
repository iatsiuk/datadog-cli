package client

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"

	"github.com/iatsiuk/datadog-cli/internal/config"
)

// New creates a configured Datadog API client and context with auth keys.
// The returned context must be passed to all API calls.
func New(cfg config.Config) (*datadog.APIClient, context.Context) {
	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {Key: cfg.APIKey},
			"appKeyAuth": {Key: cfg.AppKey},
		},
	)

	ctx = context.WithValue(
		ctx,
		datadog.ContextServerVariables,
		map[string]string{"site": cfg.Site},
	)

	return datadog.NewAPIClient(datadog.NewConfiguration()), ctx
}
