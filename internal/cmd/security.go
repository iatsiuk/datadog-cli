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
