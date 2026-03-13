package cmd

import (
	"errors"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errIncidentTeamNameRequired = errors.New("--name is required")
	errIncidentTeamYesRequired  = errors.New("--yes is required to delete an incident team")
)

func newIncidentsTeamCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage incident teams",
	}
	cmd.AddCommand(newIncidentTeamListCmd(mkAPI))
	cmd.AddCommand(newIncidentTeamShowCmd(mkAPI))
	cmd.AddCommand(newIncidentTeamCreateCmd(mkAPI))
	cmd.AddCommand(newIncidentTeamUpdateCmd(mkAPI))
	cmd.AddCommand(newIncidentTeamDeleteCmd(mkAPI))
	return cmd
}

func incidentTeamTableRows(items []datadogV2.IncidentTeamResponseData) [][]string {
	rows := make([][]string, 0, len(items))
	for i := range items {
		attrs := items[i].GetAttributes()
		rows = append(rows, []string{items[i].GetId(), attrs.GetName()})
	}
	return rows
}

func printIncidentTeamDetail(cmd *cobra.Command, d datadogV2.IncidentTeamResponseData) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"NAME", attrs.GetName()},
	}
	if t := attrs.GetCreated(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newIncidentTeamListCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List incident teams",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.teamsApi.ListIncidentTeams(iapi.ctx) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list incident teams: %w", err)
			}

			items := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if items == nil {
					items = []datadogV2.IncidentTeamResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), items)
			}

			headers := []string{"ID", "NAME"}
			return output.PrintTable(cmd.OutOrStdout(), headers, incidentTeamTableRows(items))
		},
	}
}

func newIncidentTeamShowCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show incident team details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.teamsApi.GetIncidentTeam(iapi.ctx, args[0]) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident team: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentTeamDetail(cmd, d)
		},
	}
}

func newIncidentTeamCreateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an incident team",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errIncidentTeamNameRequired
			}

			attrs := datadogV2.NewIncidentTeamCreateAttributes(name)
			data := datadogV2.NewIncidentTeamCreateData(datadogV2.INCIDENTTEAMTYPE_TEAMS)
			data.SetAttributes(*attrs)
			body := datadogV2.NewIncidentTeamCreateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.teamsApi.CreateIncidentTeam(iapi.ctx, *body) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create incident team: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentTeamDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "team name (required)")
	return cmd
}

func newIncidentTeamUpdateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an incident team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") {
				return errIncidentTeamNameRequired
			}

			attrs := datadogV2.NewIncidentTeamUpdateAttributes(name)
			data := datadogV2.NewIncidentTeamUpdateData(datadogV2.INCIDENTTEAMTYPE_TEAMS)
			data.SetId(args[0])
			data.SetAttributes(*attrs)
			body := datadogV2.NewIncidentTeamUpdateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.teamsApi.UpdateIncidentTeam(iapi.ctx, args[0], *body) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update incident team: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentTeamDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "updated name (required)")
	return cmd
}

func newIncidentTeamDeleteCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an incident team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return errIncidentTeamYesRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.teamsApi.DeleteIncidentTeam(iapi.ctx, args[0]) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete incident team: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "incident team %s deleted\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
