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

type rolesAPI struct {
	api *datadogV2.RolesApi
	ctx context.Context
}

func defaultRolesAPI() (*rolesAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &rolesAPI{api: datadogV2.NewRolesApi(c), ctx: ctx}, nil
}

// NewRolesCommand returns the roles cobra command group.
func NewRolesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Manage Datadog roles",
	}
	cmd.AddCommand(newRolesListCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesShowCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesCreateCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesUpdateCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesDeleteCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesListPermissionsCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesGrantPermissionCmd(defaultRolesAPI))
	cmd.AddCommand(newRolesRevokePermissionCmd(defaultRolesAPI))
	return cmd
}

func newRolesListCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var filter string
	var pageSize, pageNumber int64

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List roles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListRolesOptionalParameters()
			if filter != "" {
				opts = opts.WithFilter(filter)
			}
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}
			if cmd.Flags().Changed("page-number") {
				opts = opts.WithPageNumber(pageNumber)
			}

			resp, httpResp, err := rapi.api.ListRoles(rapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list roles: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV2.Role{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			return printRolesTable(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "filter roles by name")
	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "number of results per page")
	cmd.Flags().Int64Var(&pageNumber, "page-number", 0, "page number (0-indexed)")
	return cmd
}

func newRolesShowCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var roleID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show role details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.GetRole(rapi.ctx, roleID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get role: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			role := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), role)
			}

			attrs := role.GetAttributes()
			createdAt := ""
			if t := attrs.CreatedAt; t != nil {
				createdAt = t.UTC().Format(time.RFC3339)
			}
			userCount := ""
			if c := attrs.UserCount; c != nil {
				userCount = fmt.Sprintf("%d", *c)
			}

			perms := extractRolePermissionIDs(role)

			fields := []struct{ k, v string }{
				{"ID", role.GetId()},
				{"Name", attrs.GetName()},
				{"Users", userCount},
				{"Created", createdAt},
			}
			if len(perms) > 0 {
				fields = append(fields, struct{ k, v string }{"Permissions", fmt.Sprintf("%d: %s", len(perms), strings.Join(perms, ", "))})
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-12s %s\n", f.k+":", f.v) //nolint:errcheck
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&roleID, "id", "", "role ID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newRolesCreateCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewRoleCreateAttributes(name)
			data := datadogV2.NewRoleCreateData(*attrs)
			body := datadogV2.NewRoleCreateRequest(*data)

			resp, httpResp, err := rapi.api.CreateRole(rapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create role: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			role := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), role)
			}

			attrs2 := role.GetAttributes()
			fmt.Fprintf(cmd.OutOrStdout(), "Created role: %s (%s)\n", attrs2.GetName(), role.GetId()) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "role name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newRolesUpdateCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var roleID, name string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewRoleUpdateAttributes()
			attrs.SetName(name)
			data := datadogV2.NewRoleUpdateData(*attrs, roleID, datadogV2.ROLESTYPE_ROLES)
			body := datadogV2.NewRoleUpdateRequest(*data)

			resp, httpResp, err := rapi.api.UpdateRole(rapi.ctx, roleID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update role: %w", err)
			}

			role := resp.GetData()
			attrs2 := role.GetAttributes()
			fmt.Fprintf(cmd.OutOrStdout(), "Updated role: %s (%s)\n", attrs2.GetName(), role.GetId()) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&roleID, "id", "", "role ID")
	cmd.Flags().StringVar(&name, "name", "", "new role name")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newRolesDeleteCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var roleID string
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm deleting role %s", roleID)
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := rapi.api.DeleteRole(rapi.ctx, roleID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete role: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted role: %s\n", roleID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&roleID, "id", "", "role ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newRolesListPermissionsCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var roleID string

	cmd := &cobra.Command{
		Use:   "list-permissions",
		Short: "List permissions for a role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.ListRolePermissions(rapi.ctx, roleID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list role permissions: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV2.Permission{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			return printPermissionsTable(cmd.OutOrStdout(), data)
		},
	}

	cmd.Flags().StringVar(&roleID, "role-id", "", "role ID")
	_ = cmd.MarkFlagRequired("role-id")
	return cmd
}

func newRolesGrantPermissionCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var roleID, permissionID string

	cmd := &cobra.Command{
		Use:   "grant-permission",
		Short: "Grant a permission to a role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			data := datadogV2.NewRelationshipToPermissionData()
			data.SetId(permissionID)
			body := datadogV2.NewRelationshipToPermission()
			body.SetData(*data)

			_, httpResp, err := rapi.api.AddPermissionToRole(rapi.ctx, roleID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("grant permission: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Granted permission %s to role %s\n", permissionID, roleID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&roleID, "role-id", "", "role ID")
	cmd.Flags().StringVar(&permissionID, "permission-id", "", "permission ID")
	_ = cmd.MarkFlagRequired("role-id")
	_ = cmd.MarkFlagRequired("permission-id")
	return cmd
}

func newRolesRevokePermissionCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var roleID, permissionID string
	var yes bool

	cmd := &cobra.Command{
		Use:   "revoke-permission",
		Short: "Revoke a permission from a role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("pass --yes to confirm revoking permission %s from role %s", permissionID, roleID)
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			data := datadogV2.NewRelationshipToPermissionData()
			data.SetId(permissionID)
			body := datadogV2.NewRelationshipToPermission()
			body.SetData(*data)

			_, httpResp, err := rapi.api.RemovePermissionFromRole(rapi.ctx, roleID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("revoke permission: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Revoked permission %s from role %s\n", permissionID, roleID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&roleID, "role-id", "", "role ID")
	cmd.Flags().StringVar(&permissionID, "permission-id", "", "permission ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm revocation")
	_ = cmd.MarkFlagRequired("role-id")
	_ = cmd.MarkFlagRequired("permission-id")
	return cmd
}

func printPermissionsTable(w io.Writer, data []datadogV2.Permission) error {
	headers := []string{"ID", "NAME", "GROUP_NAME", "DESCRIPTION"}
	var rows [][]string
	for _, p := range data {
		attrs := p.GetAttributes()
		rows = append(rows, []string{
			p.GetId(),
			attrs.GetName(),
			attrs.GetGroupName(),
			attrs.GetDescription(),
		})
	}
	return output.PrintTable(w, headers, rows)
}

func extractRolePermissionIDs(r datadogV2.Role) []string {
	rel := r.GetRelationships()
	perms := rel.GetPermissions()
	data := perms.GetData()
	ids := make([]string, 0, len(data))
	for _, p := range data {
		ids = append(ids, p.GetId())
	}
	return ids
}

func printRolesTable(w io.Writer, data []datadogV2.Role) error {
	headers := []string{"ID", "NAME", "USER_COUNT", "CREATED_AT"}
	var rows [][]string
	for _, r := range data {
		attrs := r.GetAttributes()
		createdAt := ""
		if t := attrs.CreatedAt; t != nil {
			createdAt = t.UTC().Format(time.RFC3339)
		}
		userCount := ""
		if c := attrs.UserCount; c != nil {
			userCount = fmt.Sprintf("%d", *c)
		}
		rows = append(rows, []string{
			r.GetId(),
			attrs.GetName(),
			userCount,
			createdAt,
		})
	}
	return output.PrintTable(w, headers, rows)
}
