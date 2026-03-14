package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errSyntheticsUptimeFromRequired = errors.New("--from is required")

func newSyntheticsUptimeCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		ids     string
		fromStr string
		toStr   string
	)

	cmd := &cobra.Command{
		Use:   "uptime",
		Short: "Fetch uptime for Synthetic tests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if ids == "" {
				return errSyntheticsIDFlagRequired
			}
			if fromStr == "" {
				return errSyntheticsUptimeFromRequired
			}

			publicIDs := splitTrimmed(ids)

			fromTime, err := parseRelativeTime(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}

			var toTime time.Time
			if toStr == "" || toStr == "now" {
				toTime = time.Now()
			} else {
				toTime, err = parseRelativeTime(toStr)
				if err != nil {
					return fmt.Errorf("--to: %w", err)
				}
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			payload := datadogV1.NewSyntheticsFetchUptimesPayload(
				fromTime.Unix(),
				publicIDs,
				toTime.Unix(),
			)

			uptimes, httpResp, err := sapi.api.FetchUptimes(sapi.ctx, *payload)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("fetch uptimes: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if uptimes == nil {
					uptimes = []datadogV1.SyntheticsTestUptime{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), uptimes)
			}

			headers := []string{"PUBLIC_ID", "UPTIME_%"}
			rows := make([][]string, 0, len(uptimes))
			for _, u := range uptimes {
				uptime := "-"
				if overall, ok := u.GetOverallOk(); ok {
					if pct, ok2 := overall.GetUptimeOk(); ok2 {
						uptime = fmt.Sprintf("%.2f", *pct*100)
					}
				}
				rows = append(rows, []string{
					u.GetPublicId(),
					uptime,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&ids, "id", "", "comma-separated public IDs (required)")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time, e.g. now-24h or RFC3339 (required)")
	cmd.Flags().StringVar(&toStr, "to", "now", "end time, e.g. now or RFC3339")
	return cmd
}
