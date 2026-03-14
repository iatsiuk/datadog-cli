package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errSyntheticsIDFlagRequired = errors.New("--id is required")
var errSyntheticsYesRequired = errors.New("--yes is required to confirm destructive action")

type syntheticsAPI struct {
	api *datadogV1.SyntheticsApi
	ctx context.Context
}

func defaultSyntheticsAPI() (*syntheticsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &syntheticsAPI{api: datadogV1.NewSyntheticsApi(c), ctx: ctx}, nil
}

// NewSyntheticsCommand returns the synthetics cobra command group.
func NewSyntheticsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "synthetics",
		Short: "Manage Datadog Synthetic tests",
	}
	cmd.AddCommand(newSyntheticsListCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsSearchCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsShowCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsDeleteCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsCreateCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsResultsCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsTriggerCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsBatchCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsVariableCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsLocationCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsPrivateLocationCmd(defaultSyntheticsAPI))
	cmd.AddCommand(newSyntheticsUptimeCmd(defaultSyntheticsAPI))
	return cmd
}

func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func synthTestsTableOutput(cmd *cobra.Command, tests []datadogV1.SyntheticsTestDetailsWithoutSteps) error {
	headers := []string{"PUBLIC_ID", "NAME", "TYPE", "STATUS", "LOCATIONS"}
	rows := make([][]string, 0, len(tests))
	for _, t := range tests {
		locs := strings.Join(t.GetLocations(), ",")
		rows = append(rows, []string{
			t.GetPublicId(),
			t.GetName(),
			string(t.GetType()),
			string(t.GetStatus()),
			locs,
		})
	}
	return output.PrintTable(cmd.OutOrStdout(), headers, rows)
}

func newSyntheticsListCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var pageSize int64

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Synthetic tests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewListTestsOptionalParameters()
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}

			resp, httpResp, err := sapi.api.ListTests(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list synthetics tests: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			tests := resp.GetTests()

			if asJSON {
				if tests == nil {
					tests = []datadogV1.SyntheticsTestDetailsWithoutSteps{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), tests)
			}

			return synthTestsTableOutput(cmd, tests)
		},
	}

	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "number of tests per page")
	return cmd
}

func newSyntheticsSearchCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Synthetic tests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewSearchTestsOptionalParameters()
			if query != "" {
				opts = opts.WithText(query)
			}

			resp, httpResp, err := sapi.api.SearchTests(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search synthetics tests: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			tests := resp.GetTests()

			if asJSON {
				if tests == nil {
					tests = []datadogV1.SyntheticsTestDetailsWithoutSteps{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), tests)
			}

			return synthTestsTableOutput(cmd, tests)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "search query text")
	return cmd
}

func newSyntheticsShowCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <public-id>",
		Short: "Show details of a Synthetic test",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			publicID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			// detect test type first
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
			switch testType {
			case datadogV1.SYNTHETICSTESTDETAILSTYPE_BROWSER:
				t, httpResp2, err2 := sapi.api.GetBrowserTest(sapi.ctx, publicID)
				if httpResp2 != nil {
					_ = httpResp2.Body.Close()
				}
				if err2 != nil {
					return fmt.Errorf("get browser test: %w", err2)
				}
				if asJSON {
					return output.PrintJSON(cmd.OutOrStdout(), t)
				}
				return synthShowBrowserTest(cmd, t)
			case datadogV1.SYNTHETICSTESTDETAILSTYPE_MOBILE:
				t, httpResp2, err2 := sapi.api.GetMobileTest(sapi.ctx, publicID)
				if httpResp2 != nil {
					_ = httpResp2.Body.Close()
				}
				if err2 != nil {
					return fmt.Errorf("get mobile test: %w", err2)
				}
				if asJSON {
					return output.PrintJSON(cmd.OutOrStdout(), t)
				}
				return synthShowMobileTest(cmd, t)
			default: // api or unknown - fall back to API test
				t, httpResp2, err2 := sapi.api.GetAPITest(sapi.ctx, publicID)
				if httpResp2 != nil {
					_ = httpResp2.Body.Close()
				}
				if err2 != nil {
					return fmt.Errorf("get api test: %w", err2)
				}
				if asJSON {
					return output.PrintJSON(cmd.OutOrStdout(), t)
				}
				return synthShowAPITest(cmd, t)
			}
		},
	}
	return cmd
}

func synthShowAPITest(cmd *cobra.Command, t datadogV1.SyntheticsAPITest) error {
	rows := [][]string{
		{"PUBLIC_ID", t.GetPublicId()},
		{"NAME", t.GetName()},
		{"TYPE", string(t.GetType())},
		{"STATUS", string(t.GetStatus())},
		{"LOCATIONS", strings.Join(t.GetLocations(), ", ")},
		{"TAGS", strings.Join(t.GetTags(), ", ")},
		{"MESSAGE", t.GetMessage()},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func synthShowBrowserTest(cmd *cobra.Command, t datadogV1.SyntheticsBrowserTest) error {
	rows := [][]string{
		{"PUBLIC_ID", t.GetPublicId()},
		{"NAME", t.GetName()},
		{"TYPE", string(t.GetType())},
		{"STATUS", string(t.GetStatus())},
		{"LOCATIONS", strings.Join(t.GetLocations(), ", ")},
		{"TAGS", strings.Join(t.GetTags(), ", ")},
		{"MESSAGE", t.GetMessage()},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func synthShowMobileTest(cmd *cobra.Command, t datadogV1.SyntheticsMobileTest) error {
	rows := [][]string{
		{"PUBLIC_ID", t.GetPublicId()},
		{"NAME", t.GetName()},
		{"TYPE", string(t.GetType())},
		{"STATUS", string(t.GetStatus())},
		{"DEVICE_IDS", strings.Join(t.GetDeviceIds(), ", ")},
		{"TAGS", strings.Join(t.GetTags(), ", ")},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newSyntheticsDeleteCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		ids string
		yes bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete one or more Synthetic tests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if ids == "" {
				return errSyntheticsIDFlagRequired
			}
			if !yes {
				return errSyntheticsYesRequired
			}

			publicIDs := splitTrimmed(ids)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			payload := datadogV1.SyntheticsDeleteTestsPayload{}
			payload.SetPublicIds(publicIDs)

			resp, httpResp, err := sapi.api.DeleteTests(sapi.ctx, payload)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete synthetics tests: %w", err)
			}

			deleted := resp.GetDeletedTests()
			deletedIDs := make([]string, 0, len(deleted))
			for _, d := range deleted {
				deletedIDs = append(deletedIDs, d.GetPublicId())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %d test(s): %s\n", len(deletedIDs), strings.Join(deletedIDs, ", ")) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&ids, "id", "", "comma-separated public IDs to delete (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
