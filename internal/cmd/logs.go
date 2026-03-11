package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsAPI struct {
	api *datadogV2.LogsApi
	ctx context.Context
}

func defaultLogsAPI() (*logsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &logsAPI{api: datadogV2.NewLogsApi(c), ctx: ctx}, nil
}

// NewLogsCommand returns the logs cobra command group.
func NewLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Search and manage Datadog logs",
	}
	cmd.AddCommand(newLogsSearchCmd(defaultLogsAPI))
	cmd.AddCommand(newLogsTailCmd(defaultLogsAPI))
	cmd.AddCommand(newLogsAggregateCmd(defaultLogsAPI))
	cmd.AddCommand(newLogsIndexCmd(defaultLogsIndexAPI))
	cmd.AddCommand(newLogsPipelineCmd(defaultLogsPipelineAPI))
	cmd.AddCommand(newLogsArchiveCmd(defaultLogsArchiveAPI))
	return cmd
}

func newLogsSearchCmd(mkAPI func() (*logsAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search logs",
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

			lapi, err := mkAPI()
			if err != nil {
				return err
			}

			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListLogsGetOptionalParameters().
				WithFilterFrom(fromTime).
				WithFilterTo(toTime).
				WithPageLimit(pageLimit)
			if query != "" {
				opts = opts.WithFilterQuery(query)
			}

			resp, httpResp, err := lapi.api.ListLogsGet(lapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search logs: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.Log{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "SERVICE", "HOST", "STATUS", "MESSAGE"}
			var rows [][]string
			for _, log := range resp.GetData() {
				attrs := log.GetAttributes()
				ts := ""
				if t := attrs.Timestamp; t != nil {
					ts = t.UTC().Format(time.RFC3339)
				}
				rows = append(rows, []string{
					ts,
					attrs.GetService(),
					attrs.GetHost(),
					attrs.GetStatus(),
					attrs.GetMessage(),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "log search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-15m (default: now-15m)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of logs to return")
	return cmd
}

func newLogsTailCmd(mkAPI func() (*logsAPI, error)) *cobra.Command {
	var (
		query    string
		service  string
		interval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail logs in real time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			lapi, err := mkAPI()
			if err != nil {
				return err
			}

			q := query
			if service != "" {
				if q != "" {
					q = q + " service:" + service
				} else {
					q = "service:" + service
				}
			}

			from := time.Now().Add(-15 * time.Minute)
			seen := map[string]struct{}{}

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				to := time.Now()
				opts := datadogV2.NewListLogsGetOptionalParameters().
					WithFilterFrom(from).
					WithFilterTo(to).
					WithPageLimit(100)
				if q != "" {
					opts = opts.WithFilterQuery(q)
				}

				resp, httpResp, apiErr := lapi.api.ListLogsGet(lapi.ctx, *opts)
				if httpResp != nil {
					_ = httpResp.Body.Close()
				}
				if apiErr != nil {
					if lapi.ctx.Err() != nil {
						return nil
					}
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", apiErr)
				} else {
					for _, log := range resp.GetData() {
						id := log.GetId()
						if _, ok := seen[id]; ok {
							continue
						}
						seen[id] = struct{}{}
						attrs := log.GetAttributes()
						ts := ""
						if t := attrs.Timestamp; t != nil {
							ts = t.UTC().Format(time.RFC3339)
						}
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  %s\n",
							ts,
							attrs.GetService(),
							attrs.GetStatus(),
							attrs.GetMessage(),
						)
					}
					from = to
				}

				select {
				case <-lapi.ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "log filter query")
	cmd.Flags().StringVar(&service, "service", "", "filter by service name")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "polling interval")
	return cmd
}

func newLogsAggregateCmd(mkAPI func() (*logsAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		groupBy string
		compute string
	)

	cmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Aggregate logs",
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

			agg, metric, err := parseComputeSpec(compute)
			if err != nil {
				return fmt.Errorf("--compute: %w", err)
			}

			filter := datadogV2.NewLogsQueryFilter()
			filter.SetFrom(fromTime.Format(time.RFC3339))
			filter.SetTo(toTime.Format(time.RFC3339))
			if query != "" {
				filter.SetQuery(query)
			}

			c := datadogV2.NewLogsCompute(agg)
			if metric != "" {
				c.SetMetric(metric)
			}

			req := datadogV2.NewLogsAggregateRequest()
			req.SetFilter(*filter)
			req.SetCompute([]datadogV2.LogsCompute{*c})

			if groupBy != "" {
				facets := strings.Split(groupBy, ",")
				groups := make([]datadogV2.LogsGroupBy, 0, len(facets))
				for _, f := range facets {
					f = strings.TrimSpace(f)
					if f != "" {
						groups = append(groups, *datadogV2.NewLogsGroupBy(f))
					}
				}
				req.SetGroupBy(groups)
			}

			lapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := lapi.api.AggregateLogs(lapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("aggregate logs: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			data := resp.GetData()
			if asJSON {
				buckets := data.GetBuckets()
				if buckets == nil {
					buckets = []datadogV2.LogsAggregateBucket{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), buckets)
			}

			buckets := data.GetBuckets()
			if len(buckets) == 0 {
				return output.PrintTable(cmd.OutOrStdout(), nil, nil)
			}

			// build column names: group-by facets first, then compute keys
			byKeys := sortedKeys(buckets[0].GetBy())
			computeKeys := sortedKeys2(buckets[0].GetComputes())
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
					row = append(row, fmt.Sprintf("%v", b.GetBy()[k]))
				}
				for _, k := range computeKeys {
					v := b.GetComputes()[k]
					row = append(row, formatBucketValue(v))
				}
				rows = append(rows, row)
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "log filter query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time (default: now-15m)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "comma-separated facets to group by")
	cmd.Flags().StringVar(&compute, "compute", "count", "aggregation spec: <function>[:<metric>]")
	return cmd
}

// parseComputeSpec parses "count" or "sum:@metric" into aggregation function and metric.
func parseComputeSpec(s string) (datadogV2.LogsAggregationFunction, string, error) {
	parts := strings.SplitN(s, ":", 2)
	agg, err := datadogV2.NewLogsAggregationFunctionFromValue(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("unknown aggregation %q", parts[0])
	}
	metric := ""
	if len(parts) == 2 {
		metric = parts[1]
	}
	return *agg, metric, nil
}

// sortedKeys returns map keys in sorted order (for map[string]interface{}).
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// sortedKeys2 returns map keys in sorted order (for map[string]LogsAggregateBucketValue).
func sortedKeys2(m map[string]datadogV2.LogsAggregateBucketValue) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// sortStrings sorts a string slice in place (simple insertion sort for small slices).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

func formatBucketValue(v datadogV2.LogsAggregateBucketValue) string {
	switch {
	case v.LogsAggregateBucketValueSingleString != nil:
		return *v.LogsAggregateBucketValueSingleString
	case v.LogsAggregateBucketValueSingleNumber != nil:
		f := *v.LogsAggregateBucketValueSingleNumber
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	default:
		return ""
	}
}

// parseRelativeTime parses "now", "now-<duration>", or RFC3339 into time.Time.
func parseRelativeTime(s string) (time.Time, error) {
	if s == "now" {
		return time.Now(), nil
	}
	if strings.HasPrefix(s, "now-") {
		d, err := time.ParseDuration(s[4:])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid relative time %q: %w", s, err)
		}
		return time.Now().Add(-d), nil
	}
	return time.Parse(time.RFC3339, s)
}
