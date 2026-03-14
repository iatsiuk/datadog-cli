package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errSyntheticsVariableNameRequired = errors.New("--name is required")

func newSyntheticsVariableCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable",
		Short: "Manage Synthetic global variables",
	}
	cmd.AddCommand(newSyntheticsVariableListCmd(mkAPI))
	cmd.AddCommand(newSyntheticsVariableShowCmd(mkAPI))
	cmd.AddCommand(newSyntheticsVariableCreateCmd(mkAPI))
	cmd.AddCommand(newSyntheticsVariableUpdateCmd(mkAPI))
	cmd.AddCommand(newSyntheticsVariableDeleteCmd(mkAPI))
	return cmd
}

func synthVariableTableOutput(cmd *cobra.Command, vars []datadogV1.SyntheticsGlobalVariable) error {
	headers := []string{"ID", "NAME", "DESCRIPTION", "SECURE", "TAGS"}
	rows := make([][]string, 0, len(vars))
	for _, v := range vars {
		secure := "false"
		if val := v.GetValue(); val.Secure != nil && *val.Secure {
			secure = "true"
		}
		rows = append(rows, []string{
			v.GetId(),
			v.GetName(),
			v.GetDescription(),
			secure,
			strings.Join(v.GetTags(), ","),
		})
	}
	return output.PrintTable(cmd.OutOrStdout(), headers, rows)
}

func newSyntheticsVariableListCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Synthetic global variables",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.ListGlobalVariables(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list global variables: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			vars := resp.GetVariables()

			if asJSON {
				if vars == nil {
					vars = []datadogV1.SyntheticsGlobalVariable{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), vars)
			}

			return synthVariableTableOutput(cmd, vars)
		},
	}
	return cmd
}

func newSyntheticsVariableShowCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a Synthetic global variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			varID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			v, httpResp, err := sapi.api.GetGlobalVariable(sapi.ctx, varID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get global variable: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), v)
			}

			return synthVariableDetailOutput(cmd, v)
		},
	}
	return cmd
}

func synthVariableDetailOutput(cmd *cobra.Command, v datadogV1.SyntheticsGlobalVariable) error {
	secure := "false"
	value := ""
	if val := v.GetValue(); val.Secure != nil && *val.Secure {
		secure = "true"
	} else if val := v.GetValue(); val.Value != nil {
		value = *val.Value
	}
	rows := [][]string{
		{"ID", v.GetId()},
		{"NAME", v.GetName()},
		{"DESCRIPTION", v.GetDescription()},
		{"SECURE", secure},
		{"VALUE", value},
		{"TAGS", strings.Join(v.GetTags(), ", ")},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newSyntheticsVariableCreateCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		name        string
		value       string
		description string
		tags        string
		secure      bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Synthetic global variable",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errSyntheticsVariableNameRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			tagList := []string{}
			if tags != "" {
				tagList = splitTrimmed(tags)
			}

			req := datadogV1.NewSyntheticsGlobalVariableRequest(description, name, tagList)
			val := datadogV1.NewSyntheticsGlobalVariableValue()
			val.SetSecure(secure)
			if value != "" {
				val.SetValue(value)
			}
			req.SetValue(*val)

			v, httpResp, err := sapi.api.CreateGlobalVariable(sapi.ctx, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create global variable: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), v)
			}

			return synthVariableDetailOutput(cmd, v)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "variable name (required)")
	cmd.Flags().StringVar(&value, "value", "", "variable value")
	cmd.Flags().StringVar(&description, "description", "", "variable description")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	cmd.Flags().BoolVar(&secure, "secure", false, "hide variable value")
	return cmd
}

func newSyntheticsVariableUpdateCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		name        string
		value       string
		description string
		tags        string
		secure      bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a Synthetic global variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			varID := args[0]
			if name == "" {
				return errSyntheticsVariableNameRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			tagList := []string{}
			if tags != "" {
				tagList = splitTrimmed(tags)
			}

			req := datadogV1.NewSyntheticsGlobalVariableRequest(description, name, tagList)
			val := datadogV1.NewSyntheticsGlobalVariableValue()
			val.SetSecure(secure)
			if value != "" {
				val.SetValue(value)
			}
			req.SetValue(*val)

			v, httpResp, err := sapi.api.EditGlobalVariable(sapi.ctx, varID, *req)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update global variable: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), v)
			}

			return synthVariableDetailOutput(cmd, v)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "variable name")
	cmd.Flags().StringVar(&value, "value", "", "variable value")
	cmd.Flags().StringVar(&description, "description", "", "variable description")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	cmd.Flags().BoolVar(&secure, "secure", false, "hide variable value")
	return cmd
}

func newSyntheticsVariableDeleteCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a Synthetic global variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			varID := args[0]
			if !yes {
				return errSyntheticsYesRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := sapi.api.DeleteGlobalVariable(sapi.ctx, varID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete global variable: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deleted variable %s\n", varID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
