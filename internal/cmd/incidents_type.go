package cmd

import (
	"errors"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errIncidentTypeNameRequired = errors.New("--name is required")
	errIncidentTypeYesRequired  = errors.New("--yes is required to delete an incident type")
)

func newIncidentsTypeCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "type",
		Short: "Manage incident types",
	}
	cmd.AddCommand(newIncidentTypeListCmd(mkAPI))
	cmd.AddCommand(newIncidentTypeShowCmd(mkAPI))
	cmd.AddCommand(newIncidentTypeCreateCmd(mkAPI))
	cmd.AddCommand(newIncidentTypeUpdateCmd(mkAPI))
	cmd.AddCommand(newIncidentTypeDeleteCmd(mkAPI))
	return cmd
}

func incidentTypeTableRows(items []datadogV2.IncidentTypeObject) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		attrs := item.GetAttributes()
		desc := ""
		if d, ok := attrs.GetDescriptionOk(); ok && d != nil {
			desc = *d
		}
		isDefault := "false"
		if attrs.GetIsDefault() {
			isDefault = "true"
		}
		rows = append(rows, []string{item.GetId(), attrs.GetName(), desc, isDefault})
	}
	return rows
}

func printIncidentTypeDetail(cmd *cobra.Command, d datadogV2.IncidentTypeObject) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"NAME", attrs.GetName()},
	}
	if desc, ok := attrs.GetDescriptionOk(); ok && desc != nil {
		rows = append(rows, []string{"DESCRIPTION", *desc})
	}
	isDefault := "false"
	if attrs.GetIsDefault() {
		isDefault = "true"
	}
	rows = append(rows, []string{"IS_DEFAULT", isDefault})
	if t := attrs.GetCreatedAt(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newIncidentTypeListCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List incident types",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.ListIncidentTypes(iapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list incident types: %w", err)
			}

			items := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if items == nil {
					items = []datadogV2.IncidentTypeObject{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), items)
			}

			headers := []string{"ID", "NAME", "DESCRIPTION", "IS_DEFAULT"}
			return output.PrintTable(cmd.OutOrStdout(), headers, incidentTypeTableRows(items))
		},
	}
}

func newIncidentTypeShowCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show incident type details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.GetIncidentType(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident type: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentTypeDetail(cmd, d)
		},
	}
}

func newIncidentTypeCreateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		name        string
		description string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an incident type",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errIncidentTypeNameRequired
			}

			attrs := datadogV2.NewIncidentTypeAttributes(name)
			if cmd.Flags().Changed("description") {
				attrs.SetDescription(description)
			}

			data := datadogV2.NewIncidentTypeCreateData(
				*attrs,
				datadogV2.INCIDENTTYPETYPE_INCIDENT_TYPES,
			)
			body := datadogV2.NewIncidentTypeCreateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.CreateIncidentType(iapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create incident type: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentTypeDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "incident type name (required)")
	cmd.Flags().StringVar(&description, "description", "", "incident type description")
	return cmd
}

func newIncidentTypeUpdateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		name        string
		description string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an incident type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attrs := datadogV2.NewIncidentTypeUpdateAttributes()
			if cmd.Flags().Changed("name") {
				attrs.SetName(name)
			}
			if cmd.Flags().Changed("description") {
				attrs.SetDescription(description)
			}

			data := datadogV2.NewIncidentTypePatchData(
				*attrs,
				args[0],
				datadogV2.INCIDENTTYPETYPE_INCIDENT_TYPES,
			)
			body := datadogV2.NewIncidentTypePatchRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.UpdateIncidentType(iapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update incident type: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentTypeDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "updated name")
	cmd.Flags().StringVar(&description, "description", "", "updated description")
	return cmd
}

func newIncidentTypeDeleteCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an incident type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return errIncidentTypeYesRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.api.DeleteIncidentType(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete incident type: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "incident type %s deleted\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
