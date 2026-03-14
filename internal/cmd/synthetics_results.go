package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

func newSyntheticsResultsCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var resultID string

	cmd := &cobra.Command{
		Use:   "results <public-id>",
		Short: "Show results of a Synthetic test",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			publicID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			// detect test type
			summary, httpResp, err := sapi.api.GetTest(sapi.ctx, publicID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get synthetics test: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			testType := summary.GetType()
			if testType == datadogV1.SYNTHETICSTESTDETAILSTYPE_MOBILE {
				return fmt.Errorf("results not supported for mobile tests")
			}
			isBrowser := testType == datadogV1.SYNTHETICSTESTDETAILSTYPE_BROWSER

			if resultID != "" {
				return synthGetSingleResult(cmd, sapi, publicID, resultID, isBrowser, asJSON)
			}
			return synthGetLatestResults(cmd, sapi, publicID, isBrowser, asJSON)
		},
	}

	cmd.Flags().StringVar(&resultID, "result-id", "", "specific result ID to fetch")
	return cmd
}

func synthGetLatestResults(cmd *cobra.Command, sapi *syntheticsAPI, publicID string, isBrowser bool, asJSON bool) error {
	if isBrowser {
		resp, httpResp, err := sapi.api.GetBrowserTestLatestResults(sapi.ctx, publicID)
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		if err != nil {
			return fmt.Errorf("get browser test latest results: %w", err)
		}
		results := resp.GetResults()
		if asJSON {
			if results == nil {
				results = []datadogV1.SyntheticsBrowserTestResultShort{}
			}
			return output.PrintJSON(cmd.OutOrStdout(), results)
		}
		return synthBrowserResultsTable(cmd, results)
	}

	resp, httpResp, err := sapi.api.GetAPITestLatestResults(sapi.ctx, publicID)
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("get api test latest results: %w", err)
	}
	results := resp.GetResults()
	if asJSON {
		if results == nil {
			results = []datadogV1.SyntheticsAPITestResultShort{}
		}
		return output.PrintJSON(cmd.OutOrStdout(), results)
	}
	return synthAPIResultsTable(cmd, results)
}

func synthGetSingleResult(cmd *cobra.Command, sapi *syntheticsAPI, publicID, resultID string, isBrowser bool, asJSON bool) error {
	if isBrowser {
		result, httpResp, err := sapi.api.GetBrowserTestResult(sapi.ctx, publicID, resultID)
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		if err != nil {
			return fmt.Errorf("get browser test result: %w", err)
		}
		if asJSON {
			return output.PrintJSON(cmd.OutOrStdout(), result)
		}
		return synthBrowserResultFullTable(cmd, result)
	}

	result, httpResp, err := sapi.api.GetAPITestResult(sapi.ctx, publicID, resultID)
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("get api test result: %w", err)
	}
	if asJSON {
		return output.PrintJSON(cmd.OutOrStdout(), result)
	}
	return synthAPIResultFullTable(cmd, result)
}

func synthResultStatus(status *datadogV1.SyntheticsTestMonitorStatus) string {
	if status == nil {
		return ""
	}
	switch *status {
	case datadogV1.SYNTHETICSTESTMONITORSTATUS_UNTRIGGERED:
		return "untriggered"
	case datadogV1.SYNTHETICSTESTMONITORSTATUS_TRIGGERED:
		return "triggered"
	case datadogV1.SYNTHETICSTESTMONITORSTATUS_NO_DATA:
		return "no_data"
	default:
		return strconv.FormatInt(int64(*status), 10)
	}
}

func synthFormatTimestamp(ts *float64) string {
	if ts == nil {
		return ""
	}
	// check_time is in milliseconds
	sec := int64(*ts) / 1000
	return time.Unix(sec, 0).UTC().Format("2006-01-02T15:04:05Z")
}

func synthAPIResultsTable(cmd *cobra.Command, results []datadogV1.SyntheticsAPITestResultShort) error {
	headers := []string{"RESULT_ID", "LOCATION", "STATUS", "TIMESTAMP"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{
			r.GetResultId(),
			r.GetProbeDc(),
			synthResultStatus(r.Status),
			synthFormatTimestamp(r.CheckTime),
		})
	}
	return output.PrintTable(cmd.OutOrStdout(), headers, rows)
}

func synthBrowserResultsTable(cmd *cobra.Command, results []datadogV1.SyntheticsBrowserTestResultShort) error {
	headers := []string{"RESULT_ID", "LOCATION", "STATUS", "TIMESTAMP"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{
			r.GetResultId(),
			r.GetProbeDc(),
			synthResultStatus(r.Status),
			synthFormatTimestamp(r.CheckTime),
		})
	}
	return output.PrintTable(cmd.OutOrStdout(), headers, rows)
}

func synthAPIResultFullTable(cmd *cobra.Command, r datadogV1.SyntheticsAPITestResultFull) error {
	eventType := ""
	if res := r.Result; res != nil {
		if res.EventType != nil {
			eventType = string(*res.EventType)
		}
	}
	rows := [][]string{
		{"RESULT_ID", r.GetResultId()},
		{"LOCATION", r.GetProbeDc()},
		{"STATUS", synthResultStatus(r.Status)},
		{"TIMESTAMP", synthFormatTimestamp(r.CheckTime)},
		{"EVENT_TYPE", eventType},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func synthBrowserResultFullTable(cmd *cobra.Command, r datadogV1.SyntheticsBrowserTestResultFull) error {
	duration := ""
	if res := r.Result; res != nil {
		if res.Duration != nil {
			duration = fmt.Sprintf("%.1f ms", *res.Duration)
		}
	}
	rows := [][]string{
		{"RESULT_ID", r.GetResultId()},
		{"LOCATION", r.GetProbeDc()},
		{"STATUS", synthResultStatus(r.Status)},
		{"TIMESTAMP", synthFormatTimestamp(r.CheckTime)},
		{"DURATION", duration},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}
