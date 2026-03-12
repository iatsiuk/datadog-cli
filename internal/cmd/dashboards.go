package cmd

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
)

type dashboardsAPI struct {
	api *datadogV1.DashboardsApi
	ctx context.Context
}

func defaultDashboardsAPI() (*dashboardsAPI, error) { //nolint:unused
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &dashboardsAPI{api: datadogV1.NewDashboardsApi(c), ctx: ctx}, nil
}

// NewDashboardsCommand returns the dashboards cobra command group.
func NewDashboardsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Manage Datadog dashboards",
	}
	return cmd
}
