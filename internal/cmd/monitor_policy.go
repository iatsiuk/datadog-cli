package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errPolicyIDRequired = errors.New("--id is required")
var errTagKeyRequired = errors.New("--tag-key is required")

type policiesAPI struct {
	api *datadogV2.MonitorsApi
	ctx context.Context
}

func defaultPoliciesAPI() (*policiesAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &policiesAPI{api: datadogV2.NewMonitorsApi(c), ctx: ctx}, nil
}

func newPolicyCmd(mkAPI func() (*policiesAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage monitor configuration policies",
	}
	cmd.AddCommand(newPolicyListCmd(mkAPI))
	cmd.AddCommand(newPolicyShowCmd(mkAPI))
	cmd.AddCommand(newPolicyCreateCmd(mkAPI))
	cmd.AddCommand(newPolicyUpdateCmd(mkAPI))
	cmd.AddCommand(newPolicyDeleteCmd(mkAPI))
	return cmd
}

func policyRow(p datadogV2.MonitorConfigPolicyResponseData) []string {
	attrs := p.GetAttributes()
	policyType := string(attrs.GetPolicyType())
	tagKey := ""
	tagKeyReq := ""
	validValues := ""
	if tp := attrs.GetPolicy().MonitorConfigPolicyTagPolicy; tp != nil {
		tagKey = tp.GetTagKey()
		tagKeyReq = fmt.Sprintf("%v", tp.GetTagKeyRequired())
		validValues = strings.Join(tp.GetValidTagValues(), ", ")
	}
	return []string{p.GetId(), policyType, tagKey, tagKeyReq, validValues}
}

func newPolicyListCmd(mkAPI func() (*policiesAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List monitor configuration policies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := papi.api.ListMonitorConfigPolicies(papi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list monitor config policies: %w", err)
			}

			policies := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if policies == nil {
					policies = []datadogV2.MonitorConfigPolicyResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), policies)
			}

			headers := []string{"ID", "POLICY_TYPE", "TAG_KEY", "TAG_KEY_REQUIRED", "VALID_VALUES"}
			rows := make([][]string, 0, len(policies))
			for _, p := range policies {
				rows = append(rows, policyRow(p))
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newPolicyShowCmd(mkAPI func() (*policiesAPI, error)) *cobra.Command {
	var policyID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a monitor configuration policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if policyID == "" {
				return errPolicyIDRequired
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := papi.api.GetMonitorConfigPolicy(papi.ctx, policyID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor config policy: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}

			attrs := d.GetAttributes()
			rows := [][]string{
				{"ID", d.GetId()},
				{"POLICY_TYPE", string(attrs.GetPolicyType())},
			}
			if tp := attrs.GetPolicy().MonitorConfigPolicyTagPolicy; tp != nil {
				rows = append(rows,
					[]string{"TAG_KEY", tp.GetTagKey()},
					[]string{"TAG_KEY_REQUIRED", fmt.Sprintf("%v", tp.GetTagKeyRequired())},
					[]string{"VALID_VALUES", strings.Join(tp.GetValidTagValues(), ", ")},
				)
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().StringVar(&policyID, "id", "", "policy ID")
	return cmd
}

func newPolicyCreateCmd(mkAPI func() (*policiesAPI, error)) *cobra.Command {
	var (
		tagKey         string
		tagKeyRequired bool
		validValues    []string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a monitor configuration policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if tagKey == "" {
				return errTagKeyRequired
			}

			tagPolicy := datadogV2.NewMonitorConfigPolicyTagPolicyCreateRequest(tagKey, tagKeyRequired, validValues)
			policy := datadogV2.MonitorConfigPolicyTagPolicyCreateRequestAsMonitorConfigPolicyPolicyCreateRequest(tagPolicy)
			attrs := datadogV2.NewMonitorConfigPolicyAttributeCreateRequest(policy, datadogV2.MONITORCONFIGPOLICYTYPE_TAG)
			data := datadogV2.NewMonitorConfigPolicyCreateData(*attrs, datadogV2.MONITORCONFIGPOLICYRESOURCETYPE_MONITOR_CONFIG_POLICY)
			body := datadogV2.NewMonitorConfigPolicyCreateRequest(*data)

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := papi.api.CreateMonitorConfigPolicy(papi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create monitor config policy: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}

			respAttrs := d.GetAttributes()
			rows := [][]string{
				{"ID", d.GetId()},
				{"POLICY_TYPE", string(respAttrs.GetPolicyType())},
			}
			if tp := respAttrs.GetPolicy().MonitorConfigPolicyTagPolicy; tp != nil {
				rows = append(rows,
					[]string{"TAG_KEY", tp.GetTagKey()},
					[]string{"TAG_KEY_REQUIRED", fmt.Sprintf("%v", tp.GetTagKeyRequired())},
					[]string{"VALID_VALUES", strings.Join(tp.GetValidTagValues(), ", ")},
				)
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().StringVar(&tagKey, "tag-key", "", "tag key (required)")
	cmd.Flags().BoolVar(&tagKeyRequired, "tag-key-required", false, "make tag key required for monitor creation")
	cmd.Flags().StringArrayVar(&validValues, "valid-values", nil, "valid tag values (can be specified multiple times)")
	return cmd
}

func newPolicyUpdateCmd(mkAPI func() (*policiesAPI, error)) *cobra.Command {
	var (
		policyID       string
		tagKey         string
		tagKeyRequired bool
		validValues    []string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a monitor configuration policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if policyID == "" {
				return errPolicyIDRequired
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			existing, httpResp, err := papi.api.GetMonitorConfigPolicy(papi.ctx, policyID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor config policy: %w", err)
			}

			existData := existing.GetData()
			existAttrs := existData.GetAttributes()
			curTagKey := ""
			curTagKeyRequired := false
			curValidValues := []string{}
			if existTag := existAttrs.GetPolicy().MonitorConfigPolicyTagPolicy; existTag != nil {
				curTagKey = existTag.GetTagKey()
				curTagKeyRequired = existTag.GetTagKeyRequired()
				curValidValues = existTag.GetValidTagValues()
			}

			if cmd.Flags().Changed("tag-key") {
				curTagKey = tagKey
			}
			if cmd.Flags().Changed("tag-key-required") {
				curTagKeyRequired = tagKeyRequired
			}
			if cmd.Flags().Changed("valid-values") {
				curValidValues = validValues
			}

			tagPol := datadogV2.NewMonitorConfigPolicyTagPolicy()
			tagPol.SetTagKey(curTagKey)
			tagPol.SetTagKeyRequired(curTagKeyRequired)
			tagPol.SetValidTagValues(curValidValues)

			pol := datadogV2.MonitorConfigPolicyTagPolicyAsMonitorConfigPolicyPolicy(tagPol)
			editAttrs := datadogV2.NewMonitorConfigPolicyAttributeEditRequest(pol, datadogV2.MONITORCONFIGPOLICYTYPE_TAG)
			editData := datadogV2.NewMonitorConfigPolicyEditData(*editAttrs, policyID, datadogV2.MONITORCONFIGPOLICYRESOURCETYPE_MONITOR_CONFIG_POLICY)
			body := datadogV2.NewMonitorConfigPolicyEditRequest(*editData)

			resp, httpResp, err := papi.api.UpdateMonitorConfigPolicy(papi.ctx, policyID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update monitor config policy: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}

			respAttrs := d.GetAttributes()
			rows := [][]string{
				{"ID", d.GetId()},
				{"POLICY_TYPE", string(respAttrs.GetPolicyType())},
			}
			if tp := respAttrs.GetPolicy().MonitorConfigPolicyTagPolicy; tp != nil {
				rows = append(rows,
					[]string{"TAG_KEY", tp.GetTagKey()},
					[]string{"TAG_KEY_REQUIRED", fmt.Sprintf("%v", tp.GetTagKeyRequired())},
					[]string{"VALID_VALUES", strings.Join(tp.GetValidTagValues(), ", ")},
				)
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().StringVar(&policyID, "id", "", "policy ID (required)")
	cmd.Flags().StringVar(&tagKey, "tag-key", "", "tag key")
	cmd.Flags().BoolVar(&tagKeyRequired, "tag-key-required", false, "make tag key required for monitor creation")
	cmd.Flags().StringArrayVar(&validValues, "valid-values", nil, "valid tag values")
	return cmd
}

func newPolicyDeleteCmd(mkAPI func() (*policiesAPI, error)) *cobra.Command {
	var (
		policyID string
		yes      bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a monitor configuration policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if policyID == "" {
				return errPolicyIDRequired
			}
			if !yes {
				return errYesRequired
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := papi.api.DeleteMonitorConfigPolicy(papi.ctx, policyID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete monitor config policy: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deleted monitor config policy %s\n", policyID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().StringVar(&policyID, "id", "", "policy ID (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
