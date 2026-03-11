package cmd

import (
	"context"
	"errors"
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

type spansAPI struct {
	api *datadogV2.SpansApi
	ctx context.Context
}

func defaultSpansAPI() (*spansAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &spansAPI{api: datadogV2.NewSpansApi(c), ctx: ctx}, nil
}

type apmAPI struct {
	api *datadogV2.APMApi
	ctx context.Context
}

func defaultAPMAPI() (*apmAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &apmAPI{api: datadogV2.NewAPMApi(c), ctx: ctx}, nil
}

type retentionFiltersAPI struct {
	api *datadogV2.APMRetentionFiltersApi
	ctx context.Context
}

func defaultRetentionFiltersAPI() (*retentionFiltersAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &retentionFiltersAPI{api: datadogV2.NewAPMRetentionFiltersApi(c), ctx: ctx}, nil
}

type spansMetricsAPI struct {
	api *datadogV2.SpansMetricsApi
	ctx context.Context
}

func defaultSpansMetricsAPI() (*spansMetricsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &spansMetricsAPI{api: datadogV2.NewSpansMetricsApi(c), ctx: ctx}, nil
}

// NewAPMCommand returns the apm cobra command group.
func NewAPMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apm",
		Short: "Search and manage Datadog APM",
	}
	cmd.AddCommand(newAPMSearchCmd(defaultSpansAPI))
	cmd.AddCommand(newAPMTailCmd(defaultSpansAPI))
	cmd.AddCommand(newAPMAggregateCmd(defaultSpansAPI))
	cmd.AddCommand(newAPMServicesCmd(defaultAPMAPI))
	cmd.AddCommand(newAPMRetentionFilterCmd(defaultRetentionFiltersAPI))
	cmd.AddCommand(newAPMSpanMetricCmd(defaultSpansMetricsAPI))
	return cmd
}

func newAPMSearchCmd(mkAPI func() (*spansAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
		sortStr string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search spans",
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

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}
			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListSpansGetOptionalParameters().
				WithFilterFrom(fromTime.UTC().Format(time.RFC3339)).
				WithFilterTo(toTime.UTC().Format(time.RFC3339)).
				WithPageLimit(pageLimit)
			if query != "" {
				opts = opts.WithFilterQuery(query)
			}
			if sortStr != "" {
				s, err := datadogV2.NewSpansSortFromValue(sortStr)
				if err != nil {
					return fmt.Errorf("--sort: %w", err)
				}
				opts = opts.WithSort(*s)
			}

			resp, httpResp, err := sapi.api.ListSpansGet(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search spans: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.Span{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "SERVICE", "RESOURCE", "DURATION", "STATUS"}
			var rows [][]string
			for _, span := range resp.GetData() {
				attrs := span.GetAttributes()
				ts := ""
				if t := attrs.StartTimestamp; t != nil {
					ts = t.UTC().Format(time.RFC3339)
				}
				duration := ""
				if attrs.StartTimestamp != nil && attrs.EndTimestamp != nil {
					d := attrs.EndTimestamp.Sub(*attrs.StartTimestamp)
					duration = d.String()
				}
				status := "ok"
				if errVal, ok := attrs.GetAttributes()["error"]; ok {
					if errStr, ok2 := errVal.(string); ok2 && errStr == "1" {
						status = "error"
					} else if errNum, ok3 := errVal.(float64); ok3 && errNum != 0 {
						status = "error"
					}
				}
				rows = append(rows, []string{
					ts,
					attrs.GetService(),
					attrs.GetResourceName(),
					duration,
					status,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "span search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-15m (default: now-15m)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of spans to return")
	cmd.Flags().StringVar(&sortStr, "sort", "", "sort order: timestamp or -timestamp")
	return cmd
}

const tailPollInterval = 5 * time.Second

func newAPMTailCmd(mkAPI func() (*spansAPI, error)) *cobra.Command {
	var (
		query   string
		service string
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail spans in real time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			// build effective query
			effectiveQuery := query
			if service != "" {
				q := "service:" + service
				if effectiveQuery != "" {
					q = effectiveQuery + " " + q
				}
				effectiveQuery = q
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			// overlap window to catch late-arriving spans due to ingestion delay
			const ingestionOverlap = 10 * time.Second
			since := time.Now()
			var prevSeen map[string]struct{}
			currSeen := map[string]struct{}{}

			for {
				to := time.Now()
				baseOpts := datadogV2.NewListSpansGetOptionalParameters().
					WithFilterFrom(since.UTC().Format(time.RFC3339)).
					WithFilterTo(to.UTC().Format(time.RFC3339)).
					WithPageLimit(100)
				if effectiveQuery != "" {
					baseOpts = baseOpts.WithFilterQuery(effectiveQuery)
				}

				nextSeen := map[string]struct{}{}
				opts := *baseOpts
				apiErr := func() error {
					for {
						resp, httpResp, innerErr := sapi.api.ListSpansGet(sapi.ctx, opts)
						if httpResp != nil {
							_ = httpResp.Body.Close()
						}
						if innerErr != nil {
							return innerErr
						}
						for _, span := range resp.GetData() {
							id := span.GetId()
							if id == "" {
								continue
							}
							nextSeen[id] = struct{}{}
							if _, inPrev := prevSeen[id]; inPrev {
								continue
							}
							if _, inCurr := currSeen[id]; inCurr {
								continue
							}
							if asJSON {
								_ = output.PrintJSON(cmd.OutOrStdout(), span)
							} else {
								attrs := span.GetAttributes()
								ts := ""
								if t := attrs.StartTimestamp; t != nil {
									ts = t.UTC().Format(time.RFC3339)
								}
								duration := ""
								if attrs.StartTimestamp != nil && attrs.EndTimestamp != nil {
									d := attrs.EndTimestamp.Sub(*attrs.StartTimestamp)
									duration = d.String()
								}
								status := "ok"
								if errVal, ok := attrs.GetAttributes()["error"]; ok {
									if errStr, ok2 := errVal.(string); ok2 && errStr == "1" {
										status = "error"
									} else if errNum, ok3 := errVal.(float64); ok3 && errNum != 0 {
										status = "error"
									}
								}
								_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n",
									ts, attrs.GetService(), attrs.GetResourceName(), duration, status)
							}
						}
						cursor := ""
						if resp.Meta != nil && resp.Meta.Page != nil {
							cursor = resp.Meta.Page.GetAfter()
						}
						if cursor == "" {
							break
						}
						opts = *baseOpts
						opts.PageCursor = &cursor
					}
					return nil
				}()

				if apiErr != nil {
					if errors.Is(apiErr, context.Canceled) || errors.Is(apiErr, context.DeadlineExceeded) {
						return nil
					}
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", apiErr)
					prevSeen = currSeen
					currSeen = nextSeen
				} else {
					prevSeen = currSeen
					currSeen = nextSeen
					since = to.Add(-ingestionOverlap)
				}

				select {
				case <-sapi.ctx.Done():
					return nil
				case <-time.After(tailPollInterval):
				}
			}
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "span search query")
	cmd.Flags().StringVar(&service, "service", "", "filter by service name")
	return cmd
}

func newAPMAggregateCmd(mkAPI func() (*spansAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		groupBy []string
		compute string
	)

	cmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Aggregate spans",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if compute == "" {
				return fmt.Errorf("--compute is required")
			}
			aggFn, metric, err := parseSpanComputeSpec(compute)
			if err != nil {
				return fmt.Errorf("--compute: %w", err)
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			if fromStr == "" {
				fromStr = "now-15m"
			}
			if toStr == "" {
				toStr = "now"
			}

			filter := datadogV2.NewSpansQueryFilter()
			filter.SetFrom(fromStr)
			filter.SetTo(toStr)
			if query != "" {
				filter.SetQuery(query)
			}

			c := datadogV2.NewSpansCompute(aggFn)
			if metric != "" {
				c.SetMetric(metric)
			}
			computes := []datadogV2.SpansCompute{*c}

			var groups []datadogV2.SpansGroupBy
			for _, facet := range groupBy {
				groups = append(groups, *datadogV2.NewSpansGroupBy(facet))
			}

			attrs := datadogV2.NewSpansAggregateRequestAttributes()
			attrs.SetFilter(*filter)
			attrs.SetCompute(computes)
			if len(groups) > 0 {
				attrs.SetGroupBy(groups)
			}

			data := datadogV2.NewSpansAggregateData()
			data.SetAttributes(*attrs)

			req := datadogV2.NewSpansAggregateRequest()
			req.SetData(*data)

			resp, httpResp, err := sapi.api.AggregateSpans(sapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("aggregate spans: %w", err)
			}

			// collect all group-by and compute keys from first bucket to build headers
			buckets := resp.GetData()

			// determine group-by header names from --group-by flags
			groupHeaders := groupBy
			// determine compute key names from response
			var computeKeys []string
			if len(buckets) > 0 {
				attrs := buckets[0].GetAttributes()
				for k := range attrs.GetComputes() {
					computeKeys = append(computeKeys, k)
				}
				sort.Strings(computeKeys)
			}

			headers := append(groupHeaders, computeKeys...)
			if len(headers) == 0 {
				headers = []string{"BY", "COMPUTE"}
			}

			var rows [][]string
			for _, bucket := range buckets {
				bAttrs := bucket.GetAttributes()
				by := bAttrs.GetBy()
				computes2 := bAttrs.GetComputes()

				row := make([]string, 0, len(headers))
				for _, g := range groupHeaders {
					v := ""
					if val, ok := by[g]; ok {
						v = fmt.Sprintf("%v", val)
					}
					row = append(row, v)
				}
				for _, k := range computeKeys {
					v := ""
					if bv, ok := computes2[k]; ok {
						switch {
						case bv.SpansAggregateBucketValueSingleNumber != nil:
							v = fmt.Sprintf("%g", *bv.SpansAggregateBucketValueSingleNumber)
						case bv.SpansAggregateBucketValueSingleString != nil:
							v = *bv.SpansAggregateBucketValueSingleString
						case bv.SpansAggregateBucketValueTimeseries != nil:
							v = fmt.Sprintf("%d points", len(bv.SpansAggregateBucketValueTimeseries.Items))
						}
					}
					row = append(row, v)
				}
				rows = append(rows, row)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				bucketData := resp.GetData()
				if bucketData == nil {
					bucketData = []datadogV2.SpansAggregateBucket{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), bucketData)
			}

			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "span search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, supports date math (default now-15m)")
	cmd.Flags().StringVar(&toStr, "to", "", "end time, supports date math (default now)")
	cmd.Flags().StringSliceVar(&groupBy, "group-by", nil, "facets to group by (repeatable)")
	cmd.Flags().StringVar(&compute, "compute", "", "aggregation function: count, sum, avg, min, max, etc.")
	return cmd
}

func newAPMServicesCmd(mkAPI func() (*apmAPI, error)) *cobra.Command {
	var env string

	cmd := &cobra.Command{
		Use:   "services",
		Short: "List APM services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if env == "" {
				return fmt.Errorf("--env is required")
			}

			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := aapi.api.GetServiceList(aapi.ctx, env)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get service list: %w", err)
			}

			data := resp.GetData()
			attrs := data.GetAttributes()
			services := attrs.GetServices()
			headers := []string{"SERVICE"}
			rows := make([][]string, len(services))
			for i, svc := range services {
				rows[i] = []string{svc}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&env, "env", "", "environment name (required)")
	return cmd
}

func newAPMRetentionFilterCmd(mkAPI func() (*retentionFiltersAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retention-filter",
		Short: "Manage APM retention filters",
	}
	cmd.AddCommand(newRetentionFilterListCmd(mkAPI))
	cmd.AddCommand(newRetentionFilterShowCmd(mkAPI))
	cmd.AddCommand(newRetentionFilterCreateCmd(mkAPI))
	cmd.AddCommand(newRetentionFilterUpdateCmd(mkAPI))
	cmd.AddCommand(newRetentionFilterDeleteCmd(mkAPI))
	return cmd
}

func newRetentionFilterListCmd(mkAPI func() (*retentionFiltersAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List retention filters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.ListApmRetentionFilters(rapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list retention filters: %w", err)
			}
			headers := []string{"ID", "NAME", "FILTER", "RATE", "ENABLED"}
			var rows [][]string
			for _, f := range resp.GetData() {
				attrs := f.GetAttributes()
				filterQuery := ""
				if fl := attrs.Filter; fl != nil {
					filterQuery = fl.GetQuery()
				}
				rate := ""
				if r := attrs.Rate; r != nil {
					rate = fmt.Sprintf("%g", *r)
				}
				enabled := ""
				if e := attrs.Enabled; e != nil {
					enabled = fmt.Sprintf("%v", *e)
				}
				rows = append(rows, []string{
					f.GetId(),
					attrs.GetName(),
					filterQuery,
					rate,
					enabled,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRetentionFilterShowCmd(mkAPI func() (*retentionFiltersAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a retention filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := rapi.api.GetApmRetentionFilter(rapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get retention filter: %w", err)
			}
			f := resp.GetData()
			attrs := f.GetAttributes()
			filterQuery := ""
			if fl := attrs.Filter; fl != nil {
				filterQuery = fl.GetQuery()
			}
			rate := ""
			if r := attrs.Rate; r != nil {
				rate = fmt.Sprintf("%g", *r)
			}
			enabled := ""
			if e := attrs.Enabled; e != nil {
				enabled = fmt.Sprintf("%v", *e)
			}
			headers := []string{"ID", "NAME", "FILTER", "RATE", "ENABLED"}
			rows := [][]string{{f.GetId(), attrs.GetName(), filterQuery, rate, enabled}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newRetentionFilterCreateCmd(mkAPI func() (*retentionFiltersAPI, error)) *cobra.Command {
	var (
		name       string
		filterExpr string
		rate       float64
		enabled    bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a retention filter",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if filterExpr == "" {
				return fmt.Errorf("--filter is required")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			spansFilter := datadogV2.NewSpansFilterCreate(filterExpr)
			attrs := datadogV2.NewRetentionFilterCreateAttributes(
				enabled,
				*spansFilter,
				datadogV2.RETENTIONFILTERTYPE_SPANS_SAMPLING_PROCESSOR,
				name,
				rate,
			)
			data := datadogV2.NewRetentionFilterCreateData(
				*attrs,
				datadogV2.APMRETENTIONFILTERTYPE_apm_retention_filter,
			)
			req := datadogV2.NewRetentionFilterCreateRequest(*data)

			resp, httpResp, err := rapi.api.CreateApmRetentionFilter(rapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create retention filter: %w", err)
			}

			f := resp.GetData()
			rattrs := f.GetAttributes()
			filterQuery := ""
			if fl := rattrs.Filter; fl != nil {
				filterQuery = fl.GetQuery()
			}
			rrate := ""
			if r := rattrs.Rate; r != nil {
				rrate = fmt.Sprintf("%g", *r)
			}
			renabled := ""
			if e := rattrs.Enabled; e != nil {
				renabled = fmt.Sprintf("%v", *e)
			}
			headers := []string{"ID", "NAME", "FILTER", "RATE", "ENABLED"}
			rows := [][]string{{f.GetId(), rattrs.GetName(), filterQuery, rrate, renabled}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "filter name (required)")
	cmd.Flags().StringVar(&filterExpr, "filter", "", "span search query (required)")
	cmd.Flags().Float64Var(&rate, "rate", 1.0, "sample rate (0.0-1.0)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "whether filter is enabled")
	return cmd
}

func newRetentionFilterUpdateCmd(mkAPI func() (*retentionFiltersAPI, error)) *cobra.Command {
	var (
		name       string
		filterExpr string
		rate       float64
		enabled    bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a retention filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if filterExpr == "" {
				return fmt.Errorf("--filter is required")
			}
			if !cmd.Flags().Changed("rate") {
				return fmt.Errorf("--rate is required")
			}
			if !cmd.Flags().Changed("enabled") {
				return fmt.Errorf("--enabled is required")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			spansFilter := datadogV2.NewSpansFilterCreate(filterExpr)
			attrs := datadogV2.NewRetentionFilterUpdateAttributes(
				enabled,
				*spansFilter,
				datadogV2.RETENTIONFILTERALLTYPE_SPANS_SAMPLING_PROCESSOR,
				name,
				rate,
			)
			data := datadogV2.NewRetentionFilterUpdateData(
				*attrs,
				args[0],
				datadogV2.APMRETENTIONFILTERTYPE_apm_retention_filter,
			)
			req := datadogV2.NewRetentionFilterUpdateRequest(*data)

			resp, httpResp, err := rapi.api.UpdateApmRetentionFilter(rapi.ctx, args[0], *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update retention filter: %w", err)
			}

			f := resp.GetData()
			rattrs := f.GetAttributes()
			filterQuery := ""
			if fl := rattrs.Filter; fl != nil {
				filterQuery = fl.GetQuery()
			}
			rrate := ""
			if r := rattrs.Rate; r != nil {
				rrate = fmt.Sprintf("%g", *r)
			}
			renabled := ""
			if e := rattrs.Enabled; e != nil {
				renabled = fmt.Sprintf("%v", *e)
			}
			headers := []string{"ID", "NAME", "FILTER", "RATE", "ENABLED"}
			rows := [][]string{{f.GetId(), rattrs.GetName(), filterQuery, rrate, renabled}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "filter name (required)")
	cmd.Flags().StringVar(&filterExpr, "filter", "", "span search query (required)")
	cmd.Flags().Float64Var(&rate, "rate", 1.0, "sample rate (0.0-1.0)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "whether filter is enabled")
	return cmd
}

func newRetentionFilterDeleteCmd(mkAPI func() (*retentionFiltersAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a retention filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deletion")
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := rapi.api.DeleteApmRetentionFilter(rapi.ctx, args[0])
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

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newAPMSpanMetricCmd(mkAPI func() (*spansMetricsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "span-metric",
		Short: "Manage span-based metrics",
	}
	cmd.AddCommand(newSpanMetricListCmd(mkAPI))
	cmd.AddCommand(newSpanMetricShowCmd(mkAPI))
	cmd.AddCommand(newSpanMetricCreateCmd(mkAPI))
	cmd.AddCommand(newSpanMetricUpdateCmd(mkAPI))
	cmd.AddCommand(newSpanMetricDeleteCmd(mkAPI))
	return cmd
}

func formatSpanMetricRow(d datadogV2.SpansMetricResponseData) []string {
	attrs := d.GetAttributes()
	compute := ""
	if c := attrs.Compute; c != nil {
		compute = string(c.GetAggregationType())
		if p := c.Path; p != nil {
			compute += ":" + *p
		}
	}
	filter := ""
	if f := attrs.Filter; f != nil {
		filter = f.GetQuery()
	}
	var groupBys []string
	for _, g := range attrs.GetGroupBy() {
		groupBys = append(groupBys, g.GetPath())
	}
	groupBy := strings.Join(groupBys, ",")
	return []string{d.GetId(), compute, filter, groupBy}
}

func newSpanMetricListCmd(mkAPI func() (*spansMetricsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List span-based metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			smapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := smapi.api.ListSpansMetrics(smapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list span metrics: %w", err)
			}
			headers := []string{"ID", "COMPUTE", "FILTER", "GROUP-BY"}
			var rows [][]string
			for _, d := range resp.GetData() {
				rows = append(rows, formatSpanMetricRow(d))
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newSpanMetricShowCmd(mkAPI func() (*spansMetricsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a span-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			smapi, err := mkAPI()
			if err != nil {
				return err
			}
			resp, httpResp, err := smapi.api.GetSpansMetric(smapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get span metric: %w", err)
			}
			headers := []string{"ID", "COMPUTE", "FILTER", "GROUP-BY"}
			rows := [][]string{formatSpanMetricRow(resp.GetData())}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newSpanMetricCreateCmd(mkAPI func() (*spansMetricsAPI, error)) *cobra.Command {
	var (
		id        string
		computeAg string
		computePa string
		filterQ   string
		groupBys  []string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a span-based metric",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			if computeAg == "" {
				return fmt.Errorf("--compute is required")
			}
			aggType, err := datadogV2.NewSpansMetricComputeAggregationTypeFromValue(computeAg)
			if err != nil {
				return fmt.Errorf("--compute: %w", err)
			}

			smapi, err := mkAPI()
			if err != nil {
				return err
			}

			compute := datadogV2.NewSpansMetricCompute(*aggType)
			if computePa != "" {
				compute.SetPath(computePa)
			}
			attrs := datadogV2.NewSpansMetricCreateAttributes(*compute)
			if filterQ != "" {
				f := datadogV2.NewSpansMetricFilter()
				f.SetQuery(filterQ)
				attrs.SetFilter(*f)
			}
			for _, path := range groupBys {
				attrs.GroupBy = append(attrs.GroupBy, *datadogV2.NewSpansMetricGroupBy(path))
			}
			data := datadogV2.NewSpansMetricCreateData(*attrs, id, datadogV2.SPANSMETRICTYPE_SPANS_METRICS)
			req := datadogV2.NewSpansMetricCreateRequest(*data)

			resp, httpResp, err := smapi.api.CreateSpansMetric(smapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create span metric: %w", err)
			}
			headers := []string{"ID", "COMPUTE", "FILTER", "GROUP-BY"}
			rows := [][]string{formatSpanMetricRow(resp.GetData())}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "metric id (required)")
	cmd.Flags().StringVar(&computeAg, "compute", "", "aggregation type: count or distribution (required)")
	cmd.Flags().StringVar(&computePa, "path", "", "span attribute path for distribution metric")
	cmd.Flags().StringVar(&filterQ, "filter", "", "span search query filter")
	cmd.Flags().StringSliceVar(&groupBys, "group-by", nil, "span attribute paths to group by (repeatable)")
	return cmd
}

func newSpanMetricUpdateCmd(mkAPI func() (*spansMetricsAPI, error)) *cobra.Command {
	var (
		filterQ  string
		groupBys []string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a span-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			smapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewSpansMetricUpdateAttributes()
			if cmd.Flags().Changed("filter") {
				f := datadogV2.NewSpansMetricFilter()
				f.SetQuery(filterQ)
				attrs.SetFilter(*f)
			}
			for _, path := range groupBys {
				attrs.GroupBy = append(attrs.GroupBy, *datadogV2.NewSpansMetricGroupBy(path))
			}
			data := datadogV2.NewSpansMetricUpdateData(*attrs, datadogV2.SPANSMETRICTYPE_SPANS_METRICS)
			req := datadogV2.NewSpansMetricUpdateRequest(*data)

			resp, httpResp, err := smapi.api.UpdateSpansMetric(smapi.ctx, args[0], *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update span metric: %w", err)
			}
			headers := []string{"ID", "COMPUTE", "FILTER", "GROUP-BY"}
			rows := [][]string{formatSpanMetricRow(resp.GetData())}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&filterQ, "filter", "", "span search query filter")
	cmd.Flags().StringSliceVar(&groupBys, "group-by", nil, "span attribute paths to group by (repeatable)")
	return cmd
}

func newSpanMetricDeleteCmd(mkAPI func() (*spansMetricsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a span-based metric",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deletion")
			}

			smapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := smapi.api.DeleteSpansMetric(smapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete span metric: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "deleted")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

// parseSpanComputeSpec parses "count" or "sum:@metric" into aggregation function and metric.
func parseSpanComputeSpec(s string) (datadogV2.SpansAggregationFunction, string, error) {
	parts := strings.SplitN(s, ":", 2)
	agg, err := datadogV2.NewSpansAggregationFunctionFromValue(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("unknown aggregation %q", parts[0])
	}
	metric := ""
	if len(parts) == 2 {
		metric = parts[1]
	}
	return *agg, metric, nil
}
