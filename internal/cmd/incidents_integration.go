package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errIntegrationTypeRequired     = errors.New("--type is required (slack or jira)")
	errIntegrationTypeInvalid      = errors.New("--type must be slack or jira")
	errIntegrationMetadataRequired = errors.New("--metadata is required")
	errIntegrationYesRequired      = errors.New("--yes is required to delete an integration")
)

func newIncidentsIntegrationCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integration",
		Short: "Manage incident integrations",
	}
	cmd.AddCommand(newIntegrationListCmd(mkAPI))
	cmd.AddCommand(newIntegrationShowCmd(mkAPI))
	cmd.AddCommand(newIntegrationCreateCmd(mkAPI))
	cmd.AddCommand(newIntegrationUpdateCmd(mkAPI))
	cmd.AddCommand(newIntegrationDeleteCmd(mkAPI))
	return cmd
}

func integrationTypeString(t int32) string {
	switch t {
	case 1:
		return "slack"
	case 8:
		return "jira"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

func integrationTypeToInt(typeName string) (int32, error) {
	switch typeName {
	case "slack":
		return 1, nil
	case "jira":
		return 8, nil
	default:
		return 0, errIntegrationTypeInvalid
	}
}

func integrationTableRows(items []datadogV2.IncidentIntegrationMetadataResponseData) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		attrs := item.GetAttributes()
		typeStr := integrationTypeString(attrs.GetIntegrationType())
		status := ""
		if s, ok := attrs.GetStatusOk(); ok && s != nil {
			status = fmt.Sprintf("%d", *s)
		}
		rows = append(rows, []string{item.GetId(), typeStr, status})
	}
	return rows
}

func printIntegrationDetail(cmd *cobra.Command, d datadogV2.IncidentIntegrationMetadataResponseData) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"TYPE", integrationTypeString(attrs.GetIntegrationType())},
	}
	if s, ok := attrs.GetStatusOk(); ok && s != nil {
		rows = append(rows, []string{"STATUS", fmt.Sprintf("%d", *s)})
	}
	if t := attrs.GetCreated(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func parseIntegrationMetadata(metadataJSON string) (datadogV2.IncidentIntegrationMetadataMetadata, error) {
	var meta datadogV2.IncidentIntegrationMetadataMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &meta); err != nil {
		return meta, fmt.Errorf("parse metadata: %w", err)
	}
	return meta, nil
}

func newIntegrationListCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list <incident-id>",
		Short: "List integrations for an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.ListIncidentIntegrations(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list incident integrations: %w", err)
			}

			items := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if items == nil {
					items = []datadogV2.IncidentIntegrationMetadataResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), items)
			}

			headers := []string{"ID", "TYPE", "STATUS"}
			return output.PrintTable(cmd.OutOrStdout(), headers, integrationTableRows(items))
		},
	}
}

func newIntegrationShowCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <incident-id> <integration-id>",
		Short: "Show integration details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.GetIncidentIntegration(iapi.ctx, args[0], args[1])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident integration: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIntegrationDetail(cmd, d)
		},
	}
}

func newIntegrationCreateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		typeName string
		metadata string
	)

	cmd := &cobra.Command{
		Use:   "create <incident-id>",
		Short: "Create an integration for an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if typeName == "" {
				return errIntegrationTypeRequired
			}
			if metadata == "" {
				return errIntegrationMetadataRequired
			}

			intType, err := integrationTypeToInt(typeName)
			if err != nil {
				return err
			}

			meta, err := parseIntegrationMetadata(metadata)
			if err != nil {
				return err
			}

			attrs := datadogV2.NewIncidentIntegrationMetadataAttributes(intType, meta)
			data := datadogV2.NewIncidentIntegrationMetadataCreateData(
				*attrs,
				datadogV2.INCIDENTINTEGRATIONMETADATATYPE_INCIDENT_INTEGRATIONS,
			)
			body := datadogV2.NewIncidentIntegrationMetadataCreateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.CreateIncidentIntegration(iapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create incident integration: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIntegrationDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&typeName, "type", "", "integration type: slack or jira (required)")
	cmd.Flags().StringVar(&metadata, "metadata", "", "integration metadata as JSON (required)")
	return cmd
}

func newIntegrationUpdateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var metadata string

	cmd := &cobra.Command{
		Use:   "update <incident-id> <integration-id>",
		Short: "Update an incident integration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			// fetch current to preserve required fields
			getResp, httpResp, err := iapi.api.GetIncidentIntegration(iapi.ctx, args[0], args[1])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident integration: %w", err)
			}

			current := getResp.GetData()
			currentAttrs := current.GetAttributes()

			attrs := datadogV2.NewIncidentIntegrationMetadataAttributes(
				currentAttrs.GetIntegrationType(),
				currentAttrs.GetMetadata(),
			)

			if cmd.Flags().Changed("metadata") {
				meta, parseErr := parseIntegrationMetadata(metadata)
				if parseErr != nil {
					return parseErr
				}
				attrs.SetMetadata(meta)
			}

			data := datadogV2.NewIncidentIntegrationMetadataPatchData(
				*attrs,
				datadogV2.INCIDENTINTEGRATIONMETADATATYPE_INCIDENT_INTEGRATIONS,
			)
			body := datadogV2.NewIncidentIntegrationMetadataPatchRequest(*data)

			resp, httpResp2, err := iapi.api.UpdateIncidentIntegration(iapi.ctx, args[0], args[1], *body)
			if httpResp2 != nil {
				_ = httpResp2.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update incident integration: %w", err)
			}

			d := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIntegrationDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&metadata, "metadata", "", "updated metadata as JSON")
	return cmd
}

func newIntegrationDeleteCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <incident-id> <integration-id>",
		Short: "Delete an incident integration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return errIntegrationYesRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.api.DeleteIncidentIntegration(iapi.ctx, args[0], args[1])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete incident integration: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "integration %s deleted\n", args[1])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
