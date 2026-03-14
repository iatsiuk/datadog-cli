package cmd

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

func newSecuritySuppressionCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suppression",
		Short: "Manage security suppression rules",
	}
	cmd.AddCommand(newSecuritySuppressionListCmd(mkAPI))
	cmd.AddCommand(newSecuritySuppressionShowCmd(mkAPI))
	cmd.AddCommand(newSecuritySuppressionCreateCmd(mkAPI))
	cmd.AddCommand(newSecuritySuppressionUpdateCmd(mkAPI))
	cmd.AddCommand(newSecuritySuppressionDeleteCmd(mkAPI))
	return cmd
}

func newSecuritySuppressionListCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List security suppression rules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.ListSecurityMonitoringSuppressions(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list suppressions: %w", err)
			}

			suppressions := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if suppressions == nil {
					suppressions = []datadogV2.SecurityMonitoringSuppression{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), suppressions)
			}

			headers := []string{"ID", "NAME", "RULE_QUERY", "ENABLED", "EXPIRATION"}
			rows := make([][]string, 0, len(suppressions))
			for _, s := range suppressions {
				attrs := s.GetAttributes()
				expiration := ""
				if exp := attrs.ExpirationDate; exp != nil {
					expiration = time.UnixMilli(*exp).UTC().Format("2006-01-02")
				}
				enabled := "false"
				if attrs.Enabled != nil && *attrs.Enabled {
					enabled = "true"
				}
				rows = append(rows, []string{
					s.GetId(),
					attrs.GetName(),
					attrs.GetRuleQuery(),
					enabled,
					expiration,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newSecuritySuppressionShowCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <suppression-id>",
		Short: "Show security suppression rule details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSecurityMonitoringSuppression(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get suppression: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			s := resp.GetData()
			attrs := s.GetAttributes()

			expiration := ""
			if exp := attrs.ExpirationDate; exp != nil {
				expiration = time.UnixMilli(*exp).UTC().Format("2006-01-02")
			}
			enabled := "false"
			if attrs.Enabled != nil && *attrs.Enabled {
				enabled = "true"
			}

			fields := []struct{ k, v string }{
				{"ID", s.GetId()},
				{"Name", attrs.GetName()},
				{"Enabled", enabled},
				{"RuleQuery", attrs.GetRuleQuery()},
				{"SuppressQuery", attrs.GetSuppressionQuery()},
				{"Expiration", expiration},
				{"Description", attrs.GetDescription()},
			}
			w := cmd.OutOrStdout()
			for _, f := range fields {
				if f.v == "" {
					continue
				}
				fmt.Fprintf(w, "%-15s %s\n", f.k+":", f.v) //nolint:errcheck
			}
			return nil
		},
	}
}

func newSecuritySuppressionCreateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name             string
		ruleQuery        string
		suppressionQuery string
		expiration       string
		description      string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a security suppression rule",
		RunE: func(_ *cobra.Command, _ []string) error {
			attrs := datadogV2.NewSecurityMonitoringSuppressionCreateAttributes(true, name, ruleQuery)
			if suppressionQuery != "" {
				attrs.SetSuppressionQuery(suppressionQuery)
			}
			if description != "" {
				attrs.SetDescription(description)
			}
			if expiration != "" {
				t, err := time.Parse("2006-01-02", expiration)
				if err != nil {
					return fmt.Errorf("--expiration: must be YYYY-MM-DD: %w", err)
				}
				attrs.SetExpirationDate(t.UnixMilli())
			}

			data := datadogV2.NewSecurityMonitoringSuppressionCreateData(
				*attrs,
				datadogV2.SECURITYMONITORINGSUPPRESSIONTYPE_SUPPRESSIONS,
			)
			body := datadogV2.NewSecurityMonitoringSuppressionCreateRequest(*data)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := sapi.api.CreateSecurityMonitoringSuppression(sapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create suppression: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "suppression rule name (required)")
	cmd.Flags().StringVar(&ruleQuery, "rule-query", "", "detection rule query (required)")
	cmd.Flags().StringVar(&suppressionQuery, "suppression-query", "", "suppression query")
	cmd.Flags().StringVar(&expiration, "expiration", "", "expiration date, YYYY-MM-DD")
	cmd.Flags().StringVar(&description, "description", "", "description")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("rule-query")
	return cmd
}

func newSecuritySuppressionUpdateCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		name             string
		ruleQuery        string
		suppressionQuery string
		expiration       string
		description      string
		enabled          bool
	)

	cmd := &cobra.Command{
		Use:   "update <suppression-id>",
		Short: "Update a security suppression rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attrs := datadogV2.NewSecurityMonitoringSuppressionUpdateAttributesWithDefaults()
			if name != "" {
				attrs.SetName(name)
			}
			if ruleQuery != "" {
				attrs.SetRuleQuery(ruleQuery)
			}
			if suppressionQuery != "" {
				attrs.SetSuppressionQuery(suppressionQuery)
			}
			if description != "" {
				attrs.SetDescription(description)
			}
			if cmd.Flags().Changed("enabled") {
				attrs.SetEnabled(enabled)
			}
			if expiration != "" {
				t, err := time.Parse("2006-01-02", expiration)
				if err != nil {
					return fmt.Errorf("--expiration: must be YYYY-MM-DD: %w", err)
				}
				attrs.SetExpirationDate(t.UnixMilli())
			}

			data := datadogV2.NewSecurityMonitoringSuppressionUpdateData(
				*attrs,
				datadogV2.SECURITYMONITORINGSUPPRESSIONTYPE_SUPPRESSIONS,
			)
			body := datadogV2.NewSecurityMonitoringSuppressionUpdateRequest(*data)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := sapi.api.UpdateSecurityMonitoringSuppression(sapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update suppression: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new suppression rule name")
	cmd.Flags().StringVar(&ruleQuery, "rule-query", "", "new detection rule query")
	cmd.Flags().StringVar(&suppressionQuery, "suppression-query", "", "new suppression query")
	cmd.Flags().StringVar(&expiration, "expiration", "", "new expiration date, YYYY-MM-DD")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable or disable the suppression rule")
	return cmd
}

func newSecuritySuppressionDeleteCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <suppression-id>",
		Short: "Delete a security suppression rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return errYesRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := sapi.api.DeleteSecurityMonitoringSuppression(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete suppression: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
