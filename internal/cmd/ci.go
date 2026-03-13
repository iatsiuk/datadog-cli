package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

const ciTailPollInterval = 5 * time.Second

type pipelinesAPI struct {
	api *datadogV2.CIVisibilityPipelinesApi
	ctx context.Context
}

func defaultPipelinesAPI() (*pipelinesAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &pipelinesAPI{api: datadogV2.NewCIVisibilityPipelinesApi(c), ctx: ctx}, nil
}

// NewCICommand returns the ci cobra command group.
func NewCICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Search and manage Datadog CI Visibility",
	}
	cmd.AddCommand(newCIPipelineCmd(defaultPipelinesAPI))
	cmd.AddCommand(newCITestCmd(defaultTestsAPI))
	return cmd
}

func newCIPipelineCmd(mkAPI func() (*pipelinesAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "CI pipeline events",
	}
	cmd.AddCommand(newCIPipelineSearchCmd(mkAPI))
	cmd.AddCommand(newCIPipelineTailCmd(mkAPI))
	cmd.AddCommand(newCIPipelineAggregateCmd(mkAPI))
	cmd.AddCommand(newCIPipelineCreateCmd(mkAPI))
	return cmd
}

func newCIPipelineSearchCmd(mkAPI func() (*pipelinesAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
		sortStr string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search CI pipeline events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fromStr == "" {
				fromStr = "now-1h"
			}

			fromTime, err := parseRelativeTime(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toTime, err := parseRelativeTime(toStr)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}
			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListCIAppPipelineEventsOptionalParameters().
				WithFilterFrom(fromTime.UTC()).
				WithFilterTo(toTime.UTC()).
				WithPageLimit(pageLimit)
			if query != "" {
				opts = opts.WithFilterQuery(query)
			}
			if sortStr != "" {
				s, err := datadogV2.NewCIAppSortFromValue(sortStr)
				if err != nil {
					return fmt.Errorf("--sort: %w", err)
				}
				opts = opts.WithSort(*s)
			}

			resp, httpResp, err := papi.api.ListCIAppPipelineEvents(papi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search pipeline events: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.CIAppPipelineEvent{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "PIPELINE", "STATUS", "DURATION", "BRANCH"}
			var rows [][]string
			for _, event := range resp.GetData() {
				eventAttrs := event.GetAttributes()
				attrs := eventAttrs.GetAttributes()
				ts := strAttr(attrs, "@timestamp")
				name := strAttr(attrs, "ci.pipeline.name")
				status := strAttr(attrs, "ci.status")
				branch := strAttr(attrs, "git.branch")
				duration := ""
				if d, ok := attrs["duration"]; ok {
					duration = fmtCIDuration(d)
				}
				rows = append(rows, []string{ts, name, status, duration, branch})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "pipeline event search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-1h (default: now-1h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of events to return")
	cmd.Flags().StringVar(&sortStr, "sort", "", "sort order: timestamp or -timestamp")
	return cmd
}

func newCIPipelineTailCmd(mkAPI func() (*pipelinesAPI, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail CI pipeline events in real time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			const ingestionOverlap = 10 * time.Second
			since := time.Now()
			var prevSeen map[string]struct{}
			currSeen := map[string]struct{}{}

			for {
				to := time.Now()
				baseOpts := datadogV2.NewListCIAppPipelineEventsOptionalParameters().
					WithFilterFrom(since.UTC()).
					WithFilterTo(to.UTC()).
					WithPageLimit(100)
				if query != "" {
					baseOpts = baseOpts.WithFilterQuery(query)
				}

				nextSeen := map[string]struct{}{}
				opts := *baseOpts
				apiErr := func() error {
					for {
						resp, httpResp, innerErr := papi.api.ListCIAppPipelineEvents(papi.ctx, opts)
						if httpResp != nil {
							_ = httpResp.Body.Close()
						}
						if innerErr != nil {
							return innerErr
						}
						for _, event := range resp.GetData() {
							id := event.GetId()
							if id == "" {
								continue
							}
							if _, inNext := nextSeen[id]; inNext {
								continue
							}
							nextSeen[id] = struct{}{}
							if _, inPrev := prevSeen[id]; inPrev {
								continue
							}
							if _, inCurr := currSeen[id]; inCurr {
								continue
							}
							currSeen[id] = struct{}{}
							if asJSON {
								_ = output.PrintJSON(cmd.OutOrStdout(), event)
							} else {
								eventAttrs := event.GetAttributes()
								attrs := eventAttrs.GetAttributes()
								ts := strAttr(attrs, "@timestamp")
								name := strAttr(attrs, "ci.pipeline.name")
								status := strAttr(attrs, "ci.status")
								branch := strAttr(attrs, "git.branch")
								duration := ""
								if d, ok := attrs["duration"]; ok {
									duration = fmtCIDuration(d)
								}
								_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n",
									ts, name, status, duration, branch)
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
				} else {
					prevSeen = nextSeen
					currSeen = make(map[string]struct{})
					since = to.Add(-ingestionOverlap)
				}

				timer := time.NewTimer(ciTailPollInterval)
				select {
				case <-papi.ctx.Done():
					timer.Stop()
					return nil
				case <-timer.C:
				}
			}
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "pipeline event search query")
	return cmd
}

func newCIPipelineAggregateCmd(mkAPI func() (*pipelinesAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		groupBy []string
		compute string
	)

	cmd := &cobra.Command{
		Use:   "aggregate",
		Short: "Aggregate CI pipeline events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if compute == "" {
				return fmt.Errorf("--compute is required")
			}
			aggFn, metric, err := parseCIComputeSpec(compute)
			if err != nil {
				return fmt.Errorf("--compute: %w", err)
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			if fromStr == "" {
				fromStr = "now-1h"
			}
			if toStr == "" {
				toStr = "now"
			}

			filter := datadogV2.NewCIAppPipelinesQueryFilter()
			filter.SetFrom(fromStr)
			filter.SetTo(toStr)
			if query != "" {
				filter.SetQuery(query)
			}

			c := datadogV2.NewCIAppCompute(aggFn)
			if metric != "" {
				c.SetMetric(metric)
			}
			computes := []datadogV2.CIAppCompute{*c}

			var groups []datadogV2.CIAppPipelinesGroupBy
			for _, facet := range groupBy {
				groups = append(groups, *datadogV2.NewCIAppPipelinesGroupBy(facet))
			}

			req := datadogV2.NewCIAppPipelinesAggregateRequest()
			req.SetFilter(*filter)
			req.SetCompute(computes)
			if len(groups) > 0 {
				req.SetGroupBy(groups)
			}

			resp, httpResp, err := papi.api.AggregateCIAppPipelineEvents(papi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("aggregate pipeline events: %w", err)
			}

			respData := resp.GetData()
			buckets := respData.GetBuckets()

			groupHeaders := groupBy
			var computeKeys []string
			if len(buckets) > 0 {
				for k := range buckets[0].GetComputes() {
					computeKeys = append(computeKeys, k)
				}
				sort.Strings(computeKeys)
			}

			headers := append(append([]string(nil), groupHeaders...), computeKeys...)
			if len(headers) == 0 {
				headers = []string{"BY", "COMPUTE"}
			}

			var rows [][]string
			for _, bucket := range buckets {
				by := bucket.GetBy()
				bComputes := bucket.GetComputes()
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
					if bv, ok := bComputes[k]; ok {
						switch {
						case bv.CIAppAggregateBucketValueSingleNumber != nil:
							v = fmt.Sprintf("%g", *bv.CIAppAggregateBucketValueSingleNumber)
						case bv.CIAppAggregateBucketValueSingleString != nil:
							v = *bv.CIAppAggregateBucketValueSingleString
						case bv.CIAppAggregateBucketValueTimeseries != nil:
							v = fmt.Sprintf("%d points", len(bv.CIAppAggregateBucketValueTimeseries.Items))
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
				if buckets == nil {
					buckets = []datadogV2.CIAppPipelinesBucketResponse{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), buckets)
			}

			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "pipeline event search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, supports date math (default now-1h)")
	cmd.Flags().StringVar(&toStr, "to", "", "end time, supports date math (default now)")
	cmd.Flags().StringSliceVar(&groupBy, "group-by", nil, "facets to group by (repeatable)")
	cmd.Flags().StringVar(&compute, "compute", "", "aggregation function: count, sum, avg, min, max, etc.")
	return cmd
}

func newCIPipelineCreateCmd(mkAPI func() (*pipelinesAPI, error)) *cobra.Command {
	var (
		pipelineName string
		statusStr    string
		levelStr     string
		gitBranch    string
		gitSha       string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a CI pipeline event",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if pipelineName == "" {
				return fmt.Errorf("--pipeline-name is required")
			}
			if statusStr == "" {
				return fmt.Errorf("--status is required")
			}

			status, err := datadogV2.NewCIAppPipelineEventPipelineStatusFromValue(statusStr)
			if err != nil {
				return fmt.Errorf("--status: %w", err)
			}

			level := datadogV2.CIAPPPIPELINEEVENTPIPELINELEVEL_PIPELINE
			if levelStr != "" && levelStr != string(datadogV2.CIAPPPIPELINEEVENTPIPELINELEVEL_PIPELINE) {
				l, err := datadogV2.NewCIAppPipelineEventPipelineLevelFromValue(levelStr)
				if err != nil {
					return fmt.Errorf("--level: %w", err)
				}
				level = *l
			}

			now := time.Now().UTC()
			uniqueID := fmt.Sprintf("%d", now.UnixNano())
			pipeline := datadogV2.NewCIAppPipelineEventFinishedPipeline(
				now, level, pipelineName, false, now, *status, uniqueID, "",
			)

			if gitBranch != "" && gitSha == "" {
				return fmt.Errorf("--git-sha is required when --git-branch is specified")
			}
			if gitBranch != "" || gitSha != "" {
				gitInfo := datadogV2.NewCIAppGitInfo("", "", gitSha)
				if gitBranch != "" {
					gitInfo.Branch = *datadog.NewNullableString(&gitBranch)
				}
				pipeline.Git = *datadogV2.NewNullableCIAppGitInfo(gitInfo)
			}

			pipelineUnion := &datadogV2.CIAppPipelineEventPipeline{
				CIAppPipelineEventFinishedPipeline: pipeline,
			}
			resource := datadogV2.CIAppPipelineEventPipelineAsCIAppCreatePipelineEventRequestAttributesResource(pipelineUnion)
			attrs := datadogV2.NewCIAppCreatePipelineEventRequestAttributes(resource)
			dataType := datadogV2.CIAPPCREATEPIPELINEEVENTREQUESTDATATYPE_CIPIPELINE_RESOURCE_REQUEST
			reqData := datadogV2.CIAppCreatePipelineEventRequestData{
				Attributes: attrs,
				Type:       &dataType,
			}
			singleOrArray := datadogV2.CIAppCreatePipelineEventRequestDataAsCIAppCreatePipelineEventRequestDataSingleOrArray(&reqData)
			req := datadogV2.NewCIAppCreatePipelineEventRequest()
			req.SetData(singleOrArray)

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := papi.api.CreateCIAppPipelineEvent(papi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create pipeline event: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Pipeline event created")
			return nil
		},
	}

	cmd.Flags().StringVar(&pipelineName, "pipeline-name", "", "pipeline name (required)")
	cmd.Flags().StringVar(&statusStr, "status", "", "pipeline status: success, error, canceled, skipped, blocked (required)")
	cmd.Flags().StringVar(&levelStr, "level", "pipeline", "event level")
	cmd.Flags().StringVar(&gitBranch, "git-branch", "", "git branch")
	cmd.Flags().StringVar(&gitSha, "git-sha", "", "git commit SHA")
	return cmd
}

// parseCIComputeSpec parses "count" or "sum:@metric" into aggregation function and metric.
func parseCIComputeSpec(s string) (datadogV2.CIAppAggregationFunction, string, error) {
	parts := strings.SplitN(s, ":", 2)
	agg, err := datadogV2.NewCIAppAggregationFunctionFromValue(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("unknown aggregation %q", parts[0])
	}
	metric := ""
	if len(parts) == 2 {
		metric = parts[1]
	}
	return *agg, metric, nil
}

// fmtCIDuration formats a nanosecond duration value from CI event attributes.
func fmtCIDuration(v interface{}) string {
	switch d := v.(type) {
	case float64:
		return (time.Duration(int64(d)) * time.Nanosecond).String()
	case int64:
		return (time.Duration(d) * time.Nanosecond).String()
	}
	return ""
}

// strAttr extracts a string value from a map[string]interface{}.
func strAttr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}
