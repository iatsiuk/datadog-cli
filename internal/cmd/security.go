package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

const securityTailPollInterval = 5 * time.Second

type securityAPI struct {
	api *datadogV2.SecurityMonitoringApi
	ctx context.Context
}

func defaultSecurityAPI() (*securityAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &securityAPI{api: datadogV2.NewSecurityMonitoringApi(c), ctx: ctx}, nil
}

// NewSecurityCommand returns the security cobra command group.
func NewSecurityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Manage Datadog Security Monitoring",
	}
	sig := &cobra.Command{
		Use:   "signal",
		Short: "Manage security signals",
	}
	sig.AddCommand(newSecuritySignalSearchCmd(defaultSecurityAPI))
	sig.AddCommand(newSecuritySignalTailCmd(defaultSecurityAPI))
	sig.AddCommand(newSecuritySignalShowCmd(defaultSecurityAPI))
	cmd.AddCommand(sig)
	return cmd
}

func newSecuritySignalSearchCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		query   string
		fromStr string
		toStr   string
		sort    string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search security signals",
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

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}
			pageLimit := int32(limit) //nolint:gosec
			opts := datadogV2.NewListSecurityMonitoringSignalsOptionalParameters().
				WithFilterFrom(fromTime).
				WithFilterTo(toTime).
				WithPageLimit(pageLimit)
			if query != "" {
				opts = opts.WithFilterQuery(query)
			}
			if sort != "" {
				sv, sortErr := datadogV2.NewSecurityMonitoringSignalsSortFromValue(sort)
				if sortErr != nil {
					return fmt.Errorf("--sort: %w", sortErr)
				}
				opts = opts.WithSort(*sv)
			}

			resp, httpResp, err := sapi.api.ListSecurityMonitoringSignals(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search signals: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.SecurityMonitoringSignal{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"TIMESTAMP", "ID", "RULE_NAME", "SEVERITY", "STATUS"}
			var rows [][]string
			for _, sig := range resp.GetData() {
				attrs := sig.GetAttributes()
				ts := ""
				if t := attrs.Timestamp; t != nil {
					ts = t.UTC().Format(time.RFC3339)
				}
				rows = append(rows, []string{
					ts,
					sig.GetId(),
					signalCustomString(attrs.Custom, "workflow.rule.name"),
					signalTagValue(attrs.Tags, "severity"),
					signalCustomString(attrs.Custom, "status"),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "signal search query")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-1h (default: now-1h)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now")
	cmd.Flags().StringVar(&sort, "sort", "", "sort order: timestamp or -timestamp")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of signals to return")
	return cmd
}

// signalCustomString extracts a string value from the signal Custom map.
func signalCustomString(custom map[string]interface{}, key string) string {
	if v, ok := custom[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func newSecuritySignalTailCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail security signals in real time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			const ingestionOverlap = 30 * time.Second
			since := time.Now()
			var prevSeen map[string]struct{}
			currSeen := map[string]struct{}{}

			for {
				to := time.Now()
				baseOpts := datadogV2.NewListSecurityMonitoringSignalsOptionalParameters().
					WithFilterFrom(since).
					WithFilterTo(to).
					WithSort(datadogV2.SECURITYMONITORINGSIGNALSSORT_TIMESTAMP_ASCENDING).
					WithPageLimit(1000)
				if query != "" {
					baseOpts = baseOpts.WithFilterQuery(query)
				}

				nextSeen := map[string]struct{}{}
				opts := *baseOpts
				apiErr := func() error {
					for {
						resp, httpResp, innerErr := sapi.api.ListSecurityMonitoringSignals(sapi.ctx, opts)
						if httpResp != nil {
							_ = httpResp.Body.Close()
						}
						if innerErr != nil {
							return innerErr
						}
						for _, sig := range resp.GetData() {
							id := sig.GetId()
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
							attrs := sig.GetAttributes()
							ts := ""
							if t := attrs.Timestamp; t != nil {
								ts = t.UTC().Format(time.RFC3339)
							}
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n",
								ts, id,
								signalCustomString(attrs.Custom, "workflow.rule.name"),
								signalTagValue(attrs.Tags, "severity"),
								signalCustomString(attrs.Custom, "status"),
							)
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
				case <-sapi.ctx.Done():
					return nil
				case <-time.After(securityTailPollInterval):
				}
			}
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "signal filter query")
	return cmd
}

func newSecuritySignalShowCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <signal-id>",
		Short: "Show security signal details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			signalID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSecurityMonitoringSignal(sapi.ctx, signalID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get signal: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			sig := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), sig)
			}

			attrs := sig.GetAttributes()
			ts := ""
			if t := attrs.Timestamp; t != nil {
				ts = t.UTC().Format(time.RFC3339)
			}
			fields := []struct{ k, v string }{
				{"ID", sig.GetId()},
				{"Timestamp", ts},
				{"Rule", signalCustomString(attrs.Custom, "workflow.rule.name")},
				{"Severity", signalTagValue(attrs.Tags, "severity")},
				{"Status", signalCustomString(attrs.Custom, "status")},
				{"Message", attrs.GetMessage()},
				{"Tags", strings.Join(attrs.GetTags(), ", ")},
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

// signalTagValue extracts the value for a tag prefix from a tags slice.
// For example, prefix "severity" matches tag "severity:high" and returns "high".
func signalTagValue(tags []string, prefix string) string {
	p := prefix + ":"
	for _, tag := range tags {
		if strings.HasPrefix(tag, p) {
			return tag[len(p):]
		}
	}
	return ""
}
