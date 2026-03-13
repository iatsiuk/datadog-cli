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
