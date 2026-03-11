package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
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

type metricsV2API struct {
	api *datadogV2.MetricsApi
	ctx context.Context
}

func defaultMetricsV2API() (*metricsV2API, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &metricsV2API{api: datadogV2.NewMetricsApi(c), ctx: ctx}, nil
}

// NewMetricsCommand returns the metrics cobra command group.
func NewMetricsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Query and manage Datadog metrics",
	}
	cmd.AddCommand(newMetricsQueryCmd(defaultMetricsV1API))
	cmd.AddCommand(newMetricsSearchCmd(defaultMetricsV1API))
	cmd.AddCommand(newMetricsListCmd(defaultMetricsV1API))
	cmd.AddCommand(newMetricsScalarCmd(defaultMetricsV2API))
	cmd.AddCommand(newMetricsTimeseriesCmd(defaultMetricsV2API))
	cmd.AddCommand(newMetricsSubmitCmd(defaultMetricsV2API))
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

func newMetricsSearchCmd(mkAPI func() (*metricsV1API, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for metric names matching a query",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListMetrics(mapi.ctx, query) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search metrics: %w", err)
			}

			results := resp.GetResults()
			metrics := results.GetMetrics()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if metrics == nil {
					metrics = []string{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), metrics)
			}

			headers := []string{"METRIC"}
			rows := make([][]string, len(metrics))
			for i, m := range metrics {
				rows[i] = []string{m}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "metric name search query (required)")
	return cmd
}

func newMetricsListCmd(mkAPI func() (*metricsV1API, error)) *cobra.Command {
	var fromStr string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active metrics since a given time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fromUnix, err := parseUnixOrRelative(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListActiveMetrics(mapi.ctx, fromUnix)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list active metrics: %w", err)
			}

			metrics := resp.GetMetrics()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if metrics == nil {
					metrics = []string{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), metrics)
			}

			headers := []string{"METRIC"}
			rows := make([][]string, len(metrics))
			for i, m := range metrics {
				rows[i] = []string{m}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&fromStr, "from", "", "start time: unix timestamp or relative (e.g. now-1h) (required)")
	return cmd
}

func newMetricsScalarCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var (
		query      string
		fromStr    string
		toStr      string
		aggregator string
	)

	cmd := &cobra.Command{
		Use:   "scalar",
		Short: "Query scalar metrics data using formulas",
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

			agg, err := datadogV2.NewMetricsAggregatorFromValue(aggregator)
			if err != nil {
				return fmt.Errorf("--aggregator: %w", err)
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			scalarQuery := datadogV2.MetricsScalarQueryAsScalarQuery(
				datadogV2.NewMetricsScalarQuery(*agg, datadogV2.METRICSDATASOURCE_METRICS, query),
			)
			attrs := datadogV2.NewScalarFormulaRequestAttributes(
				fromUnix*1000, // ms
				[]datadogV2.ScalarQuery{scalarQuery},
				toUnix*1000, // ms
			)
			req := datadogV2.NewScalarFormulaQueryRequest(
				*datadogV2.NewScalarFormulaRequest(*attrs, datadogV2.SCALARFORMULAREQUESTTYPE_SCALAR_REQUEST),
			)

			resp, httpResp, err := mapi.api.QueryScalarData(mapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("query scalar data: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"NAME", "VALUE"}
			var rows [][]string
			if resp.Data != nil && resp.Data.Attributes != nil {
				for _, col := range resp.Data.Attributes.GetColumns() {
					dc := col.DataScalarColumn
					if dc == nil {
						continue
					}
					name := dc.GetName()
					for _, v := range dc.GetValues() {
						if v == nil {
							continue
						}
						val := strconv.FormatFloat(*v, 'f', -1, 64)
						rows = append(rows, []string{name, val})
					}
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "metric query (required)")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time: unix timestamp or relative (e.g. now-1h) (required)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time: unix timestamp or relative (e.g. now)")
	cmd.Flags().StringVar(&aggregator, "aggregator", "avg", "aggregation function (avg, sum, min, max, last)")
	return cmd
}

func newMetricsTimeseriesCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
	)

	cmd := &cobra.Command{
		Use:   "timeseries",
		Short: "Query timeseries metrics data using formulas (V2)",
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

			tsQuery := datadogV2.TimeseriesQuery{
				MetricsTimeseriesQuery: datadogV2.NewMetricsTimeseriesQuery(
					datadogV2.METRICSDATASOURCE_METRICS,
					query,
				),
			}
			attrs := datadogV2.NewTimeseriesFormulaRequestAttributes(
				fromUnix*1000, // ms
				[]datadogV2.TimeseriesQuery{tsQuery},
				toUnix*1000, // ms
			)
			req := datadogV2.TimeseriesFormulaQueryRequest{
				Data: datadogV2.TimeseriesFormulaRequest{
					Attributes: *attrs,
					Type:       datadogV2.TIMESERIESFORMULAREQUESTTYPE_TIMESERIES_REQUEST,
				},
			}

			resp, httpResp, err := mapi.api.QueryTimeseriesData(mapi.ctx, req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("query timeseries data: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"TIMESTAMP", "VALUE"}
			var rows [][]string
			if resp.Data != nil && resp.Data.Attributes != nil {
				times := resp.Data.Attributes.GetTimes()
				values := resp.Data.Attributes.GetValues()
				for seriesIdx, seriesVals := range values {
					_ = seriesIdx
					for i, v := range seriesVals {
						if v == nil || i >= len(times) {
							continue
						}
						ts := time.Unix(times[i]/1000, 0).UTC().Format(time.RFC3339)
						val := strconv.FormatFloat(*v, 'f', -1, 64)
						rows = append(rows, []string{ts, val})
					}
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "metric query (required)")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time: unix timestamp or relative (e.g. now-1h) (required)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time: unix timestamp or relative (e.g. now)")
	return cmd
}

func newMetricsSubmitCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var (
		metricName string
		metricType string
		points     []string
		tags       []string
	)

	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit metrics to Datadog",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if metricName == "" {
				return fmt.Errorf("--metric is required")
			}
			if len(points) == 0 {
				return fmt.Errorf("--points is required")
			}

			var intakeType datadogV2.MetricIntakeType
			switch metricType {
			case "gauge":
				intakeType = datadogV2.METRICINTAKETYPE_GAUGE
			case "count":
				intakeType = datadogV2.METRICINTAKETYPE_COUNT
			case "rate":
				intakeType = datadogV2.METRICINTAKETYPE_RATE
			default:
				return fmt.Errorf("--type: invalid value %q (must be gauge, count, or rate)", metricType)
			}

			metricPoints, err := parseMetricPoints(points)
			if err != nil {
				return fmt.Errorf("--points: %w", err)
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			series := datadogV2.NewMetricSeries(metricName, metricPoints)
			series.Type = &intakeType
			if len(tags) > 0 {
				series.Tags = tags
			}

			payload := datadogV2.NewMetricPayload([]datadogV2.MetricSeries{*series})
			_, httpResp, err := mapi.api.SubmitMetrics(mapi.ctx, *payload)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("submit metrics: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "submitted")
			return err
		},
	}

	cmd.Flags().StringVar(&metricName, "metric", "", "metric name (required)")
	cmd.Flags().StringVar(&metricType, "type", "gauge", "metric type: gauge, count, rate")
	cmd.Flags().StringArrayVar(&points, "points", nil, "data points in timestamp:value format (repeatable)")
	cmd.Flags().StringArrayVar(&tags, "tags", nil, "tags in key:value format (repeatable)")
	return cmd
}

// parseMetricPoints parses a slice of "timestamp:value" strings into MetricPoint slice.
func parseMetricPoints(rawPoints []string) ([]datadogV2.MetricPoint, error) {
	pts := make([]datadogV2.MetricPoint, 0, len(rawPoints))
	for _, raw := range rawPoints {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format %q (expected timestamp:value)", raw)
		}
		ts, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp in %q: %w", raw, err)
		}
		val, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value in %q: %w", raw, err)
		}
		pt := datadogV2.NewMetricPoint()
		pt.Timestamp = &ts
		pt.Value = &val
		pts = append(pts, *pt)
	}
	return pts, nil
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
