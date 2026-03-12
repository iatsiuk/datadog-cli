package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	cmd.AddCommand(newMonitorsShowCmd(defaultMonitorsAPI))
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

var errMonitorIDRequired = errors.New("--id is required")

func newMonitorsShowCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var monitorID int64

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if monitorID == 0 {
				return errMonitorIDRequired
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			m, httpResp, err := mapi.api.GetMonitor(mapi.ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), m)
			}

			rows := [][]string{
				{"ID", fmt.Sprintf("%d", m.GetId())},
				{"NAME", m.GetName()},
				{"TYPE", string(m.GetType())},
				{"STATUS", string(m.GetOverallState())},
				{"QUERY", m.GetQuery()},
				{"MESSAGE", m.GetMessage()},
				{"TAGS", strings.Join(m.GetTags(), ", ")},
			}
			if t := m.GetCreated(); !t.IsZero() {
				rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
			}
			if t := m.GetModified(); !t.IsZero() {
				rows = append(rows, []string{"MODIFIED", t.Format("2006-01-02 15:04:05")})
			}

			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().Int64Var(&monitorID, "id", 0, "monitor ID")
	return cmd
}
