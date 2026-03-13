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
	cmd.AddCommand(newTeamsShowCmd(defaultTeamsAPI))
	cmd.AddCommand(newTeamsCreateCmd(defaultTeamsAPI))
	cmd.AddCommand(newTeamsUpdateCmd(defaultTeamsAPI))
	cmd.AddCommand(newTeamsDeleteCmd(defaultTeamsAPI))
	cmd.AddCommand(newTeamsMembersCmd(defaultTeamsAPI))
	cmd.AddCommand(newTeamsAddMemberCmd(defaultTeamsAPI))
	cmd.AddCommand(newTeamsRemoveMemberCmd(defaultTeamsAPI))
	return cmd
}

func newTeamsListCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var filter string
	var pageSize, pageNumber int64

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
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}
			if cmd.Flags().Changed("page-number") {
				opts = opts.WithPageNumber(pageNumber)
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
	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "number of results per page")
	cmd.Flags().Int64Var(&pageNumber, "page-number", 0, "page number (0-indexed)")
	return cmd
}

func newTeamsShowCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var teamID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show team details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := tapi.api.GetTeam(tapi.ctx, teamID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get team: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			team := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), team)
			}

			attrs := team.GetAttributes()
			userCount := ""
			if c := attrs.UserCount; c != nil {
				userCount = fmt.Sprintf("%d", *c)
			}
			fields := []struct{ k, v string }{
				{"ID", team.GetId()},
				{"Name", attrs.GetName()},
				{"Handle", attrs.GetHandle()},
				{"Description", attrs.GetDescription()},
				{"User Count", userCount},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-12s %s\n", f.k+":", f.v) //nolint:errcheck
			}

			membResp, membHTTPResp, err := tapi.api.GetTeamMemberships(tapi.ctx, teamID, *datadogV2.NewGetTeamMembershipsOptionalParameters())
			if membHTTPResp != nil {
				_ = membHTTPResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get team memberships: %w", err)
			}
			fmt.Fprintln(w) //nolint:errcheck
			return printTeamMembersTable(w, membResp.GetData())
		},
	}

	cmd.Flags().StringVar(&teamID, "id", "", "team ID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTeamsCreateCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var name, handle, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new team",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewTeamCreateAttributes(handle, name)
			if description != "" {
				attrs.SetDescription(description)
			}
			data := datadogV2.NewTeamCreate(*attrs, datadogV2.TEAMTYPE_TEAM)
			body := datadogV2.NewTeamCreateRequest(*data)

			resp, httpResp, err := tapi.api.CreateTeam(tapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create team: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			team := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), team)
			}

			teamAttrs := team.GetAttributes()
			fmt.Fprintf(cmd.OutOrStdout(), "Created team: %s (%s)\n", teamAttrs.GetName(), team.GetId()) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "team name")
	cmd.Flags().StringVar(&handle, "handle", "", "team handle (unique, lowercase alphanumeric + hyphens)")
	cmd.Flags().StringVar(&description, "description", "", "team description")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("handle")
	return cmd
}

func newTeamsUpdateCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var teamID, name, handle, description string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a team",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" && handle == "" && !cmd.Flags().Changed("description") {
				return fmt.Errorf("at least one of --name, --handle, or --description is required")
			}

			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			// fetch current values to satisfy required fields in update
			getResp, httpResp, err := tapi.api.GetTeam(tapi.ctx, teamID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get team: %w", err)
			}

			currentTeam := getResp.GetData()
			current := currentTeam.GetAttributes()
			updName := current.GetName()
			updHandle := current.GetHandle()
			updDescription := current.GetDescription()
			if name != "" {
				updName = name
			}
			if handle != "" {
				updHandle = handle
			}
			if cmd.Flags().Changed("description") {
				updDescription = description
			}

			attrs := datadogV2.NewTeamUpdateAttributes(updHandle, updName)
			attrs.SetDescription(updDescription)
			data := datadogV2.NewTeamUpdate(*attrs, datadogV2.TEAMTYPE_TEAM)
			body := datadogV2.NewTeamUpdateRequest(*data)

			resp, httpResp2, err := tapi.api.UpdateTeam(tapi.ctx, teamID, *body)
			if httpResp2 != nil {
				_ = httpResp2.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update team: %w", err)
			}

			team := resp.GetData()
			updAttrs := team.GetAttributes()
			fmt.Fprintf(cmd.OutOrStdout(), "Updated team: %s (%s)\n", updAttrs.GetName(), team.GetId()) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&teamID, "id", "", "team ID")
	cmd.Flags().StringVar(&name, "name", "", "team name")
	cmd.Flags().StringVar(&handle, "handle", "", "team handle")
	cmd.Flags().StringVar(&description, "description", "", "team description")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTeamsDeleteCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var teamID string
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a team",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deleting team %s", teamID)
			}

			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := tapi.api.DeleteTeam(tapi.ctx, teamID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete team: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted team: %s\n", teamID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&teamID, "id", "", "team ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deleting the team")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTeamsMembersCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var teamID string
	var pageSize, pageNumber int64

	cmd := &cobra.Command{
		Use:   "members",
		Short: "List team members",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewGetTeamMembershipsOptionalParameters()
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}
			if cmd.Flags().Changed("page-number") {
				opts = opts.WithPageNumber(pageNumber)
			}

			resp, httpResp, err := tapi.api.GetTeamMemberships(tapi.ctx, teamID, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get team memberships: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV2.UserTeam{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			return printTeamMembersTable(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().StringVar(&teamID, "id", "", "team ID")
	_ = cmd.MarkFlagRequired("id")
	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "number of results per page")
	cmd.Flags().Int64Var(&pageNumber, "page-number", 0, "page number (0-indexed)")
	return cmd
}

func newTeamsAddMemberCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var teamID, userID, role string

	cmd := &cobra.Command{
		Use:   "add-member",
		Short: "Add a member to a team",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			userData := datadogV2.NewRelationshipToUserTeamUserData(userID, datadogV2.USERTEAMUSERTYPE_USERS)
			userRel := datadogV2.NewRelationshipToUserTeamUser(*userData)
			rels := datadogV2.NewUserTeamRelationships()
			rels.SetUser(*userRel)

			data := datadogV2.NewUserTeamCreate(datadogV2.USERTEAMTYPE_TEAM_MEMBERSHIPS)
			data.SetRelationships(*rels)

			if role != "" {
				roleVal, rerr := datadogV2.NewUserTeamRoleFromValue(role)
				if rerr != nil {
					return fmt.Errorf("invalid role %q: must be 'admin' (omit flag for regular member)", role)
				}
				attrs := datadogV2.NewUserTeamAttributes()
				attrs.SetRole(*roleVal)
				data.SetAttributes(*attrs)
			}

			body := datadogV2.NewUserTeamRequest(*data)

			_, httpResp, err := tapi.api.CreateTeamMembership(tapi.ctx, teamID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("add team member: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added user %s to team %s\n", userID, teamID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&teamID, "id", "", "team ID")
	cmd.Flags().StringVar(&userID, "user-id", "", "user ID to add")
	cmd.Flags().StringVar(&role, "role", "", "member role (admin or leave empty for member)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}

func newTeamsRemoveMemberCmd(mkAPI func() (*teamsAPI, error)) *cobra.Command {
	var teamID, userID string
	var yes bool

	cmd := &cobra.Command{
		Use:   "remove-member",
		Short: "Remove a member from a team",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm removing user %s from team %s", userID, teamID)
			}

			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := tapi.api.DeleteTeamMembership(tapi.ctx, teamID, userID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("remove team member: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed user %s from team %s\n", userID, teamID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&teamID, "id", "", "team ID")
	cmd.Flags().StringVar(&userID, "user-id", "", "user ID to remove")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm removal")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("user-id")
	return cmd
}

func printTeamMembersTable(w io.Writer, data []datadogV2.UserTeam) error {
	headers := []string{"MEMBERSHIP_ID", "USER_ID", "ROLE"}
	var rows [][]string
	for _, m := range data {
		role := "member"
		attrs := m.GetAttributes()
		if r, ok := attrs.GetRoleOk(); ok && r != nil {
			role = string(*r)
		}
		rels := m.GetRelationships()
		userRel := rels.GetUser()
		userData := userRel.GetData()
		userID := userData.GetId()
		rows = append(rows, []string{
			m.GetId(),
			userID,
			role,
		})
	}
	return output.PrintTable(w, headers, rows)
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
