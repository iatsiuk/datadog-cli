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
	c.GetConfig().SetUnstableOperationEnabled("v2.EventsApi.CreateEvent", true)
	return &eventsAPI{api: datadogV2.NewEventsApi(c), ctx: ctx}, nil
}

// NewEventsCommand returns the events cobra command group.
func NewEventsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Search and manage Datadog events",
	}
	cmd.AddCommand(newEventsListCmd(defaultEventsAPI))
	cmd.AddCommand(newEventsSearchCmd(defaultEventsAPI))
	cmd.AddCommand(newEventsShowCmd(defaultEventsAPI))
	cmd.AddCommand(newEventsCreateCmd(defaultEventsAPI))
	return cmd
}

var validAlertTypes = map[string]datadogV2.AlertEventCustomAttributesStatus{
	"info":    datadogV2.ALERTEVENTCUSTOMATTRIBUTESSTATUS_OK,
	"success": datadogV2.ALERTEVENTCUSTOMATTRIBUTESSTATUS_OK,
	"warning": datadogV2.ALERTEVENTCUSTOMATTRIBUTESSTATUS_WARN,
	"error":   datadogV2.ALERTEVENTCUSTOMATTRIBUTESSTATUS_ERROR,
}

func newEventsCreateCmd(mkAPI func() (*eventsAPI, error)) *cobra.Command {
	var (
		title     string
		text      string
		tagsStr   string
		alertType string
		source    string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an event",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}

			status, ok := validAlertTypes[alertType]
			if !ok {
				return fmt.Errorf("--alert-type must be one of: info, warning, error, success")
			}

			eapi, err := mkAPI()
			if err != nil {
				return err
			}

			alertAttrs := datadogV2.NewAlertEventCustomAttributes(status)
			attrs := datadogV2.AlertEventCustomAttributesAsEventPayloadAttributes(alertAttrs)

			payload := datadogV2.NewEventPayload(attrs, datadogV2.EVENTCATEGORY_ALERT, title)
			if text != "" {
				payload.SetMessage(text)
			}
			if tagsStr != "" {
				payload.SetTags(strings.Split(tagsStr, ","))
			}
			if source != "" {
				_ = source // no direct source field in v2 create API
			}

			req := datadogV2.NewEventCreateRequest(*payload, datadogV2.EVENTCREATEREQUESTTYPE_EVENT)
			body := datadogV2.NewEventCreateRequestPayload(*req)

			resp, httpResp, err := eapi.api.CreateEvent(eapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create event: %w", err)
			}

			data := resp.GetData()
			respAttrs := data.GetAttributes()
			innerAttrs := respAttrs.GetAttributes()
			evt := innerAttrs.GetEvt()
			eventID := evt.GetId()
			if uid := evt.GetUid(); uid != "" {
				eventID = uid
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created event: %s\n", eventID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "event title (required)")
	cmd.Flags().StringVar(&text, "text", "", "event text/body")
	cmd.Flags().StringVar(&tagsStr, "tags", "", "comma-separated tags, e.g. env:prod,service:web")
	cmd.Flags().StringVar(&alertType, "alert-type", "info", "alert type: info, warning, error, success")
	cmd.Flags().StringVar(&source, "source", "", "event source")
	return cmd
}

func newEventsShowCmd(mkAPI func() (*eventsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <event-id>",
		Short: "Show event details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eventID := args[0]

			eapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := eapi.api.GetEvent(eapi.ctx, eventID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get event: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			event := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), event)
			}

			attrs := event.GetAttributes()
			fields := []struct{ k, v string }{
				{"ID", event.GetId()},
				{"Date", attrs.GetTimestamp()},
				{"Tags", strings.Join(attrs.GetTags(), ", ")},
				{"Message", attrs.GetMessage()},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				fmt.Fprintf(w, "%-12s %s\n", f.k+":", f.v) //nolint:errcheck
			}
			return nil
		},
	}
	return cmd
}

func newEventsSearchCmd(mkAPI func() (*eventsAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}
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

			filter := datadogV2.NewEventsQueryFilter()
			filter.SetQuery(query)
			filter.SetFrom(fromTime.UTC().Format(time.RFC3339))
			filter.SetTo(toTime.UTC().Format(time.RFC3339))

			pageLimit := int32(limit) //nolint:gosec
			page := datadogV2.NewEventsRequestPage()
			page.SetLimit(pageLimit)

			body := datadogV2.NewEventsListRequest()
			body.SetFilter(*filter)
			body.SetPage(*page)

			opts := datadogV2.NewSearchEventsOptionalParameters().WithBody(*body)
			resp, httpResp, err := eapi.api.SearchEvents(eapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search events: %w", err)
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

	cmd.Flags().StringVar(&query, "query", "", "search query (required)")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-24h (default: now-24h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of events to return")
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
