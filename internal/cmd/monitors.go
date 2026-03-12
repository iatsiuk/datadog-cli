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

type monitorsAPI struct {
	api *datadogV1.MonitorsApi
	ctx context.Context
}

func defaultMonitorsAPI() (*monitorsAPI, error) {
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
	cmd.AddCommand(newMonitorsListCmd(defaultMonitorsAPI))
	return cmd
}

func newMonitorsListCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		name     string
		tags     string
		pageSize int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List monitors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewListMonitorsOptionalParameters()
			if name != "" {
				opts = opts.WithName(name)
			}
			if tags != "" {
				opts = opts.WithTags(tags)
			}
			if pageSize > 0 {
				opts = opts.WithPageSize(int32(pageSize)) //nolint:gosec
			}

			monitors, httpResp, err := mapi.api.ListMonitors(mapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list monitors: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if monitors == nil {
					monitors = []datadogV1.Monitor{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), monitors)
			}

			headers := []string{"ID", "NAME", "TYPE", "STATUS", "QUERY"}
			rows := make([][]string, 0, len(monitors))
			for _, m := range monitors {
				id := fmt.Sprintf("%d", m.GetId())
				rows = append(rows, []string{
					id,
					m.GetName(),
					string(m.GetType()),
					string(m.GetOverallState()),
					m.GetQuery(),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "filter by monitor name")
	cmd.Flags().StringVar(&tags, "tags", "", "filter by tags, e.g. env:prod,service:web")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "number of monitors per page")
	return cmd
}
