package cmd

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type downtimesAPI struct {
	api *datadogV2.DowntimesApi
	ctx context.Context
}

func defaultDowntimesAPI() (*downtimesAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &downtimesAPI{api: datadogV2.NewDowntimesApi(c), ctx: ctx}, nil
}

func newDowntimeCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "downtime",
		Short: "Manage Datadog downtimes",
	}
	cmd.AddCommand(newDowntimeListCmd(mkAPI))
	return cmd
}

func newDowntimeListCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	var currentOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List downtimes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListDowntimesOptionalParameters()
			if currentOnly {
				opts = opts.WithCurrentOnly(true)
			}

			resp, httpResp, err := dapi.api.ListDowntimes(dapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list downtimes: %w", err)
			}

			downtimes := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if downtimes == nil {
					downtimes = []datadogV2.DowntimeResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), downtimes)
			}

			headers := []string{"ID", "SCOPE", "MONITOR_ID", "STATUS", "START", "END"}
			rows := make([][]string, 0, len(downtimes))
			for _, d := range downtimes {
				attrs := d.GetAttributes()

				scope := attrs.GetScope()
				status := string(attrs.GetStatus())

				monitorID := ""
				if mi := attrs.MonitorIdentifier; mi != nil && mi.DowntimeMonitorIdentifierId != nil {
					monitorID = fmt.Sprintf("%d", mi.DowntimeMonitorIdentifierId.MonitorId)
				}

				start := ""
				end := ""
				if sched := attrs.Schedule; sched != nil {
					if sched.DowntimeScheduleOneTimeResponse != nil {
						t := sched.DowntimeScheduleOneTimeResponse.Start
						if !t.IsZero() {
							start = t.Format("2006-01-02 15:04:05")
						}
						if e := sched.DowntimeScheduleOneTimeResponse.End.Get(); e != nil {
							end = e.Format("2006-01-02 15:04:05")
						}
					}
				}

				rows = append(rows, []string{d.GetId(), scope, monitorID, status, start, end})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().BoolVar(&currentOnly, "current-only", false, "only return active downtimes")
	return cmd
}
