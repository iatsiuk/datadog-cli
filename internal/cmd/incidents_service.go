package cmd

import (
	"errors"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errIncidentServiceNameRequired = errors.New("--name is required")
	errIncidentServiceYesRequired  = errors.New("--yes is required to delete an incident service")
)

func newIncidentsServiceCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage incident services",
	}
	cmd.AddCommand(newIncidentServiceListCmd(mkAPI))
	cmd.AddCommand(newIncidentServiceShowCmd(mkAPI))
	cmd.AddCommand(newIncidentServiceCreateCmd(mkAPI))
	cmd.AddCommand(newIncidentServiceUpdateCmd(mkAPI))
	cmd.AddCommand(newIncidentServiceDeleteCmd(mkAPI))
	return cmd
}

func incidentServiceTableRows(items []datadogV2.IncidentServiceResponseData) [][]string {
	rows := make([][]string, 0, len(items))
	for i := range items {
		attrs := items[i].GetAttributes()
		rows = append(rows, []string{items[i].GetId(), attrs.GetName()})
	}
	return rows
}

func printIncidentServiceDetail(cmd *cobra.Command, d datadogV2.IncidentServiceResponseData) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"NAME", attrs.GetName()},
	}
	if t := attrs.GetCreated(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newIncidentServiceListCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List incident services",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.servicesApi.ListIncidentServices(iapi.ctx) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list incident services: %w", err)
			}

			items := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if items == nil {
					items = []datadogV2.IncidentServiceResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), items)
			}

			headers := []string{"ID", "NAME"}
			return output.PrintTable(cmd.OutOrStdout(), headers, incidentServiceTableRows(items))
		},
	}
}

func newIncidentServiceShowCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show incident service details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.servicesApi.GetIncidentService(iapi.ctx, args[0]) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident service: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentServiceDetail(cmd, d)
		},
	}
}

func newIncidentServiceCreateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an incident service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errIncidentServiceNameRequired
			}

			attrs := datadogV2.NewIncidentServiceCreateAttributes(name)
			data := datadogV2.NewIncidentServiceCreateData(datadogV2.INCIDENTSERVICETYPE_SERVICES)
			data.SetAttributes(*attrs)
			body := datadogV2.NewIncidentServiceCreateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.servicesApi.CreateIncidentService(iapi.ctx, *body) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create incident service: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentServiceDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "service name (required)")
	return cmd
}

func newIncidentServiceUpdateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an incident service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") {
				return errIncidentServiceNameRequired
			}

			attrs := datadogV2.NewIncidentServiceUpdateAttributes(name)
			data := datadogV2.NewIncidentServiceUpdateData(datadogV2.INCIDENTSERVICETYPE_SERVICES)
			data.SetId(args[0])
			data.SetAttributes(*attrs)
			body := datadogV2.NewIncidentServiceUpdateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.servicesApi.UpdateIncidentService(iapi.ctx, args[0], *body) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update incident service: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentServiceDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "updated name (required)")
	return cmd
}

func newIncidentServiceDeleteCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an incident service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return errIncidentServiceYesRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.servicesApi.DeleteIncidentService(iapi.ctx, args[0]) //nolint:staticcheck
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete incident service: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "incident service %s deleted\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
