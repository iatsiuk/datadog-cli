package cmd

import (
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

// ruleResponseFields extracts common display fields from the union type.
func ruleResponseFields(r datadogV2.SecurityMonitoringRuleResponse) (id, name, ruleType string, isEnabled bool, severity string) {
	if std := r.SecurityMonitoringStandardRuleResponse; std != nil {
		id = std.GetId()
		name = std.GetName()
		ruleType = string(std.GetType())
		isEnabled = std.GetIsEnabled()
		for _, c := range std.GetCases() {
			if c.Status != nil {
				severity = string(*c.Status)
				break
			}
		}
		return
	}
	if sig := r.SecurityMonitoringSignalRuleResponse; sig != nil {
		id = sig.GetId()
		name = sig.GetName()
		ruleType = string(sig.GetType())
		isEnabled = sig.GetIsEnabled()
		for _, c := range sig.GetCases() {
			if c.Status != nil {
				severity = string(*c.Status)
				break
			}
		}
	}
	return
}

func newSecurityRuleCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage security detection rules",
	}
	cmd.AddCommand(newSecurityRuleListCmd(mkAPI))
	cmd.AddCommand(newSecurityRuleShowCmd(mkAPI))
	cmd.AddCommand(newSecurityRuleCreateCmd(mkAPI))
	cmd.AddCommand(newSecurityRuleUpdateCmd(mkAPI))
	cmd.AddCommand(newSecurityRuleDeleteCmd(mkAPI))
	cmd.AddCommand(newSecurityRuleValidateCmd(mkAPI))
	return cmd
}

func newSecurityRuleListCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var pageSize int64

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List security detection rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListSecurityMonitoringRulesOptionalParameters()
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}

			resp, httpResp, err := sapi.api.ListSecurityMonitoringRules(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list rules: %w", err)
			}

			rules := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if rules == nil {
					rules = []datadogV2.SecurityMonitoringRuleResponse{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), rules)
			}

			headers := []string{"ID", "NAME", "TYPE", "IS_ENABLED", "SEVERITY"}
			rows := make([][]string, 0, len(rules))
			for _, r := range rules {
				id, name, ruleType, isEnabled, severity := ruleResponseFields(r)
				enabled := "false"
				if isEnabled {
					enabled = "true"
				}
				rows = append(rows, []string{id, name, ruleType, enabled, severity})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "max number of rules per page")
	return cmd
}

func newSecurityRuleShowCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <rule-id>",
		Short: "Show security rule details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSecurityMonitoringRule(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get rule: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			id, name, ruleType, isEnabled, severity := ruleResponseFields(resp)
			enabled := "false"
			if isEnabled {
				enabled = "true"
			}

			var message, queries string
			if std := resp.SecurityMonitoringStandardRuleResponse; std != nil {
				message = std.GetMessage()
				var qs []string
				for _, q := range std.GetQueries() {
					if s := q.GetQuery(); s != "" {
						qs = append(qs, s)
					}
				}
				queries = strings.Join(qs, "; ")
			} else if sig := resp.SecurityMonitoringSignalRuleResponse; sig != nil {
				message = sig.GetMessage()
				var qs []string
				for _, q := range sig.GetQueries() {
					if s := q.GetRuleId(); s != "" {
						qs = append(qs, s)
					}
				}
				queries = strings.Join(qs, "; ")
			}

			fields := []struct{ k, v string }{
				{"ID", id},
				{"Name", name},
				{"Type", ruleType},
				{"Enabled", enabled},
				{"Severity", severity},
				{"Message", message},
				{"Queries", queries},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-12s %s\n", f.k+":", f.v) //nolint:errcheck
			}
			return nil
		},
	}
}

// buildRulePayload builds the common parts of a create/validate payload.
func buildRulePayload(query, severity string) (
	[]datadogV2.SecurityMonitoringRuleCaseCreate,
	datadogV2.SecurityMonitoringRuleOptions,
	[]datadogV2.SecurityMonitoringStandardRuleQuery,
	error,
) {
	sev, err := datadogV2.NewSecurityMonitoringRuleSeverityFromValue(severity)
	if err != nil {
		return nil, datadogV2.SecurityMonitoringRuleOptions{}, nil, fmt.Errorf("--severity: %w", err)
	}

	c := datadogV2.NewSecurityMonitoringRuleCaseCreate(*sev)
	cond := "a > 0"
	c.SetCondition(cond)
	cases := []datadogV2.SecurityMonitoringRuleCaseCreate{*c}

	opts := *datadogV2.NewSecurityMonitoringRuleOptions()

	q := datadogV2.NewSecurityMonitoringStandardRuleQuery()
	q.SetName("a")
	q.SetQuery(query)
	queries := []datadogV2.SecurityMonitoringStandardRuleQuery{*q}

	return cases, opts, queries, nil
}

func newSecurityRuleCreateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name     string
		query    string
		message  string
		severity string
		ruleType string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a security detection rule",
		RunE: func(_ *cobra.Command, _ []string) error {
			if ruleType == "signal_correlation" {
				return fmt.Errorf("--type signal_correlation requires a different query schema; use signal_correlation type via the API directly")
			}

			cases, opts, queries, err := buildRulePayload(query, severity)
			if err != nil {
				return err
			}

			payload := datadogV2.NewSecurityMonitoringStandardRuleCreatePayload(cases, true, message, name, opts, queries)
			if ruleType != "" {
				rt, rtErr := datadogV2.NewSecurityMonitoringRuleTypeCreateFromValue(ruleType)
				if rtErr != nil {
					return fmt.Errorf("--type: %w", rtErr)
				}
				payload.SetType(*rt)
			}

			body := datadogV2.SecurityMonitoringStandardRuleCreatePayloadAsSecurityMonitoringRuleCreatePayload(payload)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := sapi.api.CreateSecurityMonitoringRule(sapi.ctx, body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create rule: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "rule name (required)")
	cmd.Flags().StringVar(&query, "query", "", "log query (required)")
	cmd.Flags().StringVar(&message, "message", "", "signal message (required)")
	cmd.Flags().StringVar(&severity, "severity", "", "signal severity: info, low, medium, high, critical (required)")
	cmd.Flags().StringVar(&ruleType, "type", "log_detection", "rule type")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("query")
	_ = cmd.MarkFlagRequired("message")
	_ = cmd.MarkFlagRequired("severity")
	return cmd
}

func newSecurityRuleUpdateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name    string
		message string
		enabled bool
	)

	cmd := &cobra.Command{
		Use:   "update <rule-id>",
		Short: "Update a security detection rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			payload := datadogV2.NewSecurityMonitoringRuleUpdatePayload()
			if name != "" {
				payload.SetName(name)
			}
			if message != "" {
				payload.SetMessage(message)
			}
			if cmd.Flags().Changed("enabled") {
				payload.SetIsEnabled(enabled)
			}

			_, httpResp, err := sapi.api.UpdateSecurityMonitoringRule(sapi.ctx, args[0], *payload)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update rule: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new rule name")
	cmd.Flags().StringVar(&message, "message", "", "new signal message")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable or disable the rule")
	return cmd
}

func newSecurityRuleDeleteCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <rule-id>",
		Short: "Delete a security detection rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return errYesRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := sapi.api.DeleteSecurityMonitoringRule(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete rule: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newSecurityRuleValidateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name     string
		query    string
		message  string
		severity string
		ruleType string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a security detection rule without creating it",
		RunE: func(_ *cobra.Command, _ []string) error {
			if ruleType == "signal_correlation" {
				return fmt.Errorf("--type signal_correlation requires a different query schema; use signal_correlation type via the API directly")
			}

			cases, opts, queries, err := buildRulePayload(query, severity)
			if err != nil {
				return err
			}

			payload := datadogV2.NewSecurityMonitoringStandardRulePayload(cases, true, message, name, opts, queries)
			if ruleType != "" {
				rt, rtErr := datadogV2.NewSecurityMonitoringRuleTypeCreateFromValue(ruleType)
				if rtErr != nil {
					return fmt.Errorf("--type: %w", rtErr)
				}
				payload.SetType(*rt)
			}

			body := datadogV2.SecurityMonitoringStandardRulePayloadAsSecurityMonitoringRuleValidatePayload(payload)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := sapi.api.ValidateSecurityMonitoringRule(sapi.ctx, body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("validate rule: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "rule name (required)")
	cmd.Flags().StringVar(&query, "query", "", "log query (required)")
	cmd.Flags().StringVar(&message, "message", "", "signal message (required)")
	cmd.Flags().StringVar(&severity, "severity", "", "signal severity: info, low, medium, high, critical (required)")
	cmd.Flags().StringVar(&ruleType, "type", "log_detection", "rule type")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("query")
	_ = cmd.MarkFlagRequired("message")
	_ = cmd.MarkFlagRequired("severity")
	return cmd
}
