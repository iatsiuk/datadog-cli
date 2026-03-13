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
	return cmd
}

func newUsersListCmd(mkAPI func() (*usersAPI, error)) *cobra.Command {
	var filter string

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
