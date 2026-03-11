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

// NewAPMCommand returns the apm cobra command group.
func NewAPMCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apm",
		Short: "Search and manage Datadog APM",
	}
	cmd.AddCommand(newAPMSearchCmd(defaultSpansAPI))
	cmd.AddCommand(newAPMTailCmd(defaultSpansAPI))
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

			// start from now
			since := time.Now()

			for {
				now := time.Now()
				opts := datadogV2.NewListSpansGetOptionalParameters().
					WithFilterFrom(since.UTC().Format(time.RFC3339)).
					WithFilterTo(now.UTC().Format(time.RFC3339)).
					WithPageLimit(100)
				if effectiveQuery != "" {
					opts = opts.WithFilterQuery(effectiveQuery)
				}

				resp, httpResp, err := sapi.api.ListSpansGet(sapi.ctx, *opts)
				if httpResp != nil {
					_ = httpResp.Body.Close()
				}
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return nil
					}
					return fmt.Errorf("tail spans: %w", err)
				}

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
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n",
						ts, attrs.GetService(), attrs.GetResourceName(), duration, status)
				}

				since = now

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
