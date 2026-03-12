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

type eventsAPI struct {
	api *datadogV2.EventsApi
	ctx context.Context
}

func defaultEventsAPI() (*eventsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &eventsAPI{api: datadogV2.NewEventsApi(c), ctx: ctx}, nil
}

// NewEventsCommand returns the events cobra command group.
func NewEventsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Search and manage Datadog events",
	}
	cmd.AddCommand(newEventsListCmd(defaultEventsAPI))
	return cmd
}

func newEventsListCmd(mkAPI func() (*eventsAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
		sortStr string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fromStr == "" {
				fromStr = "now-24h"
			}

			fromTime, err := parseRelativeTime(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toTime, err := parseRelativeTime(toStr)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}

			eapi, err := mkAPI()
			if err != nil {
				return err
			}

			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListEventsOptionalParameters().
				WithFilterFrom(fromTime.UTC().Format(time.RFC3339)).
				WithFilterTo(toTime.UTC().Format(time.RFC3339)).
				WithPageLimit(pageLimit)
			if query != "" {
				opts = opts.WithFilterQuery(query)
			}
			if sortStr != "" {
				opts = opts.WithSort(datadogV2.EventsSort(sortStr))
			}

			resp, httpResp, err := eapi.api.ListEvents(eapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list events: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV2.EventResponse{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "TITLE", "SOURCE", "TAGS"}
			var rows [][]string
			for _, event := range data {
				attrs := event.GetAttributes()
				ts := ""
				if t := attrs.Timestamp; t != nil {
					ts = t.UTC().Format(time.RFC3339)
				}
				inner := attrs.GetAttributes()
				title := inner.GetTitle()
				source := inner.GetSourceTypeName()
				tags := strings.Join(attrs.GetTags(), ", ")
				rows = append(rows, []string{ts, title, source, tags})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "event filter query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-24h (default: now-24h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of events to return")
	cmd.Flags().StringVar(&sortStr, "sort", "", "sort order: timestamp or -timestamp")
	return cmd
}
