package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type usersAPI struct {
	api *datadogV2.UsersApi
	ctx context.Context
}

func defaultUsersAPI() (*usersAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &usersAPI{api: datadogV2.NewUsersApi(c), ctx: ctx}, nil
}

// NewUsersCommand returns the users cobra command group.
func NewUsersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage Datadog users, roles, and teams",
	}
	cmd.AddCommand(newUsersListCmd(defaultUsersAPI))
	cmd.AddCommand(newUsersShowCmd(defaultUsersAPI))
	cmd.AddCommand(newUsersCreateCmd(defaultUsersAPI))
	cmd.AddCommand(newUsersInviteCmd(defaultUsersAPI))
	cmd.AddCommand(newUsersUpdateCmd(defaultUsersAPI))
	cmd.AddCommand(newUsersDisableCmd(defaultUsersAPI))
	cmd.AddCommand(NewRolesCommand())
	cmd.AddCommand(NewTeamsCommand())
	return cmd
}

func newUsersListCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var filter string
	var pageSize, pageNumber int64

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			uapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListUsersOptionalParameters()
			if filter != "" {
				opts = opts.WithFilter(filter)
			}
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}
			if cmd.Flags().Changed("page-number") {
				opts = opts.WithPageNumber(pageNumber)
			}

			resp, httpResp, err := uapi.api.ListUsers(uapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list users: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV2.User{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			return printUsersTable(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "filter users by email or name")
	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "number of results per page")
	cmd.Flags().Int64Var(&pageNumber, "page-number", 0, "page number (0-indexed)")
	return cmd
}

func newUsersShowCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var userID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show user details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			uapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := uapi.api.GetUser(uapi.ctx, userID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get user: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			user := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), user)
			}

			attrs := user.GetAttributes()
			createdAt := ""
			if t := attrs.CreatedAt; t != nil {
				createdAt = t.UTC().Format(time.RFC3339)
			}
			roles := extractUserRoleIDs(user)

			fields := []struct{ k, v string }{
				{"ID", user.GetId()},
				{"Email", attrs.GetEmail()},
				{"Name", attrs.GetName()},
				{"Handle", attrs.GetHandle()},
				{"Status", attrs.GetStatus()},
				{"Title", attrs.GetTitle()},
				{"Roles", strings.Join(roles, ", ")},
				{"Created", createdAt},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-10s %s\n", f.k+":", f.v) //nolint:errcheck
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "id", "", "user ID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newUsersCreateCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var email, name, title string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			uapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewUserCreateAttributes(email)
			if name != "" {
				attrs.SetName(name)
			}
			if title != "" {
				attrs.SetTitle(title)
			}
			data := datadogV2.NewUserCreateData(*attrs, datadogV2.USERSTYPE_USERS)
			body := datadogV2.NewUserCreateRequest(*data)

			resp, httpResp, err := uapi.api.CreateUser(uapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			user := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), user)
			}

			attrs2 := user.GetAttributes()
			fmt.Fprintf(cmd.OutOrStdout(), "Created user: %s (%s)\n", attrs2.GetEmail(), user.GetId()) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "user email address")
	cmd.Flags().StringVar(&name, "name", "", "user display name")
	cmd.Flags().StringVar(&title, "title", "", "user title")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newUsersInviteCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var userID string

	cmd := &cobra.Command{
		Use:   "invite",
		Short: "Send invitation email to a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			uapi, err := mkAPI()
			if err != nil {
				return err
			}

			userData := datadogV2.NewRelationshipToUserData(userID, datadogV2.USERSTYPE_USERS)
			rel := datadogV2.NewRelationshipToUser(*userData)
			invRel := datadogV2.NewUserInvitationRelationships(*rel)
			invData := datadogV2.NewUserInvitationData(*invRel, datadogV2.USERINVITATIONSTYPE_USER_INVITATIONS)
			body := datadogV2.NewUserInvitationsRequest([]datadogV2.UserInvitationData{*invData})

			resp, httpResp, err := uapi.api.SendInvitations(uapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("send invitations: %w", err)
			}

			w := cmd.OutOrStdout()
			for _, inv := range resp.GetData() {
				fmt.Fprintf(w, "Invitation sent: %s\n", inv.GetId()) //nolint:errcheck
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "id", "", "user ID to invite")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newUsersUpdateCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var userID, name, email string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" && email == "" {
				return fmt.Errorf("at least one of --name or --email is required")
			}

			uapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewUserUpdateAttributes()
			if name != "" {
				attrs.SetName(name)
			}
			if email != "" {
				attrs.SetEmail(email)
			}
			data := datadogV2.NewUserUpdateData(*attrs, userID, datadogV2.USERSTYPE_USERS)
			body := datadogV2.NewUserUpdateRequest(*data)

			resp, httpResp, err := uapi.api.UpdateUser(uapi.ctx, userID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update user: %w", err)
			}

			user := resp.GetData()
			attrs2 := user.GetAttributes()
			fmt.Fprintf(cmd.OutOrStdout(), "Updated user: %s (%s)\n", attrs2.GetEmail(), user.GetId()) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "id", "", "user ID")
	cmd.Flags().StringVar(&name, "name", "", "user display name")
	cmd.Flags().StringVar(&email, "email", "", "user email address")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newUsersDisableCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var userID string
	var yes bool

	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm disabling user %s", userID)
			}

			uapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := uapi.api.DisableUser(uapi.ctx, userID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("disable user: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Disabled user: %s\n", userID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "id", "", "user ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm disabling the user")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func printUsersTable(w io.Writer, data []datadogV2.User) error {
	headers := []string{"ID", "EMAIL", "NAME", "HANDLE", "STATUS", "ROLES", "CREATED_AT"}
	var rows [][]string
	for _, u := range data {
		attrs := u.GetAttributes()
		createdAt := ""
		if t := attrs.CreatedAt; t != nil {
			createdAt = t.UTC().Format(time.RFC3339)
		}
		roles := extractUserRoleIDs(u)
		rows = append(rows, []string{
			u.GetId(),
			attrs.GetEmail(),
			attrs.GetName(),
			attrs.GetHandle(),
			attrs.GetStatus(),
			strings.Join(roles, ","),
			createdAt,
		})
	}
	return output.PrintTable(w, headers, rows)
}

func extractUserRoleIDs(u datadogV2.User) []string {
	rel := u.GetRelationships()
	roles := rel.GetRoles()
	data := roles.GetData()
	ids := make([]string, 0, len(data))
	for _, r := range data {
		ids = append(ids, r.GetId())
	}
	return ids
}
