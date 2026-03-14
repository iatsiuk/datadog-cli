package cmd

import (
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

func newSecurityFindingCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finding",
		Short: "Manage security findings",
	}
	cmd.AddCommand(newSecurityFindingListCmd(mkAPI))
	cmd.AddCommand(newSecurityFindingShowCmd(mkAPI))
	cmd.AddCommand(newSecurityFindingMuteCmd(mkAPI))
	return cmd
}

func newSecurityFindingListCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		query string
		limit int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List security findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			const maxPageLimit = 1000
			if limit <= 0 || limit > maxPageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxPageLimit)
			}

			opts := datadogV2.NewListFindingsOptionalParameters().
				WithPageLimit(int64(limit)) //nolint:gosec
			if query != "" {
				opts = opts.WithFilterTags(query)
			}

			resp, httpResp, err := sapi.api.ListFindings(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list findings: %w", err)
			}

			findings := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if findings == nil {
					findings = []datadogV2.Finding{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), findings)
			}

			headers := []string{"ID", "RULE", "RESOURCE", "STATUS", "SEVERITY"}
			rows := make([][]string, 0, len(findings))
			for _, f := range findings {
				attrs := f.GetAttributes()
				ruleName := ""
				if rule := attrs.Rule; rule != nil {
					ruleName = rule.GetName()
				}
				status := ""
				if attrs.Status != nil {
					status = string(*attrs.Status)
				}
				// severity is derived from status for findings
				severity := signalTagValue(attrs.Tags, "severity")
				rows = append(rows, []string{
					f.GetId(),
					ruleName,
					attrs.GetResource(),
					status,
					severity,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "filter by tags (e.g. env:prod)")
	cmd.Flags().IntVar(&limit, "limit", 50, "max number of findings to return")
	return cmd
}

func newSecurityFindingShowCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <finding-id>",
		Short: "Show security finding details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetFinding(sapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get finding: %w", err)
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

			ruleName := ""
			if rule := attrs.Rule; rule != nil {
				ruleName = rule.GetName()
			}
			status := ""
			if attrs.Status != nil {
				status = string(*attrs.Status)
			}

			fields := []struct{ k, v string }{
				{"ID", f.GetId()},
				{"Rule", ruleName},
				{"Resource", attrs.GetResource()},
				{"ResourceType", attrs.GetResourceType()},
				{"Status", status},
				{"Evaluation", findingEvaluationStr(attrs.Evaluation)},
				{"Tags", strings.Join(attrs.GetTags(), ", ")},
				{"Message", attrs.GetMessage()},
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

func newSecurityFindingMuteCmd(mkAPI func() (*securityAPI, error)) *cobra.Command {
	var (
		reason     string
		expiration int64
	)

	cmd := &cobra.Command{
		Use:   "mute <finding-id>",
		Short: "Mute a security finding",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			findingID := args[0]

			muteReason, err := datadogV2.NewFindingMuteReasonFromValue(reason)
			if err != nil {
				return fmt.Errorf("--reason: %w", err)
			}

			props := datadogV2.NewBulkMuteFindingsRequestProperties(true, *muteReason)
			if expiration > 0 {
				props.SetExpirationDate(expiration)
			}

			findingRef := datadogV2.NewBulkMuteFindingsRequestMetaFindings()
			findingRef.SetFindingId(findingID)

			meta := datadogV2.NewBulkMuteFindingsRequestMeta()
			meta.SetFindings([]datadogV2.BulkMuteFindingsRequestMetaFindings{*findingRef})

			attrs := datadogV2.NewBulkMuteFindingsRequestAttributes(*props)
			data := datadogV2.NewBulkMuteFindingsRequestData(*attrs, findingID, *meta, datadogV2.FINDINGTYPE_FINDING)
			body := datadogV2.NewBulkMuteFindingsRequest(*data)

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := sapi.api.MuteFindings(sapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("mute finding: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "mute reason: PENDING_FIX, FALSE_POSITIVE, ACCEPTED_RISK, NO_PENDING_FIX, HUMAN_ERROR, OTHER (required)")
	cmd.Flags().Int64Var(&expiration, "expiration", 0, "expiration timestamp in Unix ms (0 = indefinite)")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

// findingEvaluationStr returns the string representation of a FindingEvaluation.
func findingEvaluationStr(e *datadogV2.FindingEvaluation) string {
	if e == nil {
		return ""
	}
	return string(*e)
}
