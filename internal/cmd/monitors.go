package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type monitorsAPI struct {
	api *datadogV1.MonitorsApi
	ctx context.Context
}

func defaultMonitorsAPI() (*monitorsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &monitorsAPI{api: datadogV1.NewMonitorsApi(c), ctx: ctx}, nil
}

// NewMonitorsCommand returns the monitors cobra command group.
func NewMonitorsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitors",
		Short: "Manage Datadog monitors",
	}
	cmd.AddCommand(newMonitorsListCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsShowCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsSearchCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsCreateCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsUpdateCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsDeleteCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsMuteCmd(defaultMonitorsAPI))
	cmd.AddCommand(newMonitorsUnmuteCmd(defaultMonitorsAPI))
	cmd.AddCommand(newDowntimeCmd(defaultDowntimesAPI))
	cmd.AddCommand(newPolicyCmd(defaultPoliciesAPI))
	return cmd
}

func newMonitorsListCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		name     string
		tags     string
		pageSize int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List monitors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewListMonitorsOptionalParameters()
			if name != "" {
				opts = opts.WithName(name)
			}
			if tags != "" {
				opts = opts.WithTags(tags)
			}
			if pageSize > 0 {
				opts = opts.WithPageSize(int32(pageSize)) //nolint:gosec
			}

			monitors, httpResp, err := mapi.api.ListMonitors(mapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list monitors: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if monitors == nil {
					monitors = []datadogV1.Monitor{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), monitors)
			}

			headers := []string{"ID", "NAME", "TYPE", "STATUS", "QUERY"}
			rows := make([][]string, 0, len(monitors))
			for _, m := range monitors {
				id := fmt.Sprintf("%d", m.GetId())
				rows = append(rows, []string{
					id,
					m.GetName(),
					string(m.GetType()),
					string(m.GetOverallState()),
					m.GetQuery(),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "filter by monitor name")
	cmd.Flags().StringVar(&tags, "tags", "", "filter by tags, e.g. env:prod,service:web")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "number of monitors per page")
	return cmd
}

func newMonitorsSearchCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		query   string
		page    int64
		perPage int64
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search monitors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewSearchMonitorsOptionalParameters()
			if query != "" {
				opts = opts.WithQuery(query)
			}
			if page > 0 {
				opts = opts.WithPage(page)
			}
			if perPage > 0 {
				opts = opts.WithPerPage(perPage)
			}

			resp, httpResp, err := mapi.api.SearchMonitors(mapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search monitors: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			monitors := resp.GetMonitors()

			if asJSON {
				if monitors == nil {
					monitors = []datadogV1.MonitorSearchResult{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), monitors)
			}

			headers := []string{"ID", "NAME", "TYPE", "STATUS", "QUERY"}
			rows := make([][]string, 0, len(monitors))
			for _, m := range monitors {
				rows = append(rows, []string{
					fmt.Sprintf("%d", m.GetId()),
					m.GetName(),
					string(m.GetType()),
					string(m.GetStatus()),
					m.GetQuery(),
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "search query (e.g. type:metric status:Alert)")
	cmd.Flags().Int64Var(&page, "page", 0, "page number (0-indexed)")
	cmd.Flags().Int64Var(&perPage, "per-page", 0, "number of monitors per page")
	return cmd
}

var (
	errMonitorIDRequired    = errors.New("--id is required")
	errMonitorNameRequired  = errors.New("--name is required")
	errMonitorTypeRequired  = errors.New("--type is required")
	errMonitorQueryRequired = errors.New("--query is required")
	errYesRequired          = errors.New("--yes is required to confirm destructive action")
)

func newMonitorsShowCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var monitorID int64

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if monitorID == 0 {
				return errMonitorIDRequired
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			m, httpResp, err := mapi.api.GetMonitor(mapi.ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), m)
			}

			rows := [][]string{
				{"ID", fmt.Sprintf("%d", m.GetId())},
				{"NAME", m.GetName()},
				{"TYPE", string(m.GetType())},
				{"STATUS", string(m.GetOverallState())},
				{"QUERY", m.GetQuery()},
				{"MESSAGE", m.GetMessage()},
				{"TAGS", strings.Join(m.GetTags(), ", ")},
			}
			if t := m.GetCreated(); !t.IsZero() {
				rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
			}
			if t := m.GetModified(); !t.IsZero() {
				rows = append(rows, []string{"MODIFIED", t.Format("2006-01-02 15:04:05")})
			}

			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().Int64Var(&monitorID, "id", 0, "monitor ID")
	return cmd
}

func newMonitorsCreateCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		name       string
		monType    string
		query      string
		message    string
		tagsStr    string
		priority   int64
		thresholds string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errMonitorNameRequired
			}
			if monType == "" {
				return errMonitorTypeRequired
			}
			if query == "" {
				return errMonitorQueryRequired
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			body := datadogV1.NewMonitor(query, datadogV1.MonitorType(monType))
			body.SetName(name)

			if message != "" {
				body.SetMessage(message)
			}
			if tagsStr != "" {
				rawTags := strings.Split(tagsStr, ",")
				tags := make([]string, len(rawTags))
				for i, t := range rawTags {
					tags[i] = strings.TrimSpace(t)
				}
				body.SetTags(tags)
			}
			if priority > 0 {
				body.SetPriority(priority)
			}
			if thresholds != "" {
				var thr datadogV1.MonitorThresholds
				if err := json.Unmarshal([]byte(thresholds), &thr); err != nil {
					return fmt.Errorf("invalid --thresholds JSON: %w", err)
				}
				opts := body.GetOptions()
				opts.SetThresholds(thr)
				body.SetOptions(opts)
			}

			m, httpResp, err := mapi.api.CreateMonitor(mapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create monitor: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), m)
			}

			rows := [][]string{
				{"ID", fmt.Sprintf("%d", m.GetId())},
				{"NAME", m.GetName()},
				{"TYPE", string(m.GetType())},
				{"STATUS", string(m.GetOverallState())},
				{"QUERY", m.GetQuery()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "monitor name (required)")
	cmd.Flags().StringVar(&monType, "type", "", "monitor type, e.g. metric alert (required)")
	cmd.Flags().StringVar(&query, "query", "", "monitor query (required)")
	cmd.Flags().StringVar(&message, "message", "", "notification message")
	cmd.Flags().StringVar(&tagsStr, "tags", "", "comma-separated tags, e.g. env:prod,service:web")
	cmd.Flags().Int64Var(&priority, "priority", 0, "priority 1 (high) to 5 (low)")
	cmd.Flags().StringVar(&thresholds, "thresholds", "", "thresholds as JSON, e.g. {\"critical\":90,\"warning\":80}")
	return cmd
}

func newMonitorsUpdateCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		monitorID  int64
		name       string
		query      string
		message    string
		tagsStr    string
		priority   int64
		thresholds string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if monitorID == 0 {
				return errMonitorIDRequired
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			existing, httpResp, err := mapi.api.GetMonitor(mapi.ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor: %w", err)
			}

			body := datadogV1.NewMonitorUpdateRequest()
			// start from existing values
			body.SetName(existing.GetName())
			body.SetQuery(existing.GetQuery())
			body.SetMessage(existing.GetMessage())
			body.SetTags(existing.GetTags())
			if p := existing.GetPriority(); p != 0 {
				body.SetPriority(p)
			}
			body.SetOptions(existing.GetOptions())

			// override with explicitly changed flags
			if cmd.Flags().Changed("name") {
				body.SetName(name)
			}
			if cmd.Flags().Changed("query") {
				body.SetQuery(query)
			}
			if cmd.Flags().Changed("message") {
				body.SetMessage(message)
			}
			if cmd.Flags().Changed("tags") {
				rawTags := strings.Split(tagsStr, ",")
				tags := make([]string, len(rawTags))
				for i, t := range rawTags {
					tags[i] = strings.TrimSpace(t)
				}
				body.SetTags(tags)
			}
			if cmd.Flags().Changed("priority") {
				body.SetPriority(priority)
			}
			if cmd.Flags().Changed("thresholds") {
				var thr datadogV1.MonitorThresholds
				if err := json.Unmarshal([]byte(thresholds), &thr); err != nil {
					return fmt.Errorf("invalid --thresholds JSON: %w", err)
				}
				opts := body.GetOptions()
				opts.SetThresholds(thr)
				body.SetOptions(opts)
			}

			m, httpResp, err := mapi.api.UpdateMonitor(mapi.ctx, monitorID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update monitor: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), m)
			}

			rows := [][]string{
				{"ID", fmt.Sprintf("%d", m.GetId())},
				{"NAME", m.GetName()},
				{"TYPE", string(m.GetType())},
				{"STATUS", string(m.GetOverallState())},
				{"QUERY", m.GetQuery()},
			}
			return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().Int64Var(&monitorID, "id", 0, "monitor ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "monitor name")
	cmd.Flags().StringVar(&query, "query", "", "monitor query")
	cmd.Flags().StringVar(&message, "message", "", "notification message")
	cmd.Flags().StringVar(&tagsStr, "tags", "", "comma-separated tags")
	cmd.Flags().Int64Var(&priority, "priority", 0, "priority 1 (high) to 5 (low)")
	cmd.Flags().StringVar(&thresholds, "thresholds", "", "thresholds as JSON")
	return cmd
}

func newMonitorsDeleteCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		monitorID int64
		yes       bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if monitorID == 0 {
				return errMonitorIDRequired
			}
			if !yes {
				return errYesRequired
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			_, httpResp, err := mapi.api.DeleteMonitor(mapi.ctx, monitorID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete monitor: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deleted monitor %d\n", monitorID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().Int64Var(&monitorID, "id", 0, "monitor ID (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newMonitorsMuteCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		monitorID int64
		scope     string
		end       int64
	)

	cmd := &cobra.Command{
		Use:   "mute",
		Short: "Mute a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if monitorID == 0 {
				return errMonitorIDRequired
			}
			if scope == "" {
				return fmt.Errorf("--scope cannot be empty")
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			existing, httpResp, err := mapi.api.GetMonitor(mapi.ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor: %w", err)
			}

			opts := existing.GetOptions()
			silenced := opts.GetSilenced()
			if silenced == nil {
				silenced = make(map[string]int64)
			}
			silenced[scope] = end
			opts.SetSilenced(silenced)
			existing.SetOptions(opts)

			body := datadogV1.NewMonitorUpdateRequest()
			body.SetName(existing.GetName())
			body.SetQuery(existing.GetQuery())
			body.SetMessage(existing.GetMessage())
			body.SetTags(existing.GetTags())
			body.SetOptions(opts)

			_, httpResp, err = mapi.api.UpdateMonitor(mapi.ctx, monitorID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("mute monitor: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "muted monitor %d (scope: %s)\n", monitorID, scope) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().Int64Var(&monitorID, "id", 0, "monitor ID (required)")
	cmd.Flags().StringVar(&scope, "scope", "*", "scope to mute (default: all groups)")
	cmd.Flags().Int64Var(&end, "end", 0, "mute end time as Unix timestamp (0 = indefinite)")
	return cmd
}

func newMonitorsUnmuteCmd(mkAPI func() (*monitorsAPI, error)) *cobra.Command {
	var (
		monitorID int64
		scope     string
	)

	cmd := &cobra.Command{
		Use:   "unmute",
		Short: "Unmute a monitor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if monitorID == 0 {
				return errMonitorIDRequired
			}

			mapi, err := mkAPI()
			if err != nil {
				return err
			}

			existing, httpResp, err := mapi.api.GetMonitor(mapi.ctx, monitorID, *datadogV1.NewGetMonitorOptionalParameters())
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get monitor: %w", err)
			}

			opts := existing.GetOptions()
			if scope == "" {
				// clear all silenced scopes
				opts.SetSilenced(map[string]int64{})
			} else {
				silenced := opts.GetSilenced()
				if silenced == nil {
					silenced = make(map[string]int64)
				}
				delete(silenced, scope)
				opts.SetSilenced(silenced)
			}
			existing.SetOptions(opts)

			body := datadogV1.NewMonitorUpdateRequest()
			body.SetName(existing.GetName())
			body.SetQuery(existing.GetQuery())
			body.SetMessage(existing.GetMessage())
			body.SetTags(existing.GetTags())
			body.SetOptions(opts)

			_, httpResp, err = mapi.api.UpdateMonitor(mapi.ctx, monitorID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("unmute monitor: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "unmuted monitor %d\n", monitorID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().Int64Var(&monitorID, "id", 0, "monitor ID (required)")
	cmd.Flags().StringVar(&scope, "scope", "", "scope to unmute (empty = unmute all)")
	return cmd
}
