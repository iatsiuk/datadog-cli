package cmd

import (
	"errors"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errFilterYesRequired = errors.New("--yes is required to delete a filter")

func newSecurityFilterCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Manage security filters",
	}
	cmd.AddCommand(newSecurityFilterListCmd(mkAPI))
	cmd.AddCommand(newSecurityFilterShowCmd(mkAPI))
	cmd.AddCommand(newSecurityFilterCreateCmd(mkAPI))
	cmd.AddCommand(newSecurityFilterUpdateCmd(mkAPI))
	cmd.AddCommand(newSecurityFilterDeleteCmd(mkAPI))
	return cmd
}

func newSecurityFilterListCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List security filters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.ListSecurityFilters(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list filters: %w", err)
			}

			filters := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if filters == nil {
					filters = []datadogV2.SecurityFilter{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), filters)
			}

			headers := []string{"ID", "NAME", "QUERY", "ENABLED", "TYPE"}
			rows := make([][]string, 0, len(filters))
			for _, f := range filters {
				attrs := f.GetAttributes()
				enabled := "false"
				if attrs.IsEnabled != nil && *attrs.IsEnabled {
					enabled = "true"
				}
				filteredDataType := ""
				if attrs.FilteredDataType != nil {
					filteredDataType = string(*attrs.FilteredDataType)
				}
				rows = append(rows, []string{
					f.GetId(),
					attrs.GetName(),
					attrs.GetQuery(),
					enabled,
					filteredDataType,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newSecurityFilterShowCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <filter-id>",
		Short: "Show security filter details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSecurityFilter(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get filter: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			f := resp.GetData()
			attrs := f.GetAttributes()

			enabled := "false"
			if attrs.IsEnabled != nil && *attrs.IsEnabled {
				enabled = "true"
			}
			filteredDataType := ""
			if attrs.FilteredDataType != nil {
				filteredDataType = string(*attrs.FilteredDataType)
			}

			fields := []struct{ k, v string }{
				{"ID", f.GetId()},
				{"Name", attrs.GetName()},
				{"Query", attrs.GetQuery()},
				{"Enabled", enabled},
				{"DataType", filteredDataType},
			}
			w := cmd.OutOrStdout()
			for _, fld := range fields {
				if fld.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-15s %s\n", fld.k+":", fld.v) //nolint:errcheck
			}
			return nil
		},
	}
}

func newSecurityFilterCreateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name             string
		query            string
		filteredDataType string
		isEnabled        bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a security filter",
		RunE: func(_ *cobra.Command, _ []string) error {
			fdt, err := datadogV2.NewSecurityFilterFilteredDataTypeFromValue(filteredDataType)
			if err != nil {
				return fmt.Errorf("--filtered-data-type: invalid value %q: %w", filteredDataType, err)
			}

			attrs := datadogV2.NewSecurityFilterCreateAttributes(
				[]datadogV2.SecurityFilterExclusionFilter{},
				*fdt,
				isEnabled,
				name,
				query,
			)
			data := datadogV2.NewSecurityFilterCreateData(
				*attrs,
				datadogV2.SECURITYFILTERTYPE_SECURITY_FILTERS,
			)
			body := datadogV2.NewSecurityFilterCreateRequest(*data)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := sapi.api.CreateSecurityFilter(sapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create filter: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "filter name (required)")
	cmd.Flags().StringVar(&query, "query", "", "filter query (required)")
	cmd.Flags().StringVar(&filteredDataType, "filtered-data-type", "logs", "filtered data type (logs)")
	cmd.Flags().BoolVar(&isEnabled, "is-enabled", true, "enable the filter")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func newSecurityFilterUpdateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name             string
		query            string
		filteredDataType string
		isEnabled        bool
	)

	cmd := &cobra.Command{
		Use:   "update <filter-id>",
		Short: "Update a security filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attrs := datadogV2.NewSecurityFilterUpdateAttributesWithDefaults()
			if name != "" {
				attrs.SetName(name)
			}
			if query != "" {
				attrs.SetQuery(query)
			}
			if filteredDataType != "" {
				fdt, err := datadogV2.NewSecurityFilterFilteredDataTypeFromValue(filteredDataType)
				if err != nil {
					return fmt.Errorf("--filtered-data-type: invalid value %q: %w", filteredDataType, err)
				}
				attrs.SetFilteredDataType(*fdt)
			}
			if cmd.Flags().Changed("is-enabled") {
				attrs.SetIsEnabled(isEnabled)
			}

			data := datadogV2.NewSecurityFilterUpdateData(
				*attrs,
				datadogV2.SECURITYFILTERTYPE_SECURITY_FILTERS,
			)
			body := datadogV2.NewSecurityFilterUpdateRequest(*data)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := sapi.api.UpdateSecurityFilter(sapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update filter: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new filter name")
	cmd.Flags().StringVar(&query, "query", "", "new filter query")
	cmd.Flags().StringVar(&filteredDataType, "filtered-data-type", "", "new filtered data type")
	cmd.Flags().BoolVar(&isEnabled, "is-enabled", true, "enable or disable the filter")
	return cmd
}

func newSecurityFilterDeleteCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <filter-id>",
		Short: "Delete a security filter",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return errFilterYesRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := sapi.api.DeleteSecurityFilter(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete filter: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
