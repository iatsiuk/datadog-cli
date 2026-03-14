package cmd

import (
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

func newSyntheticsTriggerCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var ids string

	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Trigger one or more Synthetic tests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if ids == "" {
				return errSyntheticsIDFlagRequired
			}

			publicIDs := strings.Split(ids, ",")
			tests := make([]datadogV1.SyntheticsTriggerTest, 0, len(publicIDs))
			for _, id := range publicIDs {
				id = strings.TrimSpace(id)
				t := datadogV1.NewSyntheticsTriggerTest(id)
				tests = append(tests, *t)
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			payload := datadogV1.NewSyntheticsTriggerBody(tests)
			resp, httpResp, err := sapi.api.TriggerTests(sapi.ctx, *payload)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("trigger synthetics tests: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			return synthTriggerTable(cmd, resp)
		},
	}

	cmd.Flags().StringVar(&ids, "id", "", "comma-separated public IDs to trigger (required)")
	return cmd
}

func synthTriggerTable(cmd *cobra.Command, resp datadogV1.SyntheticsTriggerCITestsResponse) error {
	batchID := resp.GetBatchId()
	fmt.Fprintf(cmd.OutOrStdout(), "BATCH_ID: %s\n", batchID) //nolint:errcheck

	results := resp.GetResults()
	headers := []string{"PUBLIC_ID", "RESULT_ID"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{
			r.GetPublicId(),
			r.GetResultId(),
		})
	}
	return output.PrintTable(cmd.OutOrStdout(), headers, rows)
}

func newSyntheticsBatchCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch <batch-id>",
		Short: "Show details of a Synthetic CI batch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			batchID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSyntheticsCIBatch(sapi.ctx, batchID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get synthetics batch: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			return synthBatchTable(cmd, resp)
		},
	}

	return cmd
}

func synthBatchTable(cmd *cobra.Command, resp datadogV1.SyntheticsBatchDetails) error {
	data := resp.GetData()
	status := ""
	if s := data.Status; s != nil {
		status = string(*s)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "STATUS: %s\n", status) //nolint:errcheck

	results := data.GetResults()
	headers := []string{"TEST_PUBLIC_ID", "TEST_NAME", "RESULT_ID", "LOCATION", "STATUS"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		resultStatus := ""
		if s := r.Status; s != nil {
			resultStatus = string(*s)
		}
		rows = append(rows, []string{
			r.GetTestPublicId(),
			r.GetTestName(),
			r.GetResultId(),
			r.GetLocation(),
			resultStatus,
		})
	}
	return output.PrintTable(cmd.OutOrStdout(), headers, rows)
}
