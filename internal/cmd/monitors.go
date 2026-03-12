package cmd

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
)

type monitorsAPI struct {
	api *datadogV1.MonitorsApi
	ctx context.Context
}

func defaultMonitorsAPI() (*monitorsAPI, error) { //nolint:unused
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &monitorsAPI{api: datadogV1.NewMonitorsApi(c), ctx: ctx}, nil
}

// NewMonitorsCommand returns the monitors cobra command group.
func NewMonitorsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitors",
		Short: "Manage Datadog monitors",
	}
	return cmd
}
