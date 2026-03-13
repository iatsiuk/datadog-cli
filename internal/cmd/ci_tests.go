package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type testsAPI struct {
	api *datadogV2.CIVisibilityTestsApi
	ctx context.Context
}

func defaultTestsAPI() (*testsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &testsAPI{api: datadogV2.NewCIVisibilityTestsApi(c), ctx: ctx}, nil
}

func newCITestCmd(mkAPI func() (*testsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "CI test events",
	}
	cmd.AddCommand(newCITestSearchCmd(mkAPI))
	return cmd
}

func newCITestSearchCmd(mkAPI func() (*testsAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
		sortStr string
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search CI test events",
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

			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}
			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListCIAppTestEventsOptionalParameters().
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

			resp, httpResp, err := tapi.api.ListCIAppTestEvents(tapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search test events: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.CIAppTestEvent{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "TEST", "SUITE", "STATUS", "DURATION", "SERVICE"}
			var rows [][]string
			for _, event := range resp.GetData() {
				eventAttrs := event.GetAttributes()
				attrs := eventAttrs.GetAttributes()
				ts := strAttr(attrs, "@timestamp")
				testName := strAttr(attrs, "test.name")
				suite := strAttr(attrs, "test.suite")
				status := strAttr(attrs, "test.status")
				service := strAttr(attrs, "service")
				duration := ""
				if d, ok := attrs["duration"]; ok {
					switch v := d.(type) {
					case float64:
						duration = (time.Duration(int64(v)) * time.Nanosecond).String()
					case int64:
						duration = (time.Duration(v) * time.Nanosecond).String()
					}
				}
				rows = append(rows, []string{ts, testName, suite, status, duration, service})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "test event search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-1h (default: now-1h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of events to return")
	cmd.Flags().StringVar(&sortStr, "sort", "", "sort order: timestamp or -timestamp")
	return cmd
}
