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
