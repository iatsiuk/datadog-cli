package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	return cmd
}

func newCIPipelineCmd(mkAPI func() (*pipelinesAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "CI pipeline events",
	}
	cmd.AddCommand(newCIPipelineSearchCmd(mkAPI))
	cmd.AddCommand(newCIPipelineTailCmd(mkAPI))
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
					switch v := d.(type) {
					case float64:
						duration = (time.Duration(int64(v)) * time.Nanosecond).String()
					case int64:
						duration = (time.Duration(v) * time.Nanosecond).String()
					}
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
							nextSeen[id] = struct{}{}
							if _, inPrev := prevSeen[id]; inPrev {
								continue
							}
							if _, inCurr := currSeen[id]; inCurr {
								continue
							}
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
									switch v := d.(type) {
									case float64:
										duration = (time.Duration(int64(v)) * time.Nanosecond).String()
									case int64:
										duration = (time.Duration(v) * time.Nanosecond).String()
									}
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
					prevSeen = currSeen
					currSeen = nextSeen
					since = to.Add(-ingestionOverlap)
				}

				select {
				case <-papi.ctx.Done():
					return nil
				case <-time.After(ciTailPollInterval):
				}
			}
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "pipeline event search query")
	return cmd
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
