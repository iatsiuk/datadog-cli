package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsMetricAPI struct {
	api *datadogV2.LogsMetricsApi
	ctx context.Context
}

func defaultLogsMetricAPI() (*logsMetricAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &logsMetricAPI{api: datadogV2.NewLogsMetricsApi(c), ctx: ctx}, nil
}

func newLogsMetricCmd(mkAPI func() (*logsMetricAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metric",
		Short: "Manage log-based metrics",
	}
	cmd.AddCommand(newLogsMetricListCmd(mkAPI))
	cmd.AddCommand(newLogsMetricShowCmd(mkAPI))
	cmd.AddCommand(newLogsMetricCreateCmd(mkAPI))
	cmd.AddCommand(newLogsMetricUpdateCmd(mkAPI))
	cmd.AddCommand(newLogsMetricDeleteCmd(mkAPI))
	return cmd
}

func newLogsMetricListCmd(mkAPI func() (*logsMetricAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List log-based metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListLogsMetrics(mapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list log metrics: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.LogsMetricResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "COMPUTE", "FILTER", "GROUP-BY"}
			var rows [][]string
			for _, m := range resp.GetData() {
				id := m.GetId()
				compute, filter, groupBy := metricResponseFields(m.Attributes)
				rows = append(rows, []string{id, compute, filter, groupBy})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsMetricShowCmd(mkAPI func() (*logsMetricAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a log-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.GetLogsMetric(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get log metric: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			d := resp.GetData()
			id := d.GetId()
			compute, filter, groupBy := metricResponseFields(d.Attributes)
			headers := []string{"ID", "COMPUTE", "FILTER", "GROUP-BY"}
			rows := [][]string{{id, compute, filter, groupBy}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsMetricCreateCmd(mkAPI func() (*logsMetricAPI, error)) *cobra.Command {
	var (
		id          string
		computeType string
		path        string
		filter      string
		groupBy     string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a log-based metric",
		RunE: func(cmd *cobra.Command, _ []string) error {
			aggType, err := datadogV2.NewLogsMetricComputeAggregationTypeFromValue(computeType)
			if err != nil {
				return fmt.Errorf("--compute-type: unknown value %q", computeType)
			}

			compute := datadogV2.NewLogsMetricCompute(*aggType)
			if path != "" {
				compute.SetPath(path)
			}

			attrs := datadogV2.NewLogsMetricCreateAttributes(*compute)
			if filter != "" {
				f := datadogV2.NewLogsMetricFilter()
				f.SetQuery(filter)
				attrs.SetFilter(*f)
			}
			if groupBy != "" {
				attrs.SetGroupBy(parseMetricGroupBy(groupBy))
			}

			data := datadogV2.NewLogsMetricCreateData(*attrs, id, datadogV2.LOGSMETRICTYPE_LOGS_METRICS)
			body := datadogV2.NewLogsMetricCreateRequest(*data)

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.CreateLogsMetric(mapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create log metric: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "metric name/id (required)")
	cmd.Flags().StringVar(&computeType, "compute-type", "count", "aggregation type: count or distribution")
	cmd.Flags().StringVar(&path, "path", "", "path for distribution metrics")
	cmd.Flags().StringVar(&filter, "filter", "", "log filter query")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "comma-separated paths to group by")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newLogsMetricUpdateCmd(mkAPI func() (*logsMetricAPI, error)) *cobra.Command {
	var (
		filter  string
		groupBy string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a log-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attrs := datadogV2.NewLogsMetricUpdateAttributes()
			if filter != "" {
				f := datadogV2.NewLogsMetricFilter()
				f.SetQuery(filter)
				attrs.SetFilter(*f)
			}
			if groupBy != "" {
				attrs.SetGroupBy(parseMetricGroupBy(groupBy))
			}

			data := datadogV2.NewLogsMetricUpdateData(*attrs, datadogV2.LOGSMETRICTYPE_LOGS_METRICS)
			body := datadogV2.NewLogsMetricUpdateRequest(*data)

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.UpdateLogsMetric(mapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update log metric: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "log filter query")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "comma-separated paths to group by")
	return cmd
}

func newLogsMetricDeleteCmd(mkAPI func() (*logsMetricAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a log-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of metric %q", args[0])
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := mapi.api.DeleteLogsMetric(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete log metric: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted metric %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

// metricResponseFields extracts compute, filter, and group-by strings from response attributes.
func metricResponseFields(attrs *datadogV2.LogsMetricResponseAttributes) (compute, filter, groupBy string) {
	if attrs == nil {
		return
	}
	if c := attrs.Compute; c != nil {
		compute = string(c.GetAggregationType())
		if p := c.GetPath(); p != "" {
			compute = compute + ":" + p
		}
	}
	if f := attrs.Filter; f != nil {
		filter = f.GetQuery()
	}
	parts := make([]string, 0, len(attrs.GroupBy))
	for _, g := range attrs.GroupBy {
		parts = append(parts, g.GetPath())
	}
	groupBy = strings.Join(parts, ",")
	return
}

// parseMetricGroupBy parses a comma-separated list of paths into LogsMetricGroupBy slice.
func parseMetricGroupBy(s string) []datadogV2.LogsMetricGroupBy {
	parts := strings.Split(s, ",")
	groups := make([]datadogV2.LogsMetricGroupBy, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			groups = append(groups, *datadogV2.NewLogsMetricGroupBy(p))
		}
	}
	return groups
}
