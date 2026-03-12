package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type rumAPI struct {
	api *datadogV2.RUMApi
	ctx context.Context
}

func defaultRUMAPI() (*rumAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rumAPI{api: datadogV2.NewRUMApi(c), ctx: ctx}, nil
}

type rumMetricsAPI struct {
	api *datadogV2.RumMetricsApi
	ctx context.Context
}

func defaultRUMMetricsAPI() (*rumMetricsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rumMetricsAPI{api: datadogV2.NewRumMetricsApi(c), ctx: ctx}, nil
}

type rumRetentionFiltersAPI struct {
	api *datadogV2.RumRetentionFiltersApi
	ctx context.Context
}

func defaultRUMRetentionFiltersAPI() (*rumRetentionFiltersAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rumRetentionFiltersAPI{api: datadogV2.NewRumRetentionFiltersApi(c), ctx: ctx}, nil
}

type rumPlaylistsAPI struct {
	api *datadogV2.RumReplayPlaylistsApi
	ctx context.Context
}

func defaultRUMPlaylistsAPI() (*rumPlaylistsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rumPlaylistsAPI{api: datadogV2.NewRumReplayPlaylistsApi(c), ctx: ctx}, nil
}

type rumHeatmapsAPI struct {
	api *datadogV2.RumReplayHeatmapsApi
	ctx context.Context
}

func defaultRUMHeatmapsAPI() (*rumHeatmapsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rumHeatmapsAPI{api: datadogV2.NewRumReplayHeatmapsApi(c), ctx: ctx}, nil
}

type rumSessionsAPI struct {
	sessions   *datadogV2.RumReplaySessionsApi
	viewership *datadogV2.RumReplayViewershipApi
	ctx        context.Context
}

func defaultRUMSessionsAPI() (*rumSessionsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rumSessionsAPI{
		sessions:   datadogV2.NewRumReplaySessionsApi(c),
		viewership: datadogV2.NewRumReplayViewershipApi(c),
		ctx:        ctx,
	}, nil
}

type rumAudienceAPI struct {
	api *datadogV2.RumAudienceManagementApi
	ctx context.Context
}

func defaultRUMAudienceAPI() (*rumAudienceAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	apiCfg := c.GetConfig()
	for _, op := range []string{
		"v2.ListConnections",
		"v2.CreateConnection",
		"v2.UpdateConnection",
		"v2.DeleteConnection",
		"v2.GetMapping",
		"v2.QueryUsers",
		"v2.QueryAccounts",
	} {
		apiCfg.SetUnstableOperationEnabled(op, true)
	}
	return &rumAudienceAPI{api: datadogV2.NewRumAudienceManagementApi(c), ctx: ctx}, nil
}

// NewRUMCommand returns the rum cobra command group.
func NewRUMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rum",
		Short: "Search and manage Datadog RUM",
	}
	cmd.AddCommand(newRUMSearchCmd(defaultRUMAPI))
	cmd.AddCommand(newRUMAggregateCmd(defaultRUMAPI))
	cmd.AddCommand(newRUMAppCmd(defaultRUMAPI))
	cmd.AddCommand(newRUMMetricCmd(defaultRUMMetricsAPI))
	cmd.AddCommand(newRUMRetentionFilterCmd(defaultRUMRetentionFiltersAPI))
	cmd.AddCommand(newRUMPlaylistCmd(defaultRUMPlaylistsAPI))
	cmd.AddCommand(newRUMHeatmapCmd(defaultRUMHeatmapsAPI))
	cmd.AddCommand(newRUMSessionCmd(defaultRUMSessionsAPI))
	cmd.AddCommand(newRUMAudienceCmd(defaultRUMAudienceAPI))
	return cmd
}

func newRUMSearchCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
		sortStr string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search RUM events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fromStr == "" {
				fromStr = "now-15m"
			}

			fromTime, err := parseRelativeTime(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toTime, err := parseRelativeTime(toStr)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}
			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListRUMEventsOptionalParameters().
				WithFilterFrom(fromTime.UTC()).
				WithFilterTo(toTime.UTC()).
				WithPageLimit(pageLimit)
			if query != "" {
				opts = opts.WithFilterQuery(query)
			}
			if sortStr != "" {
				s, err := datadogV2.NewRUMSortFromValue(sortStr)
				if err != nil {
					return fmt.Errorf("--sort: %w", err)
				}
				opts = opts.WithSort(*s)
			}

			resp, httpResp, err := rapi.api.ListRUMEvents(rapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search rum events: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.RUMEvent{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "TYPE", "APPLICATION", "VIEW", "DURATION"}
			var rows [][]string
			for _, event := range resp.GetData() {
				attrs := event.GetAttributes()
				ts := ""
				if t := attrs.Timestamp; t != nil {
					ts = t.UTC().Format(time.RFC3339)
				}
				extra := attrs.GetAttributes()
				eventType := strFromMap(extra, "type")
				appID := strFromMap(extra, "application.id")
				viewURL := strFromMap(extra, "view.url")
				duration := strFromMap(extra, "duration")
				rows = append(rows, []string{ts, eventType, appID, viewURL, duration})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "RUM search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-15m (default: now-15m)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of events to return")
	cmd.Flags().StringVar(&sortStr, "sort", "", "sort order: timestamp or -timestamp")
	return cmd
}

// strFromMap extracts a string value from a map[string]interface{}.
func strFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func newRUMAggregateCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		groupBy string
		compute string
	)

	cmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Aggregate RUM events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fromStr == "" {
				fromStr = "now-15m"
			}

			fromTime, err := parseRelativeTime(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toTime, err := parseRelativeTime(toStr)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}

			agg, metric, err := parseRUMComputeSpec(compute)
			if err != nil {
				return fmt.Errorf("--compute: %w", err)
			}

			filter := datadogV2.NewRUMQueryFilter()
			filter.SetFrom(fromTime.UTC().Format(time.RFC3339))
			filter.SetTo(toTime.UTC().Format(time.RFC3339))
			if query != "" {
				filter.SetQuery(query)
			}

			c := datadogV2.NewRUMCompute(agg)
			if metric != "" {
				c.SetMetric(metric)
			}

			req := datadogV2.NewRUMAggregateRequest()
			req.SetFilter(*filter)
			req.SetCompute([]datadogV2.RUMCompute{*c})

			if groupBy != "" {
				facets := strings.Split(groupBy, ",")
				groups := make([]datadogV2.RUMGroupBy, 0, len(facets))
				for _, f := range facets {
					f = strings.TrimSpace(f)
					if f != "" {
						groups = append(groups, *datadogV2.NewRUMGroupBy(f))
					}
				}
				req.SetGroupBy(groups)
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.AggregateRUMEvents(rapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("aggregate rum events: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			data := resp.GetData()
			if asJSON {
				buckets := data.GetBuckets()
				if buckets == nil {
					buckets = []datadogV2.RUMBucketResponse{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), buckets)
			}

			buckets := data.GetBuckets()
			if len(buckets) == 0 {
				return output.PrintTable(cmd.OutOrStdout(), nil, nil)
			}

			byKeys := sortedStringMapKeys(buckets[0].GetBy())
			computeKeys := sortedRUMComputeKeys(buckets[0].GetComputes())
			headers := make([]string, 0, len(byKeys)+len(computeKeys))
			for _, k := range byKeys {
				headers = append(headers, strings.ToUpper(k))
			}
			for _, k := range computeKeys {
				headers = append(headers, strings.ToUpper(k))
			}

			var rows [][]string
			for _, b := range buckets {
				row := make([]string, 0, len(byKeys)+len(computeKeys))
				for _, k := range byKeys {
					row = append(row, b.GetBy()[k])
				}
				for _, k := range computeKeys {
					v := b.GetComputes()[k]
					row = append(row, formatRUMBucketValue(v))
				}
				rows = append(rows, row)
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "RUM filter query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time (default: now-15m)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "comma-separated facets to group by")
	cmd.Flags().StringVar(&compute, "compute", "count", "aggregation spec: <function>[:<metric>]")
	return cmd
}

// parseRUMComputeSpec parses "count" or "sum:@metric" into RUM aggregation function and metric.
func parseRUMComputeSpec(s string) (datadogV2.RUMAggregationFunction, string, error) {
	parts := strings.SplitN(s, ":", 2)
	agg, err := datadogV2.NewRUMAggregationFunctionFromValue(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("unknown aggregation %q", parts[0])
	}
	metric := ""
	if len(parts) == 2 {
		metric = parts[1]
	}
	return *agg, metric, nil
}

func sortedStringMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedRUMComputeKeys(m map[string]datadogV2.RUMAggregateBucketValue) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatRUMBucketValue(v datadogV2.RUMAggregateBucketValue) string {
	switch {
	case v.RUMAggregateBucketValueSingleString != nil:
		return *v.RUMAggregateBucketValueSingleString
	case v.RUMAggregateBucketValueSingleNumber != nil:
		f := *v.RUMAggregateBucketValueSingleNumber
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	default:
		return ""
	}
}

func newRUMAppCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage RUM applications",
	}
	cmd.AddCommand(newRUMAppListCmd(mkAPI))
	cmd.AddCommand(newRUMAppShowCmd(mkAPI))
	cmd.AddCommand(newRUMAppCreateCmd(mkAPI))
	cmd.AddCommand(newRUMAppUpdateCmd(mkAPI))
	cmd.AddCommand(newRUMAppDeleteCmd(mkAPI))
	return cmd
}

func newRUMAppListCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List RUM applications",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.GetRUMApplications(rapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list rum applications: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.RUMApplicationList{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "TYPE"}
			var rows [][]string
			for _, app := range resp.GetData() {
				attrs := app.GetAttributes()
				id := app.GetId()
				rows = append(rows, []string{id, attrs.GetName(), attrs.GetType()})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRUMAppShowCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a RUM application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.GetRUMApplication(rapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get rum application: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			app := resp.GetData()
			attrs := app.GetAttributes()
			rows := [][]string{
				{"ID", app.GetId()},
				{"NAME", attrs.GetName()},
				{"TYPE", attrs.GetType()},
				{"CLIENT TOKEN", attrs.GetClientToken()},
				{"CREATED BY", attrs.GetCreatedByHandle()},
				{"UPDATED BY", attrs.GetUpdatedByHandle()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
}

func newRUMAppCreateCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	var (
		name    string
		appType string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a RUM application",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewRUMApplicationCreateAttributes(name)
			if appType != "" {
				attrs.SetType(appType)
			}
			data := datadogV2.NewRUMApplicationCreate(*attrs, datadogV2.RUMAPPLICATIONCREATETYPE_RUM_APPLICATION_CREATE)
			req := datadogV2.NewRUMApplicationCreateRequest(*data)

			resp, httpResp, err := rapi.api.CreateRUMApplication(rapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create rum application: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			app := resp.GetData()
			attrs2 := app.GetAttributes()
			rows := [][]string{
				{"ID", app.GetId()},
				{"NAME", attrs2.GetName()},
				{"TYPE", attrs2.GetType()},
				{"CLIENT TOKEN", attrs2.GetClientToken()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "application name (required)")
	cmd.Flags().StringVar(&appType, "type", "browser", "application type: browser, ios, android, react-native, flutter, roku, electron, unity, kotlin-multiplatform")
	return cmd
}

func newRUMAppUpdateCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	var (
		name    string
		appType string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a RUM application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("type") {
				return fmt.Errorf("at least one of --name or --type must be specified")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			updateAttrs := datadogV2.NewRUMApplicationUpdateAttributes()
			if cmd.Flags().Changed("name") {
				updateAttrs.SetName(name)
			}
			if appType != "" {
				updateAttrs.SetType(appType)
			}

			data := datadogV2.NewRUMApplicationUpdate(id, datadogV2.RUMAPPLICATIONUPDATETYPE_RUM_APPLICATION_UPDATE)
			data.SetAttributes(*updateAttrs)
			req := datadogV2.NewRUMApplicationUpdateRequest(*data)

			resp, httpResp, err := rapi.api.UpdateRUMApplication(rapi.ctx, id, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update rum application: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			app := resp.GetData()
			attrs := app.GetAttributes()
			rows := [][]string{
				{"ID", app.GetId()},
				{"NAME", attrs.GetName()},
				{"TYPE", attrs.GetType()},
				{"CLIENT TOKEN", attrs.GetClientToken()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new application name")
	cmd.Flags().StringVar(&appType, "type", "", "new application type")
	return cmd
}

func newRUMAppDeleteCmd(mkAPI func() (*rumAPI, error)) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a RUM application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("requires --yes flag to confirm deletion")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := rapi.api.DeleteRUMApplication(rapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete rum application: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newRUMMetricCmd(mkAPI func() (*rumMetricsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metric",
		Short: "Manage RUM-based metrics",
	}
	cmd.AddCommand(newRUMMetricListCmd(mkAPI))
	cmd.AddCommand(newRUMMetricShowCmd(mkAPI))
	cmd.AddCommand(newRUMMetricCreateCmd(mkAPI))
	cmd.AddCommand(newRUMMetricUpdateCmd(mkAPI))
	cmd.AddCommand(newRUMMetricDeleteCmd(mkAPI))
	return cmd
}

func newRUMMetricListCmd(mkAPI func() (*rumMetricsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List RUM-based metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := mapi.api.ListRumMetrics(mapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list rum metrics: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.RumMetricResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "EVENT TYPE", "COMPUTE", "FILTER", "GROUP BY"}
			var rows [][]string
			for _, m := range resp.GetData() {
				id := m.GetId()
				attrs := m.GetAttributes()
				compute := attrs.GetCompute()
				eventType := string(attrs.GetEventType())
				computeStr := string(compute.GetAggregationType())
				if compute.Path != nil {
					computeStr += ":" + *compute.Path
				}
				mFilter := attrs.GetFilter()
				filter := mFilter.GetQuery()
				var groupByPaths []string
				for _, gb := range attrs.GetGroupBy() {
					groupByPaths = append(groupByPaths, gb.GetPath())
				}
				rows = append(rows, []string{id, eventType, computeStr, filter, strings.Join(groupByPaths, ", ")})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRUMMetricShowCmd(mkAPI func() (*rumMetricsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a RUM-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := mapi.api.GetRumMetric(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get rum metric: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			m := resp.GetData()
			attrs := m.GetAttributes()
			compute := attrs.GetCompute()
			computeStr := string(compute.GetAggregationType())
			if compute.Path != nil {
				computeStr += ":" + *compute.Path
			}
			groupByPaths := make([]string, 0, len(attrs.GetGroupBy()))
			for _, g := range attrs.GetGroupBy() {
				groupByPaths = append(groupByPaths, g.GetPath())
			}
			showFilter := attrs.GetFilter()
			rows := [][]string{
				{"ID", m.GetId()},
				{"EVENT TYPE", string(attrs.GetEventType())},
				{"COMPUTE", computeStr},
				{"FILTER", showFilter.GetQuery()},
				{"GROUP BY", strings.Join(groupByPaths, ", ")},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
}

func newRUMMetricCreateCmd(mkAPI func() (*rumMetricsAPI, error)) *cobra.Command {
	var (
		metricID  string
		compute   string
		eventType string
		filter    string
		groupBy   string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a RUM-based metric",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if metricID == "" {
				return fmt.Errorf("--id is required")
			}
			if compute == "" {
				return fmt.Errorf("--compute is required")
			}

			aggType, metricPath, err := parseRUMMetricComputeSpec(compute)
			if err != nil {
				return fmt.Errorf("--compute: %w", err)
			}

			et, err := datadogV2.NewRumMetricEventTypeFromValue(eventType)
			if err != nil {
				return fmt.Errorf("--event-type: %w", err)
			}

			c := datadogV2.NewRumMetricCompute(aggType)
			if metricPath != "" {
				c.SetPath(metricPath)
			}

			attrs := datadogV2.NewRumMetricCreateAttributes(*c, *et)
			if filter != "" {
				f := datadogV2.NewRumMetricFilter(filter)
				attrs.SetFilter(*f)
			}
			if groupBy != "" {
				facets := strings.Split(groupBy, ",")
				groups := make([]datadogV2.RumMetricGroupBy, 0, len(facets))
				for _, facet := range facets {
					facet = strings.TrimSpace(facet)
					if facet != "" {
						groups = append(groups, *datadogV2.NewRumMetricGroupBy(facet))
					}
				}
				attrs.SetGroupBy(groups)
			}

			data := datadogV2.NewRumMetricCreateData(*attrs, metricID, datadogV2.RUMMETRICTYPE_RUM_METRICS)
			req := datadogV2.NewRumMetricCreateRequest(*data)

			mapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := mapi.api.CreateRumMetric(mapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create rum metric: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			m := resp.GetData()
			attrs2 := m.GetAttributes()
			compute2 := attrs2.GetCompute()
			computeStr := string(compute2.GetAggregationType())
			if compute2.Path != nil {
				computeStr += ":" + *compute2.Path
			}
			createFilter := attrs2.GetFilter()
			rows := [][]string{
				{"ID", m.GetId()},
				{"EVENT TYPE", string(attrs2.GetEventType())},
				{"COMPUTE", computeStr},
				{"FILTER", createFilter.GetQuery()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&metricID, "id", "", "metric ID (required)")
	cmd.Flags().StringVar(&compute, "compute", "", "compute spec: <aggregation>[:<path>] (required)")
	cmd.Flags().StringVar(&eventType, "event-type", "view", "RUM event type: session, view, action, error, resource, long_task, vital")
	cmd.Flags().StringVar(&filter, "filter", "", "filter query")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "comma-separated paths to group by")
	return cmd
}

func newRUMMetricUpdateCmd(mkAPI func() (*rumMetricsAPI, error)) *cobra.Command {
	var (
		filter  string
		groupBy string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a RUM-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			if !cmd.Flags().Changed("filter") && !cmd.Flags().Changed("group-by") {
				return fmt.Errorf("at least one of --filter or --group-by must be specified")
			}

			updateAttrs := datadogV2.NewRumMetricUpdateAttributes()
			if cmd.Flags().Changed("filter") {
				f := datadogV2.NewRumMetricFilter(filter)
				updateAttrs.SetFilter(*f)
			}
			if cmd.Flags().Changed("group-by") {
				facets := strings.Split(groupBy, ",")
				groups := make([]datadogV2.RumMetricGroupBy, 0, len(facets))
				for _, facet := range facets {
					facet = strings.TrimSpace(facet)
					if facet != "" {
						groups = append(groups, *datadogV2.NewRumMetricGroupBy(facet))
					}
				}
				updateAttrs.SetGroupBy(groups)
			}

			data := datadogV2.NewRumMetricUpdateData(*updateAttrs, datadogV2.RUMMETRICTYPE_RUM_METRICS)
			req := datadogV2.NewRumMetricUpdateRequest(*data)

			mapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := mapi.api.UpdateRumMetric(mapi.ctx, id, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update rum metric: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			m := resp.GetData()
			attrs := m.GetAttributes()
			compute := attrs.GetCompute()
			computeStr := string(compute.GetAggregationType())
			if compute.Path != nil {
				computeStr += ":" + *compute.Path
			}
			updateFilter := attrs.GetFilter()
			rows := [][]string{
				{"ID", m.GetId()},
				{"EVENT TYPE", string(attrs.GetEventType())},
				{"COMPUTE", computeStr},
				{"FILTER", updateFilter.GetQuery()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "new filter query")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "new comma-separated paths to group by")
	return cmd
}

func newRUMMetricDeleteCmd(mkAPI func() (*rumMetricsAPI, error)) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a RUM-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("requires --yes flag to confirm deletion")
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := mapi.api.DeleteRumMetric(mapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete rum metric: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newRUMRetentionFilterCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retention-filter",
		Short: "Manage RUM retention filters",
	}
	cmd.AddCommand(newRUMRetentionFilterListCmd(mkAPI))
	cmd.AddCommand(newRUMRetentionFilterShowCmd(mkAPI))
	cmd.AddCommand(newRUMRetentionFilterCreateCmd(mkAPI))
	cmd.AddCommand(newRUMRetentionFilterUpdateCmd(mkAPI))
	cmd.AddCommand(newRUMRetentionFilterDeleteCmd(mkAPI))
	return cmd
}

func newRUMRetentionFilterListCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) *cobra.Command {
	var appID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List RUM retention filters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.ListRetentionFilters(rapi.ctx, appID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list retention filters: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.RumRetentionFilterData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "EVENT TYPE", "SAMPLE RATE", "ENABLED"}
			var rows [][]string
			for _, f := range resp.GetData() {
				attrs := f.GetAttributes()
				enabled := fmt.Sprintf("%v", attrs.GetEnabled())
				rows = append(rows, []string{
					f.GetId(),
					attrs.GetName(),
					string(attrs.GetEventType()),
					fmt.Sprintf("%g", attrs.GetSampleRate()),
					enabled,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "RUM application ID (required)")
	return cmd
}

func newRUMRetentionFilterShowCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) *cobra.Command {
	var appID string
	cmd := &cobra.Command{
		Use:   "show <filter-id>",
		Short: "Show a RUM retention filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.GetRetentionFilter(rapi.ctx, appID, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get retention filter: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			rf := resp.GetData()
			attrs := rf.GetAttributes()
			rows := [][]string{
				{"ID", rf.GetId()},
				{"NAME", attrs.GetName()},
				{"EVENT TYPE", string(attrs.GetEventType())},
				{"SAMPLE RATE", fmt.Sprintf("%g", attrs.GetSampleRate())},
				{"ENABLED", fmt.Sprintf("%v", attrs.GetEnabled())},
				{"QUERY", attrs.GetQuery()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "RUM application ID (required)")
	return cmd
}

func newRUMRetentionFilterCreateCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) *cobra.Command {
	var (
		appID      string
		name       string
		eventType  string
		sampleRate float64
		query      string
		enabled    bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a RUM retention filter",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if sampleRate < 0.1 || sampleRate > 100 {
				return fmt.Errorf("--sample-rate must be between 0.1 and 100")
			}

			et, err := datadogV2.NewRumRetentionFilterEventTypeFromValue(eventType)
			if err != nil {
				return fmt.Errorf("--event-type: %w", err)
			}

			attrs := datadogV2.NewRumRetentionFilterCreateAttributes(*et, name, sampleRate)
			if query != "" {
				attrs.SetQuery(query)
			}
			if cmd.Flags().Changed("enabled") {
				attrs.SetEnabled(enabled)
			}

			data := datadogV2.NewRumRetentionFilterCreateData(*attrs, datadogV2.RUMRETENTIONFILTERTYPE_RETENTION_FILTERS)
			req := datadogV2.NewRumRetentionFilterCreateRequest(*data)

			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.CreateRetentionFilter(rapi.ctx, appID, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create retention filter: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			rf := resp.GetData()
			rfAttrs := rf.GetAttributes()
			rows := [][]string{
				{"ID", rf.GetId()},
				{"NAME", rfAttrs.GetName()},
				{"EVENT TYPE", string(rfAttrs.GetEventType())},
				{"SAMPLE RATE", fmt.Sprintf("%g", rfAttrs.GetSampleRate())},
				{"ENABLED", fmt.Sprintf("%v", rfAttrs.GetEnabled())},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "RUM application ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "filter name (required)")
	cmd.Flags().StringVar(&eventType, "event-type", "view", "RUM event type: session, view, action, error, resource, long_task, vital")
	cmd.Flags().Float64Var(&sampleRate, "sample-rate", 100, "sample rate (0.1-100)")
	cmd.Flags().StringVar(&query, "query", "", "filter query")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable the filter")
	return cmd
}

func newRUMRetentionFilterUpdateCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) *cobra.Command {
	var (
		appID      string
		name       string
		eventType  string
		sampleRate float64
		query      string
		enabled    bool
	)
	cmd := &cobra.Command{
		Use:   "update <filter-id>",
		Short: "Update a RUM retention filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("event-type") &&
				!cmd.Flags().Changed("sample-rate") && !cmd.Flags().Changed("query") &&
				!cmd.Flags().Changed("enabled") {
				return fmt.Errorf("at least one of --name, --event-type, --sample-rate, --query, or --enabled must be specified")
			}
			rfID := args[0]

			attrs := datadogV2.NewRumRetentionFilterUpdateAttributes()
			if cmd.Flags().Changed("name") {
				attrs.SetName(name)
			}
			if eventType != "" {
				et, err := datadogV2.NewRumRetentionFilterEventTypeFromValue(eventType)
				if err != nil {
					return fmt.Errorf("--event-type: %w", err)
				}
				attrs.SetEventType(*et)
			}
			if cmd.Flags().Changed("sample-rate") {
				if sampleRate < 0.1 || sampleRate > 100 {
					return fmt.Errorf("--sample-rate must be between 0.1 and 100")
				}
				attrs.SetSampleRate(sampleRate)
			}
			if cmd.Flags().Changed("query") {
				attrs.SetQuery(query)
			}
			if cmd.Flags().Changed("enabled") {
				attrs.SetEnabled(enabled)
			}

			data := datadogV2.NewRumRetentionFilterUpdateData(*attrs, rfID, datadogV2.RUMRETENTIONFILTERTYPE_RETENTION_FILTERS)
			req := datadogV2.NewRumRetentionFilterUpdateRequest(*data)

			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.UpdateRetentionFilter(rapi.ctx, appID, rfID, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update retention filter: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			rf := resp.GetData()
			rfAttrs := rf.GetAttributes()
			rows := [][]string{
				{"ID", rf.GetId()},
				{"NAME", rfAttrs.GetName()},
				{"EVENT TYPE", string(rfAttrs.GetEventType())},
				{"SAMPLE RATE", fmt.Sprintf("%g", rfAttrs.GetSampleRate())},
				{"ENABLED", fmt.Sprintf("%v", rfAttrs.GetEnabled())},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "RUM application ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "new filter name")
	cmd.Flags().StringVar(&eventType, "event-type", "", "new RUM event type")
	cmd.Flags().Float64Var(&sampleRate, "sample-rate", 0, "new sample rate (0.1-100)")
	cmd.Flags().StringVar(&query, "query", "", "new filter query")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable or disable the filter")
	return cmd
}

func newRUMRetentionFilterDeleteCmd(mkAPI func() (*rumRetentionFiltersAPI, error)) *cobra.Command {
	var (
		appID string
		yes   bool
	)
	cmd := &cobra.Command{
		Use:   "delete <filter-id>",
		Short: "Delete a RUM retention filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			if !yes {
				return fmt.Errorf("requires --yes flag to confirm deletion")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := rapi.api.DeleteRetentionFilter(rapi.ctx, appID, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete retention filter: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return nil
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "RUM application ID (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newRUMPlaylistCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playlist",
		Short: "Manage RUM replay playlists",
	}
	cmd.AddCommand(newRUMPlaylistListCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistShowCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistCreateCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistUpdateCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistDeleteCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistSessionsCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistAddSessionCmd(mkAPI))
	cmd.AddCommand(newRUMPlaylistRemoveSessionCmd(mkAPI))
	return cmd
}

func newRUMPlaylistListCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List RUM replay playlists",
		RunE: func(cmd *cobra.Command, _ []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := papi.api.ListRumReplayPlaylists(papi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list playlists: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.PlaylistData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "SESSIONS", "CREATED"}
			var rows [][]string
			for _, p := range resp.GetData() {
				id := ""
				if p.Id != nil {
					id = *p.Id
				}
				attrs := p.GetAttributes()
				created := ""
				if t := attrs.CreatedAt; t != nil {
					created = t.UTC().Format(time.RFC3339)
				}
				rows = append(rows, []string{
					id,
					attrs.GetName(),
					fmt.Sprintf("%d", attrs.GetSessionCount()),
					created,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRUMPlaylistShowCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a RUM replay playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePlaylistID(args[0])
			if err != nil {
				return err
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := papi.api.GetRumReplayPlaylist(papi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get playlist: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			data := resp.GetData()
			attrs := data.GetAttributes()
			created := ""
			if t := attrs.CreatedAt; t != nil {
				created = t.UTC().Format(time.RFC3339)
			}
			pid := ""
			if data.Id != nil {
				pid = *data.Id
			}
			rows := [][]string{
				{"ID", pid},
				{"NAME", attrs.GetName()},
				{"DESCRIPTION", attrs.GetDescription()},
				{"SESSIONS", fmt.Sprintf("%d", attrs.GetSessionCount())},
				{"CREATED", created},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
}

func newRUMPlaylistCreateCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	var (
		name        string
		description string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a RUM replay playlist",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			attrs := datadogV2.NewPlaylistDataAttributes(name)
			if description != "" {
				attrs.SetDescription(description)
			}
			data := datadogV2.NewPlaylistData(datadogV2.PLAYLISTDATATYPE_RUM_REPLAY_PLAYLIST)
			data.SetAttributes(*attrs)
			body := datadogV2.NewPlaylist(*data)

			papi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := papi.api.CreateRumReplayPlaylist(papi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create playlist: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			d := resp.GetData()
			a := d.GetAttributes()
			pid := ""
			if d.Id != nil {
				pid = *d.Id
			}
			rows := [][]string{
				{"ID", pid},
				{"NAME", a.GetName()},
				{"DESCRIPTION", a.GetDescription()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "playlist name (required)")
	cmd.Flags().StringVar(&description, "description", "", "playlist description")
	return cmd
}

func newRUMPlaylistUpdateCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	var (
		name        string
		description string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a RUM replay playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("description") {
				return fmt.Errorf("at least one of --name or --description must be specified")
			}

			pid, err := parsePlaylistID(args[0])
			if err != nil {
				return err
			}

			// fetch current to merge
			papi, err := mkAPI()
			if err != nil {
				return err
			}
			current, httpResp, err := papi.api.GetRumReplayPlaylist(papi.ctx, pid)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get playlist: %w", err)
			}

			currentData := current.GetData()
			currentAttrs := currentData.GetAttributes()
			updatedName := currentAttrs.GetName()
			if name != "" {
				updatedName = name
			}
			attrs := datadogV2.NewPlaylistDataAttributes(updatedName)
			if cmd.Flags().Changed("description") {
				attrs.SetDescription(description)
			} else if currentAttrs.Description != nil {
				attrs.SetDescription(*currentAttrs.Description)
			}

			data := datadogV2.NewPlaylistData(datadogV2.PLAYLISTDATATYPE_RUM_REPLAY_PLAYLIST)
			data.SetAttributes(*attrs)
			body := datadogV2.NewPlaylist(*data)

			resp, httpResp2, err := papi.api.UpdateRumReplayPlaylist(papi.ctx, pid, *body)
			if httpResp2 != nil {
				_ = httpResp2.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update playlist: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			d := resp.GetData()
			a := d.GetAttributes()
			idStr := ""
			if d.Id != nil {
				idStr = *d.Id
			}
			rows := [][]string{
				{"ID", idStr},
				{"NAME", a.GetName()},
				{"DESCRIPTION", a.GetDescription()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new playlist name")
	cmd.Flags().StringVar(&description, "description", "", "new playlist description")
	return cmd
}

func newRUMPlaylistDeleteCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a RUM replay playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("requires --yes flag to confirm deletion")
			}
			pid, err := parsePlaylistID(args[0])
			if err != nil {
				return err
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := papi.api.DeleteRumReplayPlaylist(papi.ctx, pid)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete playlist: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newRUMPlaylistSessionsCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions <id>",
		Short: "List sessions in a RUM replay playlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := parsePlaylistID(args[0])
			if err != nil {
				return err
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := papi.api.ListRumReplayPlaylistSessions(papi.ctx, pid)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list playlist sessions: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.PlaylistsSessionData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"SESSION ID"}
			var rows [][]string
			for _, s := range resp.GetData() {
				sid := ""
				if s.Id != nil {
					sid = *s.Id
				}
				rows = append(rows, []string{sid})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRUMPlaylistAddSessionCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "add-session <playlist-id> <session-id>",
		Short: "Add a session to a RUM replay playlist",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := parsePlaylistID(args[0])
			if err != nil {
				return err
			}
			sessionID := args[1]
			ts := time.Now().UnixMilli()

			papi, err := mkAPI()
			if err != nil {
				return err
			}
			_, httpResp, err := papi.api.AddRumReplaySessionToPlaylist(papi.ctx, ts, pid, sessionID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("add session to playlist: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "added")
			return nil
		},
	}
}

func newRUMPlaylistRemoveSessionCmd(mkAPI func() (*rumPlaylistsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-session <playlist-id> <session-id>",
		Short: "Remove a session from a RUM replay playlist",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := parsePlaylistID(args[0])
			if err != nil {
				return err
			}
			sessionID := args[1]

			papi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := papi.api.RemoveRumReplaySessionFromPlaylist(papi.ctx, pid, sessionID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("remove session from playlist: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removed")
			return nil
		},
	}
}

// parsePlaylistID converts a string playlist ID to int32.
func parsePlaylistID(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid playlist id %q: must be an integer", s)
	}
	return int32(n), nil //nolint:gosec
}

func newRUMHeatmapCmd(mkAPI func() (*rumHeatmapsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "heatmap",
		Short: "Manage RUM replay heatmap snapshots",
	}
	cmd.AddCommand(newRUMHeatmapListCmd(mkAPI))
	cmd.AddCommand(newRUMHeatmapCreateCmd(mkAPI))
	cmd.AddCommand(newRUMHeatmapUpdateCmd(mkAPI))
	cmd.AddCommand(newRUMHeatmapDeleteCmd(mkAPI))
	return cmd
}

func newRUMHeatmapListCmd(mkAPI func() (*rumHeatmapsAPI, error)) *cobra.Command {
	var view string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List RUM replay heatmap snapshots",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if view == "" {
				return fmt.Errorf("--view is required")
			}
			hapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := hapi.api.ListReplayHeatmapSnapshots(hapi.ctx, view)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list heatmaps: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.SnapshotData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "VIEW", "DEVICE", "CREATED AT"}
			var rows [][]string
			for _, s := range resp.GetData() {
				id := ""
				if s.Id != nil {
					id = *s.Id
				}
				a := s.GetAttributes()
				createdAt := ""
				if t := a.GetCreatedAt(); !t.IsZero() {
					createdAt = t.Format(time.RFC3339)
				}
				rows = append(rows, []string{
					id,
					a.GetViewName(),
					a.GetDeviceType(),
					createdAt,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&view, "view", "", "view name to filter by (required)")
	return cmd
}

func newRUMHeatmapCreateCmd(mkAPI func() (*rumHeatmapsAPI, error)) *cobra.Command {
	var (
		appID      string
		deviceType string
		eventID    string
		name       string
		start      int64
		viewName   string
		sessionID  string
		viewID     string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a RUM replay heatmap snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			if deviceType == "" {
				return fmt.Errorf("--device is required")
			}
			if eventID == "" {
				return fmt.Errorf("--event is required")
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if viewName == "" {
				return fmt.Errorf("--view is required")
			}
			if !cmd.Flags().Changed("start") {
				return fmt.Errorf("--start is required")
			}

			attrs := datadogV2.NewSnapshotCreateRequestDataAttributes(appID, deviceType, eventID, false, name, start, viewName)
			if sessionID != "" {
				attrs.SetSessionId(sessionID)
			}
			if viewID != "" {
				attrs.SetViewId(viewID)
			}
			data := datadogV2.NewSnapshotCreateRequestDataWithDefaults()
			data.SetAttributes(*attrs)
			body := datadogV2.NewSnapshotCreateRequest(*data)

			hapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := hapi.api.CreateReplayHeatmapSnapshot(hapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create heatmap: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			d := resp.GetData()
			a := d.GetAttributes()
			id := ""
			if d.Id != nil {
				id = *d.Id
			}
			createdAt := ""
			if t := a.GetCreatedAt(); !t.IsZero() {
				createdAt = t.Format(time.RFC3339)
			}
			rows := [][]string{
				{"ID", id},
				{"VIEW", a.GetViewName()},
				{"DEVICE", a.GetDeviceType()},
				{"CREATED AT", createdAt},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "application ID (required)")
	cmd.Flags().StringVar(&deviceType, "device", "", "device type (required)")
	cmd.Flags().StringVar(&eventID, "event", "", "event ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "snapshot name (required)")
	cmd.Flags().Int64Var(&start, "start", 0, "start timestamp in ms (required)")
	cmd.Flags().StringVar(&viewName, "view", "", "view name (required)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session ID")
	cmd.Flags().StringVar(&viewID, "view-id", "", "view ID")
	return cmd
}

func newRUMHeatmapUpdateCmd(mkAPI func() (*rumHeatmapsAPI, error)) *cobra.Command {
	var (
		eventID   string
		start     int64
		sessionID string
		viewID    string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a RUM replay heatmap snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshotID := args[0]
			if eventID == "" {
				return fmt.Errorf("--event is required")
			}
			if !cmd.Flags().Changed("start") {
				return fmt.Errorf("--start is required")
			}

			attrs := datadogV2.NewSnapshotUpdateRequestDataAttributes(eventID, false, start)
			if sessionID != "" {
				attrs.SetSessionId(sessionID)
			}
			if viewID != "" {
				attrs.SetViewId(viewID)
			}
			data := datadogV2.NewSnapshotUpdateRequestDataWithDefaults()
			data.SetId(snapshotID)
			data.SetAttributes(*attrs)
			body := datadogV2.NewSnapshotUpdateRequest(*data)

			hapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := hapi.api.UpdateReplayHeatmapSnapshot(hapi.ctx, snapshotID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update heatmap: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			d := resp.GetData()
			a := d.GetAttributes()
			id := ""
			if d.Id != nil {
				id = *d.Id
			}
			rows := [][]string{
				{"ID", id},
				{"VIEW", a.GetViewName()},
				{"DEVICE", a.GetDeviceType()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&eventID, "event", "", "event ID (required)")
	cmd.Flags().Int64Var(&start, "start", 0, "start timestamp in ms (required)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session ID")
	cmd.Flags().StringVar(&viewID, "view-id", "", "view ID")
	return cmd
}

func newRUMHeatmapDeleteCmd(mkAPI func() (*rumHeatmapsAPI, error)) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a RUM replay heatmap snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deletion")
			}
			snapshotID := args[0]

			hapi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := hapi.api.DeleteReplayHeatmapSnapshot(hapi.ctx, snapshotID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete heatmap: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newRUMSessionCmd(mkAPI func() (*rumSessionsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage RUM replay sessions",
	}
	cmd.AddCommand(newRUMSessionSegmentsCmd(mkAPI))
	cmd.AddCommand(newRUMSessionWatchersCmd(mkAPI))
	cmd.AddCommand(newRUMSessionWatchCmd(mkAPI))
	cmd.AddCommand(newRUMSessionHistoryCmd(mkAPI))
	return cmd
}

func newRUMSessionSegmentsCmd(mkAPI func() (*rumSessionsAPI, error)) *cobra.Command {
	var (
		viewID    string
		sessionID string
	)
	cmd := &cobra.Command{
		Use:   "segments",
		Short: "Get segments for a RUM replay session view",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if viewID == "" {
				return fmt.Errorf("--view is required")
			}
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := sapi.sessions.GetSegments(sapi.ctx, viewID, sessionID)
			if httpResp != nil {
				defer func() { _ = httpResp.Body.Close() }()
			}
			if err != nil {
				return fmt.Errorf("get segments: %w", err)
			}

			if _, err = io.Copy(cmd.OutOrStdout(), httpResp.Body); err != nil {
				return fmt.Errorf("stream segments: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&viewID, "view", "", "view ID (required)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session ID (required)")
	return cmd
}

func newRUMSessionWatchersCmd(mkAPI func() (*rumSessionsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "watchers <session-id>",
		Short: "List watchers of a RUM replay session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := sapi.viewership.ListRumReplaySessionWatchers(sapi.ctx, sessionID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list watchers: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.WatcherData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "HANDLE", "LAST WATCHED", "WATCH COUNT"}
			var rows [][]string
			for _, w := range resp.GetData() {
				id := ""
				if w.Id != nil {
					id = *w.Id
				}
				a := w.GetAttributes()
				lastWatched := ""
				if t := a.GetLastWatchedAt(); !t.IsZero() {
					lastWatched = t.Format(time.RFC3339)
				}
				rows = append(rows, []string{
					id,
					a.GetHandle(),
					lastWatched,
					fmt.Sprintf("%d", a.GetWatchCount()),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRUMSessionWatchCmd(mkAPI func() (*rumSessionsAPI, error)) *cobra.Command {
	var (
		appID   string
		eventID string
	)
	cmd := &cobra.Command{
		Use:   "watch <session-id>",
		Short: "Record a watch on a RUM replay session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]
			if appID == "" {
				return fmt.Errorf("--app is required")
			}
			if eventID == "" {
				return fmt.Errorf("--event is required")
			}

			attrs := datadogV2.NewWatchDataAttributes(appID, eventID, time.Now())
			data := datadogV2.NewWatchDataWithDefaults()
			data.SetAttributes(*attrs)
			body := datadogV2.NewWatch(*data)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := sapi.viewership.CreateRumReplaySessionWatch(sapi.ctx, sessionID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("watch session: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp.GetData())
			}

			d := resp.GetData()
			a := d.GetAttributes()
			id := ""
			if d.Id != nil {
				id = *d.Id
			}
			rows := [][]string{
				{"ID", id},
				{"APPLICATION", a.GetApplicationId()},
				{"EVENT", a.GetEventId()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}
	cmd.Flags().StringVar(&appID, "app", "", "application ID (required)")
	cmd.Flags().StringVar(&eventID, "event", "", "event ID (required)")
	return cmd
}

func newRUMSessionHistoryCmd(mkAPI func() (*rumSessionsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "List RUM replay session viewership history",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := sapi.viewership.ListRumReplayViewershipHistorySessions(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list history: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.ViewershipHistorySessionData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"SESSION ID", "LAST WATCHED", "TRACK"}
			var rows [][]string
			for _, s := range resp.GetData() {
				id := ""
				if s.Id != nil {
					id = *s.Id
				}
				a := s.GetAttributes()
				lastWatched := ""
				if t := a.GetLastWatchedAt(); !t.IsZero() {
					lastWatched = t.Format(time.RFC3339)
				}
				rows = append(rows, []string{
					id,
					lastWatched,
					a.GetTrack(),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRUMAudienceCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audience",
		Short: "Manage RUM audience data connections, mappings, and queries",
	}
	connections := &cobra.Command{
		Use:   "connections",
		Short: "Manage audience data connections",
	}
	connections.AddCommand(newRUMAudienceConnectionsListCmd(mkAPI))
	connections.AddCommand(newRUMAudienceConnectionsCreateCmd(mkAPI))
	connections.AddCommand(newRUMAudienceConnectionsUpdateCmd(mkAPI))
	connections.AddCommand(newRUMAudienceConnectionsDeleteCmd(mkAPI))
	cmd.AddCommand(connections)
	cmd.AddCommand(newRUMAudienceMappingCmd(mkAPI))
	cmd.AddCommand(newRUMAudienceQueryUsersCmd(mkAPI))
	cmd.AddCommand(newRUMAudienceQueryAccountsCmd(mkAPI))
	return cmd
}

func newRUMAudienceConnectionsListCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var entity string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List audience data connections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if entity == "" {
				return fmt.Errorf("--entity is required")
			}
			aapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := aapi.api.ListConnections(aapi.ctx, entity)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list connections: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			a := d.GetAttributes()
			conns := a.GetConnections()
			if asJSON {
				if conns == nil {
					conns = []datadogV2.ListConnectionsResponseDataAttributesConnectionsItems{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), conns)
			}

			headers := []string{"ID", "TYPE", "JOIN ATTRIBUTE", "CREATED AT"}
			var rows [][]string
			for _, c := range conns {
				id := c.GetId()
				typ := c.GetType()
				join := ""
				if j := c.Join; j != nil {
					join = j.GetAttribute()
				}
				createdAt := ""
				if c.CreatedAt != nil {
					createdAt = c.CreatedAt.Format(time.RFC3339)
				}
				rows = append(rows, []string{id, typ, join, createdAt})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "", "entity name (required)")
	return cmd
}

func newRUMAudienceConnectionsCreateCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var (
		entity        string
		joinAttribute string
		joinType      string
		connType      string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an audience data connection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if entity == "" {
				return fmt.Errorf("--entity is required")
			}
			if joinAttribute == "" || joinType == "" || connType == "" {
				return fmt.Errorf("--join-attribute, --join-type, and --type are required")
			}
			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewCreateConnectionRequestDataAttributes(joinAttribute, joinType, connType)
			data := datadogV2.NewCreateConnectionRequestData(datadogV2.UPDATECONNECTIONREQUESTDATATYPE_CONNECTION_ID)
			data.SetAttributes(*attrs)
			body := datadogV2.NewCreateConnectionRequest()
			body.SetData(*data)

			httpResp, err := aapi.api.CreateConnection(aapi.ctx, entity, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create connection: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "connection created")
			return err
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "", "entity name (required)")
	cmd.Flags().StringVar(&joinAttribute, "join-attribute", "", "join attribute (required)")
	cmd.Flags().StringVar(&joinType, "join-type", "", "join type (required)")
	cmd.Flags().StringVar(&connType, "type", "", "connection type (required)")
	return cmd
}

func newRUMAudienceConnectionsUpdateCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var (
		entity         string
		fieldsToDelete string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an audience data connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connectionID := args[0]
			if entity == "" {
				return fmt.Errorf("--entity is required")
			}
			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewUpdateConnectionRequestDataAttributes()
			if fieldsToDelete != "" {
				var names []string
				for _, s := range strings.Split(fieldsToDelete, ",") {
					if s = strings.TrimSpace(s); s != "" {
						names = append(names, s)
					}
				}
				attrs.SetFieldsToDelete(names)
			}
			data := datadogV2.NewUpdateConnectionRequestData(connectionID, datadogV2.UPDATECONNECTIONREQUESTDATATYPE_CONNECTION_ID)
			data.SetAttributes(*attrs)
			body := datadogV2.NewUpdateConnectionRequest()
			body.SetData(*data)

			httpResp, err := aapi.api.UpdateConnection(aapi.ctx, entity, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update connection: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "connection updated")
			return err
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "", "entity name (required)")
	cmd.Flags().StringVar(&fieldsToDelete, "fields-to-delete", "", "comma-separated field names to delete")
	return cmd
}

func newRUMAudienceConnectionsDeleteCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var (
		entity string
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an audience data connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion")
			}
			if entity == "" {
				return fmt.Errorf("--entity is required")
			}
			aapi, err := mkAPI()
			if err != nil {
				return err
			}
			httpResp, err := aapi.api.DeleteConnection(aapi.ctx, args[0], entity)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete connection: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "connection deleted")
			return err
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "", "entity name (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newRUMAudienceMappingCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var entity string
	cmd := &cobra.Command{
		Use:   "mapping",
		Short: "Get entity mapping configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if entity == "" {
				return fmt.Errorf("--entity is required")
			}
			aapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := aapi.api.GetMapping(aapi.ctx, entity)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get mapping: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			md := resp.GetData()
			ma := md.GetAttributes()
			attrs := ma.GetAttributes()
			if asJSON {
				if attrs == nil {
					attrs = []datadogV2.GetMappingResponseDataAttributesAttributesItems{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), attrs)
			}

			headers := []string{"ATTRIBUTE", "DISPLAY NAME", "TYPE", "CUSTOM"}
			var rows [][]string
			for _, a := range attrs {
				custom := "false"
				if v := a.IsCustom; v != nil && *v {
					custom = "true"
				}
				rows = append(rows, []string{
					a.GetAttribute(),
					a.GetDisplayName(),
					a.GetType(),
					custom,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "", "entity name (required)")
	return cmd
}

func newRUMAudienceQueryUsersCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var (
		query string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "query-users",
		Short: "Query RUM audience users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewQueryUsersRequestDataAttributes()
			if query != "" {
				attrs.SetQuery(query)
			}
			if limit > 0 {
				l := int64(limit) //nolint:gosec
				attrs.SetLimit(l)
			}
			data := datadogV2.NewQueryUsersRequestData(datadogV2.QUERYUSERSREQUESTDATATYPE_QUERY_USERS_REQUEST)
			data.SetAttributes(*attrs)
			body := datadogV2.NewQueryUsersRequest()
			body.SetData(*data)

			resp, httpResp, err := aapi.api.QueryUsers(aapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("query users: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			ud := resp.GetData()
			ua := ud.GetAttributes()
			hits := ua.GetHits()
			if asJSON {
				if hits == nil {
					hits = []map[string]interface{}{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), hits)
			}

			total := ua.GetTotal()
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "total: %d\n", total)
			if err != nil {
				return err
			}
			for _, h := range hits {
				b, merr := json.Marshal(h)
				if merr != nil {
					return merr
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "filter query")
	cmd.Flags().IntVar(&limit, "limit", 0, "max results")
	return cmd
}

func newRUMAudienceQueryAccountsCmd(mkAPI func() (*rumAudienceAPI, error)) *cobra.Command {
	var (
		query string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "query-accounts",
		Short: "Query RUM audience accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewQueryAccountRequestDataAttributes()
			if query != "" {
				attrs.SetQuery(query)
			}
			if limit > 0 {
				l := int64(limit) //nolint:gosec
				attrs.SetLimit(l)
			}
			data := datadogV2.NewQueryAccountRequestData(datadogV2.QUERYACCOUNTREQUESTDATATYPE_QUERY_ACCOUNT_REQUEST)
			data.SetAttributes(*attrs)
			body := datadogV2.NewQueryAccountRequest()
			body.SetData(*data)

			resp, httpResp, err := aapi.api.QueryAccounts(aapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("query accounts: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			ad := resp.GetData()
			aa := ad.GetAttributes()
			hits := aa.GetHits()
			if asJSON {
				if hits == nil {
					hits = []map[string]interface{}{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), hits)
			}

			total := aa.GetTotal()
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "total: %d\n", total)
			if err != nil {
				return err
			}
			for _, h := range hits {
				b, merr := json.Marshal(h)
				if merr != nil {
					return merr
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", b)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "filter query")
	cmd.Flags().IntVar(&limit, "limit", 0, "max results")
	return cmd
}

// parseRUMMetricComputeSpec parses "count" or "sum:@metric.path" into aggregation type and path.
func parseRUMMetricComputeSpec(s string) (datadogV2.RumMetricComputeAggregationType, string, error) {
	parts := strings.SplitN(s, ":", 2)
	aggType, err := datadogV2.NewRumMetricComputeAggregationTypeFromValue(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("unknown aggregation %q", parts[0])
	}
	path := ""
	if len(parts) == 2 {
		path = parts[1]
	}
	return *aggType, path, nil
}
