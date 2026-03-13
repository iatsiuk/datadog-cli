package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type teamsAPI struct {
	api *datadogV2.TeamsApi
	ctx context.Context
}

func defaultTeamsAPI() (*teamsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &teamsAPI{api: datadogV2.NewTeamsApi(c), ctx: ctx}, nil
}

// NewTeamsCommand returns the teams cobra command group.
func NewTeamsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teams",
		Short: "Manage Datadog teams",
	}
	cmd.AddCommand(newTeamsListCmd(defaultTeamsAPI))
	return cmd
}

func newTeamsListCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var filter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List teams",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListTeamsOptionalParameters()
			if filter != "" {
				opts = opts.WithFilterKeyword(filter)
			}

			resp, httpResp, err := tapi.api.ListTeams(tapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list teams: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV2.Team{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			return printTeamsTable(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "filter teams by keyword")
	return cmd
}

func printTeamsTable(w io.Writer, data []datadogV2.Team) error {
	headers := []string{"ID", "NAME", "HANDLE", "USER_COUNT", "DESCRIPTION"}
	var rows [][]string
	for _, t := range data {
		attrs := t.GetAttributes()
		userCount := ""
		if c := attrs.UserCount; c != nil {
			userCount = fmt.Sprintf("%d", *c)
		}
		rows = append(rows, []string{
			t.GetId(),
			attrs.GetName(),
			attrs.GetHandle(),
			userCount,
			attrs.GetDescription(),
		})
	}
	return output.PrintTable(w, headers, rows)
}
