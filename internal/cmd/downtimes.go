package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errDowntimeIDRequired            = errors.New("--id is required")
	errDowntimeScopeRequired         = errors.New("--scope is required")
	errDowntimeMonitorFlagsExclusive = errors.New("--monitor-id and --monitor-tags are mutually exclusive")
)

type downtimesAPI struct {
	api *datadogV2.DowntimesApi
	ctx context.Context
}

func defaultDowntimesAPI() (*downtimesAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &downtimesAPI{api: datadogV2.NewDowntimesApi(c), ctx: ctx}, nil
}

var (
	errDowntimeYesRequired = errors.New("--yes is required to cancel a downtime")
)

func newDowntimeCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "downtime",
		Short: "Manage Datadog downtimes",
	}
	cmd.AddCommand(newDowntimeListCmd(mkAPI))
	cmd.AddCommand(newDowntimeShowCmd(mkAPI))
	cmd.AddCommand(newDowntimeCreateCmd(mkAPI))
	cmd.AddCommand(newDowntimeUpdateCmd(mkAPI))
	cmd.AddCommand(newDowntimeCancelCmd(mkAPI))
	return cmd
}

func newDowntimeListCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	var currentOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List downtimes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListDowntimesOptionalParameters()
			if currentOnly {
				opts = opts.WithCurrentOnly(true)
			}

			resp, httpResp, err := dapi.api.ListDowntimes(dapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list downtimes: %w", err)
			}

			downtimes := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if downtimes == nil {
					downtimes = []datadogV2.DowntimeResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), downtimes)
			}

			headers := []string{"ID", "SCOPE", "MONITOR_ID", "STATUS", "START", "END"}
			rows := make([][]string, 0, len(downtimes))
			for _, d := range downtimes {
				attrs := d.GetAttributes()

				scope := attrs.GetScope()
				status := string(attrs.GetStatus())

				monitorID := ""
				if mi := attrs.MonitorIdentifier; mi != nil && mi.DowntimeMonitorIdentifierId != nil {
					monitorID = fmt.Sprintf("%d", mi.DowntimeMonitorIdentifierId.MonitorId)
				}

				start := ""
				end := ""
				if sched := attrs.Schedule; sched != nil {
					if sched.DowntimeScheduleOneTimeResponse != nil {
						t := sched.DowntimeScheduleOneTimeResponse.Start
						if !t.IsZero() {
							start = t.Format("2006-01-02 15:04:05")
						}
						if e := sched.DowntimeScheduleOneTimeResponse.End.Get(); e != nil {
							end = e.Format("2006-01-02 15:04:05")
						}
					} else if sched.DowntimeScheduleRecurrencesResponse != nil {
						if cur := sched.DowntimeScheduleRecurrencesResponse.CurrentDowntime; cur != nil {
							if s := cur.Start; s != nil && !s.IsZero() {
								start = s.Format("2006-01-02 15:04:05")
							}
							if e := cur.End.Get(); e != nil {
								end = e.Format("2006-01-02 15:04:05")
							}
						}
					}
				}

				rows = append(rows, []string{d.GetId(), scope, monitorID, status, start, end})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().BoolVar(&currentOnly, "current-only", false, "only return active downtimes")
	return cmd
}

func newDowntimeShowCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	var downtimeID string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a downtime",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if downtimeID == "" {
				return errDowntimeIDRequired
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.GetDowntime(dapi.ctx, downtimeID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get downtime: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}

			return printDowntimeTable(cmd, d)
		},
	}

	cmd.Flags().StringVar(&downtimeID, "id", "", "downtime ID")
	return cmd
}

func printDowntimeTable(cmd *cobra.Command, d datadogV2.DowntimeResponseData) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"SCOPE", attrs.GetScope()},
		{"STATUS", string(attrs.GetStatus())},
	}

	if mi := attrs.MonitorIdentifier; mi != nil {
		if mi.DowntimeMonitorIdentifierId != nil {
			rows = append(rows, []string{"MONITOR_ID", fmt.Sprintf("%d", mi.DowntimeMonitorIdentifierId.MonitorId)})
		} else if mi.DowntimeMonitorIdentifierTags != nil {
			rows = append(rows, []string{"MONITOR_TAGS", strings.Join(mi.DowntimeMonitorIdentifierTags.MonitorTags, ", ")})
		}
	}

	if msg := attrs.GetMessage(); msg != "" {
		rows = append(rows, []string{"MESSAGE", msg})
	}

	if sched := attrs.Schedule; sched != nil {
		if sched.DowntimeScheduleOneTimeResponse != nil {
			t := sched.DowntimeScheduleOneTimeResponse.Start
			if !t.IsZero() {
				rows = append(rows, []string{"START", t.Format("2006-01-02 15:04:05")})
			}
			if e := sched.DowntimeScheduleOneTimeResponse.End.Get(); e != nil {
				rows = append(rows, []string{"END", e.Format("2006-01-02 15:04:05")})
			}
		} else if sched.DowntimeScheduleRecurrencesResponse != nil {
			if cur := sched.DowntimeScheduleRecurrencesResponse.CurrentDowntime; cur != nil {
				if s := cur.Start; s != nil && !s.IsZero() {
					rows = append(rows, []string{"START", s.Format("2006-01-02 15:04:05")})
				}
				if e := cur.End.Get(); e != nil {
					rows = append(rows, []string{"END", e.Format("2006-01-02 15:04:05")})
				}
			}
		}
	}

	if t := attrs.GetCreated(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}

	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newDowntimeCreateCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	var (
		scope       string
		monitorID   int64
		monitorTags []string
		message     string
		startStr    string
		endStr      string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a downtime",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if scope == "" {
				return errDowntimeScopeRequired
			}
			if monitorID != 0 && len(monitorTags) > 0 {
				return errDowntimeMonitorFlagsExclusive
			}

			var monIdentifier datadogV2.DowntimeMonitorIdentifier
			if monitorID != 0 {
				mid := datadogV2.NewDowntimeMonitorIdentifierId(monitorID)
				monIdentifier = datadogV2.DowntimeMonitorIdentifierIdAsDowntimeMonitorIdentifier(mid)
			} else {
				tags := monitorTags
				if len(tags) == 0 {
					tags = []string{"*"}
				}
				mtags := datadogV2.NewDowntimeMonitorIdentifierTags(tags)
				monIdentifier = datadogV2.DowntimeMonitorIdentifierTagsAsDowntimeMonitorIdentifier(mtags)
			}

			attrs := datadogV2.NewDowntimeCreateRequestAttributes(monIdentifier, scope)
			if message != "" {
				attrs.SetMessage(message)
			}

			if startStr != "" || endStr != "" {
				sched := &datadogV2.DowntimeScheduleOneTimeCreateUpdateRequest{}
				if startStr != "" {
					t, err := time.Parse(time.RFC3339, startStr)
					if err != nil {
						return fmt.Errorf("parse --start: %w", err)
					}
					sched.Start.Set(&t)
				}
				if endStr != "" {
					t, err := time.Parse(time.RFC3339, endStr)
					if err != nil {
						return fmt.Errorf("parse --end: %w", err)
					}
					sched.End.Set(&t)
				}
				schedReq := datadogV2.DowntimeScheduleOneTimeCreateUpdateRequestAsDowntimeScheduleCreateRequest(sched)
				attrs.Schedule = &schedReq
			}

			data := datadogV2.NewDowntimeCreateRequestData(*attrs, datadogV2.DOWNTIMERESOURCETYPE_DOWNTIME)
			body := datadogV2.NewDowntimeCreateRequest(*data)

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.CreateDowntime(dapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create downtime: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}

			return printDowntimeTable(cmd, d)
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "scope for the downtime (required)")
	cmd.Flags().Int64Var(&monitorID, "monitor-id", 0, "mute specific monitor by ID")
	cmd.Flags().StringArrayVar(&monitorTags, "monitor-tags", nil, "mute monitors matching tags (default: all)")
	cmd.Flags().StringVar(&message, "message", "", "notification message")
	cmd.Flags().StringVar(&startStr, "start", "", "start time (RFC3339, e.g. 2026-03-13T10:00:00Z)")
	cmd.Flags().StringVar(&endStr, "end", "", "end time (RFC3339)")
	return cmd
}

func newDowntimeUpdateCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	var (
		downtimeID string
		scope      string
		message    string
		startStr   string
		endStr     string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a downtime",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if downtimeID == "" {
				return errDowntimeIDRequired
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewDowntimeUpdateRequestAttributes()

			if cmd.Flags().Changed("scope") {
				attrs.SetScope(scope)
			}
			if cmd.Flags().Changed("message") {
				attrs.Message.Set(&message)
			}
			if cmd.Flags().Changed("start") || cmd.Flags().Changed("end") {
				sched := &datadogV2.DowntimeScheduleOneTimeCreateUpdateRequest{}
				if cmd.Flags().Changed("start") {
					t, err := time.Parse(time.RFC3339, startStr)
					if err != nil {
						return fmt.Errorf("parse --start: %w", err)
					}
					sched.Start.Set(&t)
				}
				if cmd.Flags().Changed("end") {
					t, err := time.Parse(time.RFC3339, endStr)
					if err != nil {
						return fmt.Errorf("parse --end: %w", err)
					}
					sched.End.Set(&t)
				}
				schedReq := datadogV2.DowntimeScheduleOneTimeCreateUpdateRequestAsDowntimeScheduleUpdateRequest(sched)
				attrs.Schedule = &schedReq
			}

			data := datadogV2.NewDowntimeUpdateRequestData(*attrs, downtimeID, datadogV2.DOWNTIMERESOURCETYPE_DOWNTIME)
			body := datadogV2.NewDowntimeUpdateRequest(*data)

			resp, httpResp, err := dapi.api.UpdateDowntime(dapi.ctx, downtimeID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update downtime: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}

			return printDowntimeTable(cmd, d)
		},
	}

	cmd.Flags().StringVar(&downtimeID, "id", "", "downtime ID")
	cmd.Flags().StringVar(&scope, "scope", "", "new scope for the downtime")
	cmd.Flags().StringVar(&message, "message", "", "notification message")
	cmd.Flags().StringVar(&startStr, "start", "", "start time (RFC3339)")
	cmd.Flags().StringVar(&endStr, "end", "", "end time (RFC3339)")
	return cmd
}

func newDowntimeCancelCmd(mkAPI func() (*downtimesAPI, error)) *cobra.Command {
	var (
		downtimeID string
		yes        bool
	)

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a downtime",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if downtimeID == "" {
				return errDowntimeIDRequired
			}
			if !yes {
				return errDowntimeYesRequired
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := dapi.api.CancelDowntime(dapi.ctx, downtimeID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("cancel downtime: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "downtime %s cancelled\n", downtimeID)
			return nil
		},
	}

	cmd.Flags().StringVar(&downtimeID, "id", "", "downtime ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm cancellation")
	return cmd
}
