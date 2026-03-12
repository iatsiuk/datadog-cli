package cmd

import (
	"context"
	"fmt"
	"io"
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
	cmd.AddCommand(newEventsTailCmd(defaultEventsAPI))
	return cmd
}

func newEventsTailCmd(mkAPI func() (*eventsAPI, error)) *cobra.Command {
	var (
		query    string
		interval time.Duration
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail events in real time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			eapi, err := mkAPI()
			if err != nil {
				return err
			}

			ctx := eapi.ctx
			seen := make(map[string]bool)
			from := time.Now().Add(-interval)

			for {
				fromStr := from.UTC().Format(time.RFC3339)
				toStr := time.Now().UTC().Format(time.RFC3339)

				opts := datadogV2.NewListEventsOptionalParameters().
					WithFilterFrom(fromStr).
					WithFilterTo(toStr)
				if query != "" {
					opts = opts.WithFilterQuery(query)
				}

				resp, httpResp, err := eapi.api.ListEvents(ctx, *opts)
				if httpResp != nil {
					_ = httpResp.Body.Close()
				}
				if err != nil {
					// stop on context cancellation
					if ctx.Err() != nil {
						return nil
					}
					return fmt.Errorf("list events: %w", err)
				}

				for _, event := range resp.GetData() {
					id := event.GetId()
					if seen[id] {
						continue
					}
					seen[id] = true
					attrs := event.GetAttributes()
					ts := ""
					if t := attrs.Timestamp; t != nil {
						ts = t.UTC().Format(time.RFC3339)
					}
					inner := attrs.GetAttributes()
					title := inner.GetTitle()
					source := inner.GetSourceTypeName()
					tags := strings.Join(attrs.GetTags(), ", ")
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", ts, title, source, tags) //nolint:errcheck
				}

				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}

				from = time.Now().Add(-interval)
			}
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "event filter query")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "poll interval")
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
				rawTags := strings.Split(tagsStr, ",")
				tags := make([]string, len(rawTags))
				for i, t := range rawTags {
					tags[i] = strings.TrimSpace(t)
				}
				payload.SetTags(tags)
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
			title, alertType, url := extractAlertEventFields(attrs.GetAttributes())
			fields := []struct{ k, v string }{
				{"ID", event.GetId()},
				{"Title", title},
				{"Date", attrs.GetTimestamp()},
				{"Alert Type", alertType},
				{"Tags", strings.Join(attrs.GetTags(), ", ")},
				{"Message", attrs.GetMessage()},
				{"URL", url},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-12s %s\n", f.k+":", f.v) //nolint:errcheck
			}
			return nil
		},
	}
	return cmd
}

// extractAlertEventFields extracts title, alert type, and URL from the union
// attributes type. V2EventAttributesAttributes is a union of AlertEventAttributes
// and ChangeEventAttributes; both have Title. AlertEventAttributes also has Status
// and Links. When neither typed field is populated, fall back to UnparsedObject.
func extractAlertEventFields(inner datadogV2.V2EventAttributesAttributes) (title, alertType, url string) {
	if a := inner.AlertEventAttributes; a != nil {
		title = a.GetTitle()
		alertType = string(a.GetStatus())
		for _, link := range a.GetLinks() {
			if link.Url != nil {
				url = *link.Url
				break
			}
		}
		return
	}
	if c := inner.ChangeEventAttributes; c != nil {
		title = c.GetTitle()
		// read status/links from additionalProperties (alert-specific fields)
		if raw := c.AdditionalProperties; raw != nil {
			if v, ok := raw["status"].(string); ok {
				alertType = v
			}
			if links, ok := raw["links"].([]interface{}); ok {
				for _, l := range links {
					if link, ok := l.(map[string]interface{}); ok {
						if v, ok := link["url"].(string); ok {
							url = v
							break
						}
					}
				}
			}
		}
		return
	}
	raw, ok := inner.UnparsedObject.(map[string]interface{})
	if !ok {
		return
	}
	if v, ok := raw["title"].(string); ok {
		title = v
	}
	if v, ok := raw["status"].(string); ok {
		alertType = v
	}
	if links, ok := raw["links"].([]interface{}); ok {
		for _, l := range links {
			if link, ok := l.(map[string]interface{}); ok {
				if v, ok := link["url"].(string); ok {
					url = v
					break
				}
			}
		}
	}
	return
}

func printEventsTable(w io.Writer, data []datadogV2.EventResponse) error {
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
	return output.PrintTable(w, headers, rows)
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

			return printEventsTable(cmd.OutOrStdout(), data)
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

			return printEventsTable(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "event filter query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-24h (default: now-24h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of events to return")
	cmd.Flags().StringVar(&sortStr, "sort", "", "sort order: timestamp or -timestamp")
	return cmd
}
