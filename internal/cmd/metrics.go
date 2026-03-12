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
	cmd.AddCommand(newMetricsMetadataCmd(defaultMetricsV1API))
	cmd.AddCommand(newMetricsTagConfigCmd(defaultMetricsV2API))
	cmd.AddCommand(newMetricsTagsCmd(defaultMetricsV2API))
	cmd.AddCommand(newMetricsVolumesCmd(defaultMetricsV2API))
	cmd.AddCommand(newMetricsAssetsCmd(defaultMetricsV2API))
	cmd.AddCommand(newMetricsEstimateCmd(defaultMetricsV2API))
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
	cmd.Flags().StringVar(&fromStr, "from", "", "start time: unix timestamp or relative (e.g. now-1h) (required)")
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

			var groupCols []*datadogV2.GroupScalarColumn
			var dataCols []*datadogV2.DataScalarColumn
			if resp.Data != nil && resp.Data.Attributes != nil {
				for _, col := range resp.Data.Attributes.GetColumns() {
					if col.GroupScalarColumn != nil {
						groupCols = append(groupCols, col.GroupScalarColumn)
					} else if col.DataScalarColumn != nil {
						dataCols = append(dataCols, col.DataScalarColumn)
					}
				}
			}

			var headers []string
			var rows [][]string
			if len(groupCols) == 0 {
				headers = []string{"NAME", "VALUE"}
				for _, dc := range dataCols {
					name := dc.GetName()
					for _, v := range dc.GetValues() {
						if v == nil {
							continue
						}
						rows = append(rows, []string{name, strconv.FormatFloat(*v, 'f', -1, 64)})
					}
				}
			} else {
				multiData := len(dataCols) > 1
				if multiData {
					headers = append(headers, "NAME")
				}
				for _, gc := range groupCols {
					headers = append(headers, strings.ToUpper(gc.GetName()))
				}
				headers = append(headers, "VALUE")
				for _, dc := range dataCols {
					for i, v := range dc.GetValues() {
						if v == nil {
							continue
						}
						row := make([]string, 0, len(headers))
						if multiData {
							row = append(row, dc.GetName())
						}
						for _, gc := range groupCols {
							gcVals := gc.GetValues()
							if i < len(gcVals) && len(gcVals[i]) > 0 {
								row = append(row, gcVals[i][0])
							} else {
								row = append(row, "")
							}
						}
						row = append(row, strconv.FormatFloat(*v, 'f', -1, 64))
						rows = append(rows, row)
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
				for _, seriesVals := range values {
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

func newMetricsMetadataCmd(mkAPI func() (*metricsV1API, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Manage metric metadata",
	}
	cmd.AddCommand(newMetricsMetadataShowCmd(mkAPI))
	cmd.AddCommand(newMetricsMetadataUpdateCmd(mkAPI))
	return cmd
}

func newMetricsMetadataShowCmd(mkAPI func() (*metricsV1API, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <metric>",
		Short: "Show metadata for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.GetMetricMetadata(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get metric metadata: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"type", resp.GetType()},
				{"description", resp.GetDescription()},
				{"unit", resp.GetUnit()},
				{"per_unit", resp.GetPerUnit()},
				{"short_name", resp.GetShortName()},
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newMetricsMetadataUpdateCmd(mkAPI func() (*metricsV1API, error)) *cobra.Command {
	var (
		metricType  string
		description string
		unit        string
		perUnit     string
		shortName   string
	)

	cmd := &cobra.Command{
		Use:   "update <metric>",
		Short: "Update metadata for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			body := datadogV1.MetricMetadata{}
			if cmd.Flags().Changed("type") {
				body.Type = &metricType
			}
			if cmd.Flags().Changed("description") {
				body.Description = &description
			}
			if cmd.Flags().Changed("unit") {
				body.Unit = &unit
			}
			if cmd.Flags().Changed("per-unit") {
				body.PerUnit = &perUnit
			}
			if cmd.Flags().Changed("short-name") {
				body.ShortName = &shortName
			}

			_, httpResp, err := mapi.api.UpdateMetricMetadata(mapi.ctx, args[0], body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update metric metadata: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "updated")
			return err
		},
	}

	cmd.Flags().StringVar(&metricType, "type", "", "metric type (e.g. gauge, count, rate)")
	cmd.Flags().StringVar(&description, "description", "", "metric description")
	cmd.Flags().StringVar(&unit, "unit", "", "metric unit")
	cmd.Flags().StringVar(&perUnit, "per-unit", "", "per unit denominator")
	cmd.Flags().StringVar(&shortName, "short-name", "", "metric short name")
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

func newMetricsTagsCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "tags <metric>",
		Short: "List tags for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListTagsByMetricName(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list tags by metric name: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"TAG", "TYPE"}
			var rows [][]string
			if resp.Data != nil && resp.Data.Attributes != nil {
				for _, tag := range resp.Data.Attributes.GetTags() {
					rows = append(rows, []string{tag, "indexed"})
				}
				for _, tag := range resp.Data.Attributes.GetIngestedTags() {
					rows = append(rows, []string{tag, "ingested"})
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newMetricsVolumesCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "volumes <metric>",
		Short: "Show ingested and indexed volumes for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListVolumesByMetricName(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list volumes by metric name: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"FIELD", "VALUE"}
			var rows [][]string
			if resp.Data != nil {
				if v := resp.Data.MetricIngestedIndexedVolume; v != nil && v.Attributes != nil {
					if v.Attributes.IngestedVolume != nil {
						rows = append(rows, []string{"ingested_volume", strconv.FormatInt(*v.Attributes.IngestedVolume, 10)})
					}
					if v.Attributes.IndexedVolume != nil {
						rows = append(rows, []string{"indexed_volume", strconv.FormatInt(*v.Attributes.IndexedVolume, 10)})
					}
				} else if v := resp.Data.MetricDistinctVolume; v != nil && v.Attributes != nil {
					if v.Attributes.DistinctVolume != nil {
						rows = append(rows, []string{"distinct_volume", strconv.FormatInt(*v.Attributes.DistinctVolume, 10)})
					}
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newMetricsAssetsCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "assets <metric>",
		Short: "Show assets related to a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListMetricAssets(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list metric assets: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"TYPE", "ID", "TITLE"}
			var rows [][]string
			for _, item := range resp.GetIncluded() {
				if d := item.MetricDashboardAsset; d != nil {
					title := ""
					if d.Attributes != nil {
						title = d.Attributes.GetTitle()
					}
					rows = append(rows, []string{"dashboard", d.GetId(), title})
				} else if m := item.MetricMonitorAsset; m != nil {
					title := ""
					if m.Attributes != nil {
						title = m.Attributes.GetTitle()
					}
					rows = append(rows, []string{"monitor", m.GetId(), title})
				} else if n := item.MetricNotebookAsset; n != nil {
					title := ""
					if n.Attributes != nil {
						title = n.Attributes.GetTitle()
					}
					rows = append(rows, []string{"notebook", n.GetId(), title})
				} else if s := item.MetricSLOAsset; s != nil {
					title := ""
					if s.Attributes != nil {
						title = s.Attributes.GetTitle()
					}
					rows = append(rows, []string{"slo", s.GetId(), title})
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newMetricsEstimateCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var (
		filterGroups          string
		filterNumAggregations int32
		filterPct             bool
		filterHoursAgo        int32
		filterTimespanH       int32
	)

	cmd := &cobra.Command{
		Use:   "estimate <metric>",
		Short: "Estimate cardinality for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewEstimateMetricsOutputSeriesOptionalParameters()
			if cmd.Flags().Changed("filter-groups") {
				opts = opts.WithFilterGroups(filterGroups)
			}
			if cmd.Flags().Changed("filter-num-aggregations") {
				opts = opts.WithFilterNumAggregations(filterNumAggregations)
			}
			if cmd.Flags().Changed("filter-pct") {
				opts = opts.WithFilterPct(filterPct)
			}
			if cmd.Flags().Changed("filter-hours-ago") {
				opts = opts.WithFilterHoursAgo(filterHoursAgo)
			}
			if cmd.Flags().Changed("filter-timespan-h") {
				opts = opts.WithFilterTimespanH(filterTimespanH)
			}

			resp, httpResp, err := mapi.api.EstimateMetricsOutputSeries(mapi.ctx, args[0], *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("estimate metrics output series: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"FIELD", "VALUE"}
			var rows [][]string
			if data := resp.Data; data != nil {
				if attrs := data.Attributes; attrs != nil {
					if t := attrs.GetEstimateType(); t != "" {
						rows = append(rows, []string{"estimate_type", string(t)})
					}
					rows = append(rows, []string{"estimated_output_series", strconv.FormatInt(attrs.GetEstimatedOutputSeries(), 10)})
					if t := attrs.GetEstimatedAt(); !t.IsZero() {
						rows = append(rows, []string{"estimated_at", t.UTC().Format(time.RFC3339)})
					}
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&filterGroups, "filter-groups", "", "tag keys to filter by (comma-separated)")
	cmd.Flags().Int32Var(&filterNumAggregations, "filter-num-aggregations", 0, "number of aggregations")
	cmd.Flags().BoolVar(&filterPct, "filter-pct", false, "include percentile aggregations")
	cmd.Flags().Int32Var(&filterHoursAgo, "filter-hours-ago", 0, "number of hours ago to start")
	cmd.Flags().Int32Var(&filterTimespanH, "filter-timespan-h", 0, "timespan in hours")
	return cmd
}

func newMetricsTagConfigCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag-config",
		Short: "Manage metric tag configurations",
	}
	cmd.AddCommand(newMetricsTagConfigListCmd(mkAPI))
	cmd.AddCommand(newMetricsTagConfigShowCmd(mkAPI))
	cmd.AddCommand(newMetricsTagConfigCreateCmd(mkAPI))
	cmd.AddCommand(newMetricsTagConfigUpdateCmd(mkAPI))
	cmd.AddCommand(newMetricsTagConfigDeleteCmd(mkAPI))
	return cmd
}

func newMetricsTagConfigListCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tag configurations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListTagConfigurations(mapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list tag configurations: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			headers := []string{"METRIC", "TYPE", "TAGS"}
			var rows [][]string
			for _, item := range resp.GetData() {
				tc := item.MetricTagConfiguration
				if tc == nil {
					continue
				}
				name := tc.GetId()
				metricType := ""
				tags := ""
				if attrs := tc.Attributes; attrs != nil {
					metricType = string(attrs.GetMetricType())
					tags = strings.Join(attrs.GetTags(), ", ")
				}
				rows = append(rows, []string{name, metricType, tags})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newMetricsTagConfigShowCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <metric>",
		Short: "Show tag configuration for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := mapi.api.ListTagConfigurationByName(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get tag configuration: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			metricType := ""
			tags := ""
			id := ""
			if data := resp.Data; data != nil {
				id = data.GetId()
				if attrs := data.Attributes; attrs != nil {
					metricType = string(attrs.GetMetricType())
					tags = strings.Join(attrs.GetTags(), ", ")
				}
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"metric", id},
				{"type", metricType},
				{"tags", tags},
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newMetricsTagConfigCreateCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var (
		tags       []string
		metricType string
	)

	cmd := &cobra.Command{
		Use:   "create <metric>",
		Short: "Create a tag configuration for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(tags) == 0 {
				return fmt.Errorf("--tags is required")
			}

			mt, err := datadogV2.NewMetricTagConfigurationMetricTypesFromValue(metricType)
			if err != nil {
				return fmt.Errorf("--metric-type: %w", err)
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewMetricTagConfigurationCreateAttributes(*mt, tags)
			data := datadogV2.NewMetricTagConfigurationCreateData(args[0], datadogV2.METRICTAGCONFIGURATIONTYPE_MANAGE_TAGS)
			data.SetAttributes(*attrs)
			body := datadogV2.NewMetricTagConfigurationCreateRequest(*data)

			_, httpResp, err := mapi.api.CreateTagConfiguration(mapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create tag configuration: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "created")
			return err
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tags", nil, "tag keys to include (repeatable)")
	cmd.Flags().StringVar(&metricType, "metric-type", "gauge", "metric type: gauge, count, rate, distribution")
	return cmd
}

func newMetricsTagConfigUpdateCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var tags []string

	cmd := &cobra.Command{
		Use:   "update <metric>",
		Short: "Update a tag configuration for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("tags") {
				return fmt.Errorf("at least one flag must be provided (--tags)")
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewMetricTagConfigurationUpdateAttributes()
			if cmd.Flags().Changed("tags") {
				attrs.SetTags(tags)
			}

			data := datadogV2.NewMetricTagConfigurationUpdateData(args[0], datadogV2.METRICTAGCONFIGURATIONTYPE_MANAGE_TAGS)
			data.SetAttributes(*attrs)
			body := datadogV2.NewMetricTagConfigurationUpdateRequest(*data)

			_, httpResp, err := mapi.api.UpdateTagConfiguration(mapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update tag configuration: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "updated")
			return err
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tags", nil, "tag keys to include (repeatable)")
	return cmd
}

func newMetricsTagConfigDeleteCmd(mkAPI func() (*metricsV2API, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <metric>",
		Short: "Delete a tag configuration for a metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deletion of tag configuration for %q", args[0])
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := mapi.api.DeleteTagConfiguration(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete tag configuration: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return err
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
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
