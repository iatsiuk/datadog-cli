package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsCustomDestAPI struct {
	api *datadogV2.LogsCustomDestinationsApi
	ctx context.Context
}

func defaultLogsCustomDestAPI() (*logsCustomDestAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &logsCustomDestAPI{api: datadogV2.NewLogsCustomDestinationsApi(c), ctx: ctx}, nil
}

func newLogsCustomDestCmd(mkAPI func() (*logsCustomDestAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom-destination",
		Short: "Manage log custom destinations",
	}
	cmd.AddCommand(newLogsCustomDestListCmd(mkAPI))
	cmd.AddCommand(newLogsCustomDestShowCmd(mkAPI))
	cmd.AddCommand(newLogsCustomDestCreateCmd(mkAPI))
	cmd.AddCommand(newLogsCustomDestUpdateCmd(mkAPI))
	cmd.AddCommand(newLogsCustomDestDeleteCmd(mkAPI))
	return cmd
}

func newLogsCustomDestListCmd(mkAPI func() (*logsCustomDestAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List custom destinations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.ListLogsCustomDestinations(dapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list custom destinations: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.CustomDestinationResponseDefinition{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "QUERY", "ENABLED"}
			var rows [][]string
			for _, d := range resp.GetData() {
				id := d.GetId()
				name, query, enabled := customDestFields(d.Attributes)
				rows = append(rows, []string{id, name, query, enabled})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsCustomDestShowCmd(mkAPI func() (*logsCustomDestAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a custom destination",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.GetLogsCustomDestination(dapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get custom destination: %w", err)
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
			name, query, enabled := customDestFields(d.Attributes)
			headers := []string{"ID", "NAME", "QUERY", "ENABLED"}
			rows := [][]string{{id, name, query, enabled}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsCustomDestCreateCmd(mkAPI func() (*logsCustomDestAPI, error)) *cobra.Command {
	var (
		name     string
		url      string
		username string
		query    string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a custom destination",
		RunE: func(cmd *cobra.Command, _ []string) error {
			password := os.Getenv("DD_LOGS_DEST_PASSWORD")
			if password == "" {
				return fmt.Errorf("password must be provided via DD_LOGS_DEST_PASSWORD env var")
			}
			auth := datadogV2.CustomDestinationHttpDestinationAuthBasicAsCustomDestinationHttpDestinationAuth(
				datadogV2.NewCustomDestinationHttpDestinationAuthBasic(
					password,
					datadogV2.CUSTOMDESTINATIONHTTPDESTINATIONAUTHBASICTYPE_BASIC,
					username,
				),
			)
			httpDest := datadogV2.NewCustomDestinationForwardDestinationHttp(
				auth,
				url,
				datadogV2.CUSTOMDESTINATIONFORWARDDESTINATIONHTTPTYPE_HTTP,
			)
			dest := datadogV2.CustomDestinationForwardDestinationHttpAsCustomDestinationForwardDestination(httpDest)

			attrs := datadogV2.NewCustomDestinationCreateRequestAttributes(dest, name)
			if query != "" {
				attrs.SetQuery(query)
			}

			def := datadogV2.NewCustomDestinationCreateRequestDefinition(
				*attrs,
				datadogV2.CUSTOMDESTINATIONTYPE_CUSTOM_DESTINATION,
			)
			body := datadogV2.NewCustomDestinationCreateRequest()
			body.SetData(*def)

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.CreateLogsCustomDestination(dapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create custom destination: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "destination name (required)")
	cmd.Flags().StringVar(&url, "url", "", "HTTP endpoint URL (required)")
	cmd.Flags().StringVar(&username, "username", "", "basic auth username (required)")
	cmd.Flags().StringVar(&query, "query", "", "log filter query")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

func newLogsCustomDestUpdateCmd(mkAPI func() (*logsCustomDestAPI, error)) *cobra.Command {
	var (
		name  string
		query string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a custom destination",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attrs := datadogV2.NewCustomDestinationUpdateRequestAttributes()
			if name != "" {
				attrs.SetName(name)
			}
			if query != "" {
				attrs.SetQuery(query)
			}

			def := datadogV2.NewCustomDestinationUpdateRequestDefinition(
				args[0],
				datadogV2.CUSTOMDESTINATIONTYPE_CUSTOM_DESTINATION,
			)
			def.SetAttributes(*attrs)

			body := &datadogV2.CustomDestinationUpdateRequest{}
			body.SetData(*def)

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.UpdateLogsCustomDestination(dapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update custom destination: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "destination name")
	cmd.Flags().StringVar(&query, "query", "", "log filter query")
	return cmd
}

func newLogsCustomDestDeleteCmd(mkAPI func() (*logsCustomDestAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a custom destination",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of custom destination %q", args[0])
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := dapi.api.DeleteLogsCustomDestination(dapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete custom destination: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted custom destination %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

// customDestFields extracts name, query, and enabled from response attributes.
func customDestFields(attrs *datadogV2.CustomDestinationResponseAttributes) (name, query, enabled string) {
	if attrs == nil {
		return
	}
	name = attrs.GetName()
	query = attrs.GetQuery()
	if e := attrs.Enabled; e != nil {
		if *e {
			enabled = "true"
		} else {
			enabled = "false"
		}
	}
	return
}
