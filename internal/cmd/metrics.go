package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type metricsV1API struct {
	api *datadogV1.MetricsApi
	ctx context.Context
}

func defaultMetricsV1API() (*metricsV1API, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &metricsV1API{api: datadogV1.NewMetricsApi(c), ctx: ctx}, nil
}

// NewMetricsCommand returns the metrics cobra command group.
func NewMetricsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Query and manage Datadog metrics",
	}
	cmd.AddCommand(newMetricsQueryCmd(defaultMetricsV1API))
	return cmd
}

func newMetricsQueryCmd(mkAPI func() (*metricsV1API, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query timeseries metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}

			fromUnix, err := parseUnixOrRelative(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toUnix, err := parseUnixOrRelative(toStr)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.QueryMetrics(mapi.ctx, fromUnix, toUnix, query)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("query metrics: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				series := resp.GetSeries()
				if series == nil {
					series = []datadogV1.MetricsQueryMetadata{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), series)
			}

			headers := []string{"TIMESTAMP", "VALUE"}
			var rows [][]string
			for _, s := range resp.GetSeries() {
				for _, pt := range s.GetPointlist() {
					if len(pt) < 2 || pt[0] == nil || pt[1] == nil {
						continue
					}
					ts := time.Unix(int64(*pt[0])/1000, 0).UTC().Format(time.RFC3339)
					val := strconv.FormatFloat(*pt[1], 'f', -1, 64)
					rows = append(rows, []string{ts, val})
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "metric query (required)")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time: unix timestamp or relative (e.g. now-1h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time: unix timestamp or relative (e.g. now)")
	return cmd
}

// parseUnixOrRelative parses a unix timestamp string or relative time expression.
func parseUnixOrRelative(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("time value is required")
	}
	// try parsing as integer unix timestamp
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	t, err := parseRelativeTime(s)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}
