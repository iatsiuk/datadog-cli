package cmd

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type dashboardsAPI struct {
	api *datadogV1.DashboardsApi
	ctx context.Context
}

func defaultDashboardsAPI() (*dashboardsAPI, error) {
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
	cmd.AddCommand(newDashboardsListCmd(defaultDashboardsAPI))
	return cmd
}

func newDashboardsListCmd(mkAPI func() (*dashboardsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.ListDashboards(dapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list dashboards: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			dashboards := resp.GetDashboards()
			if asJSON {
				if dashboards == nil {
					dashboards = []datadogV1.DashboardSummaryDefinition{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), dashboards)
			}

			headers := []string{"ID", "TITLE", "LAYOUT", "URL", "CREATED", "MODIFIED"}
			var rows [][]string
			for _, d := range dashboards {
				id := d.GetId()
				title := d.GetTitle()
				layout := string(d.GetLayoutType())
				url := d.GetUrl()
				created := ""
				if t := d.CreatedAt; t != nil {
					created = t.UTC().Format("2006-01-02")
				}
				modified := ""
				if t := d.ModifiedAt; t != nil {
					modified = t.UTC().Format("2006-01-02")
				}
				rows = append(rows, []string{id, title, layout, url, created, modified})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}
