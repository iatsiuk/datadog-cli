package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type slosAPI struct {
	api         *datadogV1.ServiceLevelObjectivesApi
	corrections *datadogV1.ServiceLevelObjectiveCorrectionsApi
	ctx         context.Context
}

func defaultSLOsAPI() (*slosAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &slosAPI{
		api:         datadogV1.NewServiceLevelObjectivesApi(c),
		corrections: datadogV1.NewServiceLevelObjectiveCorrectionsApi(c),
		ctx:         ctx,
	}, nil
}

// NewSLOsCommand returns the slos cobra command group.
func NewSLOsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slos",
		Short: "Manage Datadog SLOs",
	}
	cmd.AddCommand(newSLOsListCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsShowCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsHistoryCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsCreateCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsUpdateCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsDeleteCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsCanDeleteCmd(defaultSLOsAPI))
	cmd.AddCommand(newSLOsCorrectionCmd(defaultSLOsAPI))
	return cmd
}

func newSLOsListCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var (
		query string
		tags  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SLOs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewListSLOsOptionalParameters()
			if query != "" {
				opts = opts.WithQuery(query)
			}
			if tags != "" {
				opts = opts.WithTagsQuery(tags)
			}

			resp, httpResp, err := sapi.api.ListSLOs(sapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list SLOs: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV1.ServiceLevelObjective{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "TYPE", "TARGET", "TIMEFRAME", "TAGS"}
			var rows [][]string
			for _, slo := range data {
				target := ""
				timeframe := ""
				if thresholds := slo.GetThresholds(); len(thresholds) > 0 {
					th := thresholds[0]
					target = strconv.FormatFloat(th.GetTarget(), 'f', -1, 64)
					timeframe = string(th.GetTimeframe())
				}
				rows = append(rows, []string{
					slo.GetId(),
					slo.GetName(),
					string(slo.GetType()),
					target,
					timeframe,
					strings.Join(slo.GetTags(), ", "),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "filter SLOs by name/description")
	cmd.Flags().StringVar(&tags, "tags", "", "filter SLOs by tags")
	return cmd
}

func newSLOsShowCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show SLO details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSLO(sapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get SLO: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			// build threshold rows: timeframe -> target [warning]
			var thresholdParts []string
			for _, th := range data.GetThresholds() {
				part := string(th.GetTimeframe()) + ":" + strconv.FormatFloat(th.GetTarget(), 'f', -1, 64)
				if th.HasWarning() {
					part += " (warn:" + strconv.FormatFloat(th.GetWarning(), 'f', -1, 64) + ")"
				}
				thresholdParts = append(thresholdParts, part)
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", data.GetId()},
				{"Name", data.GetName()},
				{"Type", string(data.GetType())},
				{"Description", data.GetDescription()},
				{"Tags", strings.Join(data.GetTags(), ", ")},
				{"Thresholds", strings.Join(thresholdParts, ", ")},
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "SLO ID")
	return cmd
}

func newSLOsHistoryCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var (
		id      string
		fromStr string
		toStr   string
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show SLO history",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			fromTs, err := parseUnixOrRelative(fromStr)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toTs, err := parseUnixOrRelative(toStr)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.GetSLOHistory(sapi.ctx, id, fromTs, toTs)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get SLO history: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			overall := data.GetOverall()
			sliValue := "N/A"
			if v, ok := overall.GetSliValueOk(); ok && v != nil {
				sliValue = strconv.FormatFloat(*v, 'f', 4, 64)
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"SLI", sliValue},
				{"Type", string(data.GetType())},
			}
			for tf, budget := range overall.GetErrorBudgetRemaining() {
				rows = append(rows, []string{
					"ERROR BUDGET (" + tf + ")",
					strconv.FormatFloat(budget, 'f', 4, 64) + "%",
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "SLO ID")
	cmd.Flags().StringVar(&fromStr, "from", "", "start time: unix timestamp or relative (e.g. now-7d) (required)")
	cmd.Flags().StringVar(&toStr, "to", "", "end time: unix timestamp or relative (e.g. now) (required)")
	return cmd
}

func newSLOsCreateCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var (
		name        string
		sloType     string
		description string
		tags        string
		thresholds  string
		numerator   string
		denominator string
		monitorIDs  string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an SLO",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			if sloType == "" {
				return fmt.Errorf("--type is required")
			}
			if thresholds == "" {
				return fmt.Errorf("--thresholds is required")
			}

			var rawThresholds []struct {
				Timeframe string   `json:"timeframe"`
				Target    float64  `json:"target"`
				Warning   *float64 `json:"warning,omitempty"`
			}
			if err := json.Unmarshal([]byte(thresholds), &rawThresholds); err != nil {
				return fmt.Errorf("--thresholds: %w", err)
			}
			if len(rawThresholds) == 0 {
				return fmt.Errorf("--thresholds must not be empty")
			}

			sloThresholds := make([]datadogV1.SLOThreshold, len(rawThresholds))
			for i, t := range rawThresholds {
				th := datadogV1.NewSLOThreshold(t.Target, datadogV1.SLOTimeframe(t.Timeframe))
				if t.Warning != nil {
					th.SetWarning(*t.Warning)
				}
				sloThresholds[i] = *th
			}

			sloTypeVal := datadogV1.SLOType(sloType)
			body := datadogV1.NewServiceLevelObjectiveRequest(name, sloThresholds, sloTypeVal)
			if description != "" {
				body.SetDescription(description)
			}
			if tags != "" {
				body.SetTags(strings.Split(tags, ","))
			}

			switch sloTypeVal {
			case datadogV1.SLOTYPE_METRIC:
				if numerator == "" || denominator == "" {
					return fmt.Errorf("metric SLO requires --numerator and --denominator")
				}
				q := datadogV1.NewServiceLevelObjectiveQuery(denominator, numerator)
				body.SetQuery(*q)
			case datadogV1.SLOTYPE_MONITOR:
				if monitorIDs == "" {
					return fmt.Errorf("monitor SLO requires --monitor-ids")
				}
				parts := strings.Split(monitorIDs, ",")
				ids := make([]int64, 0, len(parts))
				for _, p := range parts {
					id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
					if err != nil {
						return fmt.Errorf("--monitor-ids: invalid id %q: %w", p, err)
					}
					ids = append(ids, id)
				}
				body.SetMonitorIds(ids)
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.CreateSLO(sapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create SLO: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV1.ServiceLevelObjective{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "TYPE", "TARGET", "TIMEFRAME", "TAGS"}
			var rows [][]string
			for _, slo := range data {
				target := ""
				timeframe := ""
				if ths := slo.GetThresholds(); len(ths) > 0 {
					th := ths[0]
					target = strconv.FormatFloat(th.GetTarget(), 'f', -1, 64)
					timeframe = string(th.GetTimeframe())
				}
				rows = append(rows, []string{
					slo.GetId(),
					slo.GetName(),
					string(slo.GetType()),
					target,
					timeframe,
					strings.Join(slo.GetTags(), ", "),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "SLO name (required)")
	cmd.Flags().StringVar(&sloType, "type", "", "SLO type: metric or monitor (required)")
	cmd.Flags().StringVar(&description, "description", "", "SLO description")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	cmd.Flags().StringVar(&thresholds, "thresholds", "", `thresholds JSON, e.g. '[{"timeframe":"30d","target":99.9}]' (required)`)
	cmd.Flags().StringVar(&numerator, "numerator", "", "metric query for good events (metric SLO)")
	cmd.Flags().StringVar(&denominator, "denominator", "", "metric query for total events (metric SLO)")
	cmd.Flags().StringVar(&monitorIDs, "monitor-ids", "", "comma-separated monitor IDs (monitor SLO)")
	return cmd
}

func newSLOsUpdateCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var (
		id          string
		name        string
		description string
		tags        string
		thresholds  string
		numerator   string
		denominator string
		monitorIDs  string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an SLO",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			// fetch existing SLO
			getResp, httpResp, err := sapi.api.GetSLO(sapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get SLO: %w", err)
			}
			// convert SLOResponseData to ServiceLevelObjective via JSON round-trip
			rawData, err := json.Marshal(getResp.GetData())
			if err != nil {
				return fmt.Errorf("marshal SLO: %w", err)
			}
			var slo datadogV1.ServiceLevelObjective
			if err := json.Unmarshal(rawData, &slo); err != nil {
				return fmt.Errorf("unmarshal SLO: %w", err)
			}

			// apply changes
			if name != "" {
				slo.SetName(name)
			}
			if cmd.Flags().Changed("description") {
				slo.SetDescription(description)
			}
			if tags != "" {
				slo.SetTags(strings.Split(tags, ","))
			}
			if thresholds != "" {
				var rawThresholds []struct {
					Timeframe string   `json:"timeframe"`
					Target    float64  `json:"target"`
					Warning   *float64 `json:"warning,omitempty"`
				}
				if err := json.Unmarshal([]byte(thresholds), &rawThresholds); err != nil {
					return fmt.Errorf("--thresholds: %w", err)
				}
				sloThresholds := make([]datadogV1.SLOThreshold, len(rawThresholds))
				for i, t := range rawThresholds {
					th := datadogV1.NewSLOThreshold(t.Target, datadogV1.SLOTimeframe(t.Timeframe))
					if t.Warning != nil {
						th.SetWarning(*t.Warning)
					}
					sloThresholds[i] = *th
				}
				slo.SetThresholds(sloThresholds)
			}
			if numerator != "" || denominator != "" {
				q := slo.GetQuery()
				if numerator != "" {
					q.SetNumerator(numerator)
				}
				if denominator != "" {
					q.SetDenominator(denominator)
				}
				slo.SetQuery(q)
			}
			if monitorIDs != "" {
				parts := strings.Split(monitorIDs, ",")
				ids := make([]int64, 0, len(parts))
				for _, p := range parts {
					mid, parseErr := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
					if parseErr != nil {
						return fmt.Errorf("--monitor-ids: invalid id %q: %w", p, parseErr)
					}
					ids = append(ids, mid)
				}
				slo.SetMonitorIds(ids)
			}

			resp, httpResp, err := sapi.api.UpdateSLO(sapi.ctx, id, slo)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update SLO: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV1.ServiceLevelObjective{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "TYPE", "TARGET", "TIMEFRAME", "TAGS"}
			var rows [][]string
			for _, slo := range data {
				target := ""
				timeframe := ""
				if ths := slo.GetThresholds(); len(ths) > 0 {
					th := ths[0]
					target = strconv.FormatFloat(th.GetTarget(), 'f', -1, 64)
					timeframe = string(th.GetTimeframe())
				}
				rows = append(rows, []string{
					slo.GetId(),
					slo.GetName(),
					string(slo.GetType()),
					target,
					timeframe,
					strings.Join(slo.GetTags(), ", "),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "SLO ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&tags, "tags", "", "new comma-separated tags")
	cmd.Flags().StringVar(&thresholds, "thresholds", "", `new thresholds JSON, e.g. '[{"timeframe":"30d","target":99.9}]'`)
	cmd.Flags().StringVar(&numerator, "numerator", "", "new metric query for good events (metric SLO)")
	cmd.Flags().StringVar(&denominator, "denominator", "", "new metric query for total events (metric SLO)")
	cmd.Flags().StringVar(&monitorIDs, "monitor-ids", "", "new comma-separated monitor IDs (monitor SLO)")
	return cmd
}

func newSLOsDeleteCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var (
		id  string
		yes bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an SLO",
		RunE: func(_ *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			if !yes {
				return fmt.Errorf("--yes is required to confirm deletion")
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.DeleteSLO(sapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete SLO: %w", err)
			}

			deleted := resp.GetData()
			errs := resp.GetErrors()
			if len(errs) > 0 {
				for k, v := range errs {
					fmt.Fprintf(os.Stderr, "error deleting %s: %s\n", k, v)
				}
			}
			if len(deleted) > 0 {
				fmt.Printf("deleted: %s\n", strings.Join(deleted, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "SLO ID (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newSLOsCanDeleteCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "can-delete",
		Short: "Check if an SLO can be deleted",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.CheckCanDeleteSLO(sapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("check can delete SLO: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"ID", "STATUS"}
			var rows [][]string

			if data := resp.GetData(); data.HasOk() {
				for _, okID := range data.GetOk() {
					rows = append(rows, []string{okID, "can delete"})
				}
			}
			for errID, errMsg := range resp.GetErrors() {
				rows = append(rows, []string{errID, "blocked: " + errMsg})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "SLO ID (required)")
	return cmd
}

func newSLOsCorrectionCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "correction",
		Short: "Manage SLO corrections",
	}
	cmd.AddCommand(newSLOCorrectionListCmd(mkAPI))
	cmd.AddCommand(newSLOCorrectionShowCmd(mkAPI))
	return cmd
}

func newSLOCorrectionListCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SLO corrections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.corrections.ListSLOCorrection(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list SLO corrections: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				if data == nil {
					data = []datadogV1.SLOCorrection{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "SLO_ID", "CATEGORY", "START", "END", "DESCRIPTION"}
			var rows [][]string
			for _, c := range data {
				attrs := c.GetAttributes()
				start := ""
				if s := attrs.Start; s != nil {
					start = time.Unix(*s, 0).UTC().Format("2006-01-02 15:04")
				}
				end := ""
				if e, ok := attrs.GetEndOk(); ok && e != nil {
					end = time.Unix(*e, 0).UTC().Format("2006-01-02 15:04")
				}
				rows = append(rows, []string{
					c.GetId(),
					attrs.GetSloId(),
					string(attrs.GetCategory()),
					start,
					end,
					attrs.GetDescription(),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newSLOCorrectionShowCmd(mkAPI func() (*slosAPI, error)) *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show SLO correction details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.corrections.GetSLOCorrection(sapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get SLO correction: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			data := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			attrs := data.GetAttributes()
			start := ""
			if s := attrs.Start; s != nil {
				start = time.Unix(*s, 0).UTC().Format(time.RFC3339)
			}
			end := ""
			if e, ok := attrs.GetEndOk(); ok && e != nil {
				end = time.Unix(*e, 0).UTC().Format(time.RFC3339)
			}

			headers := []string{"FIELD", "VALUE"}
			rows := [][]string{
				{"ID", data.GetId()},
				{"SLO ID", attrs.GetSloId()},
				{"Category", string(attrs.GetCategory())},
				{"Description", attrs.GetDescription()},
				{"Start", start},
				{"End", end},
				{"Timezone", attrs.GetTimezone()},
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "correction ID")
	return cmd
}
