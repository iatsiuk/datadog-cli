package cmd

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsRestrictionQueryAPI struct {
	api *datadogV2.LogsRestrictionQueriesApi
	ctx context.Context
}

func defaultLogsRestrictionQueryAPI() (*logsRestrictionQueryAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	apiCfg := c.GetConfig()
	for _, op := range []string{
		"v2.ListRestrictionQueries",
		"v2.GetRestrictionQuery",
		"v2.CreateRestrictionQuery",
		"v2.UpdateRestrictionQuery",
		"v2.DeleteRestrictionQuery",
	} {
		apiCfg.SetUnstableOperationEnabled(op, true)
	}
	return &logsRestrictionQueryAPI{api: datadogV2.NewLogsRestrictionQueriesApi(c), ctx: ctx}, nil
}

func newLogsRestrictionQueryCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restriction-query",
		Short: "Manage log restriction queries",
	}
	cmd.AddCommand(newLogsRQListCmd(mkAPI))
	cmd.AddCommand(newLogsRQShowCmd(mkAPI))
	cmd.AddCommand(newLogsRQCreateCmd(mkAPI))
	cmd.AddCommand(newLogsRQUpdateCmd(mkAPI))
	cmd.AddCommand(newLogsRQDeleteCmd(mkAPI))
	return cmd
}

func newLogsRQListCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List restriction queries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.ListRestrictionQueries(rapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list restriction queries: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.RestrictionQueryWithoutRelationships{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "QUERY", "ROLES", "USERS"}
			var rows [][]string
			for _, rq := range resp.GetData() {
				id := rq.GetId()
				q, roles, users := restrictionQueryFields(rq.Attributes)
				rows = append(rows, []string{id, q, roles, users})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsRQShowCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a restriction query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.GetRestrictionQuery(rapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get restriction query: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			d := resp.GetData()
			id := d.GetId()
			q, roles, users := restrictionQueryFields(d.Attributes)
			headers := []string{"ID", "QUERY", "ROLES", "USERS"}
			rows := [][]string{{id, q, roles, users}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsRQCreateCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a restriction query",
		RunE: func(cmd *cobra.Command, _ []string) error {
			attrs := datadogV2.NewRestrictionQueryCreateAttributes(query)
			data := datadogV2.NewRestrictionQueryCreateData()
			data.SetAttributes(*attrs)

			body := datadogV2.NewRestrictionQueryCreatePayload()
			body.SetData(*data)

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.CreateRestrictionQuery(rapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create restriction query: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "restriction query (required)")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func newLogsRQUpdateCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a restriction query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attrs := datadogV2.NewRestrictionQueryUpdateAttributes(query)
			data := datadogV2.NewRestrictionQueryUpdateData()
			data.SetAttributes(*attrs)

			body := datadogV2.NewRestrictionQueryUpdatePayload()
			body.SetData(*data)

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := rapi.api.UpdateRestrictionQuery(rapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update restriction query: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "restriction query (required)")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func newLogsRQDeleteCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a restriction query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of restriction query %q", args[0])
			}

			rapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := rapi.api.DeleteRestrictionQuery(rapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete restriction query: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted restriction query %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

// restrictionQueryFields extracts query, role count, and user count from attributes.
func restrictionQueryFields(attrs *datadogV2.RestrictionQueryAttributes) (query, roles, users string) {
	if attrs == nil {
		return
	}
	query = attrs.GetRestrictionQuery()
	if rc := attrs.RoleCount; rc != nil {
		roles = fmt.Sprintf("%d", *rc)
	}
	if uc := attrs.UserCount; uc != nil {
		users = fmt.Sprintf("%d", *uc)
	}
	return
}
