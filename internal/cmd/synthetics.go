package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

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
	return cmd
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
