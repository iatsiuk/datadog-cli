package cmd

import (
	"context"
	"fmt"
	"sort"
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

// NewRUMCommand returns the rum cobra command group.
func NewRUMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rum",
		Short: "Search and manage Datadog RUM",
	}
	cmd.AddCommand(newRUMSearchCmd(defaultRUMAPI))
	cmd.AddCommand(newRUMAggregateCmd(defaultRUMAPI))
	cmd.AddCommand(newRUMAppCmd(defaultRUMAPI))
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

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			updateAttrs := datadogV2.NewRUMApplicationUpdateAttributes()
			if name != "" {
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
