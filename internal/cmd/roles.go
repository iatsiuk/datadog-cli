package cmd

import (
	"context"
	"fmt"
	"io"
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
	return cmd
}

func newRolesListCmd(mkAPI func() (*rolesAPI, error)) *cobra.Command {
	var filter string

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
				{"Permissions", fmt.Sprintf("%d: %s", len(perms), joinStrings(perms, ", "))},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" || f.v == "0: " {
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

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
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
