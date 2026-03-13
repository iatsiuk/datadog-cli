package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

var (
	errIncidentQueryRequired = errors.New("--query is required")
	errIncidentTitleRequired = errors.New("--title is required")
	errIncidentYesRequired   = errors.New("--yes is required to delete an incident")
)

type incidentsAPI struct {
	api         *datadogV2.IncidentsApi
	servicesApi *datadogV2.IncidentServicesApi
	teamsApi    *datadogV2.IncidentTeamsApi
	ctx         context.Context
}

func defaultIncidentsAPI() (*incidentsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	ddCfg := datadog.NewConfiguration()
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidents", true)
	ddCfg.SetUnstableOperationEnabled("v2.SearchIncidents", true)
	ddCfg.SetUnstableOperationEnabled("v2.GetIncident", true)
	ddCfg.SetUnstableOperationEnabled("v2.CreateIncident", true)
	ddCfg.SetUnstableOperationEnabled("v2.UpdateIncident", true)
	ddCfg.SetUnstableOperationEnabled("v2.DeleteIncident", true)
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidentTodos", true)
	ddCfg.SetUnstableOperationEnabled("v2.GetIncidentTodo", true)
	ddCfg.SetUnstableOperationEnabled("v2.CreateIncidentTodo", true)
	ddCfg.SetUnstableOperationEnabled("v2.UpdateIncidentTodo", true)
	ddCfg.SetUnstableOperationEnabled("v2.DeleteIncidentTodo", true)
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidentIntegrations", true)
	ddCfg.SetUnstableOperationEnabled("v2.GetIncidentIntegration", true)
	ddCfg.SetUnstableOperationEnabled("v2.CreateIncidentIntegration", true)
	ddCfg.SetUnstableOperationEnabled("v2.UpdateIncidentIntegration", true)
	ddCfg.SetUnstableOperationEnabled("v2.DeleteIncidentIntegration", true)
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidentTypes", true)
	ddCfg.SetUnstableOperationEnabled("v2.GetIncidentType", true)
	ddCfg.SetUnstableOperationEnabled("v2.CreateIncidentType", true)
	ddCfg.SetUnstableOperationEnabled("v2.UpdateIncidentType", true)
	ddCfg.SetUnstableOperationEnabled("v2.DeleteIncidentType", true)
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidentServices", true)
	ddCfg.SetUnstableOperationEnabled("v2.GetIncidentService", true)
	ddCfg.SetUnstableOperationEnabled("v2.CreateIncidentService", true)
	ddCfg.SetUnstableOperationEnabled("v2.UpdateIncidentService", true)
	ddCfg.SetUnstableOperationEnabled("v2.DeleteIncidentService", true)
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidentTeams", true)
	ddCfg.SetUnstableOperationEnabled("v2.GetIncidentTeam", true)
	ddCfg.SetUnstableOperationEnabled("v2.CreateIncidentTeam", true)
	ddCfg.SetUnstableOperationEnabled("v2.UpdateIncidentTeam", true)
	ddCfg.SetUnstableOperationEnabled("v2.DeleteIncidentTeam", true)
	c, ctx := client.NewWithConfig(ddCfg, cfg)
	return &incidentsAPI{
		api:         datadogV2.NewIncidentsApi(c),
		servicesApi: datadogV2.NewIncidentServicesApi(c),
		teamsApi:    datadogV2.NewIncidentTeamsApi(c),
		ctx:         ctx,
	}, nil
}

// NewIncidentsCommand returns the incidents command group.
func NewIncidentsCommand(mkAPI ...func() (*incidentsAPI, error)) *cobra.Command {
	var mk func() (*incidentsAPI, error)
	if len(mkAPI) > 0 && mkAPI[0] != nil {
		mk = mkAPI[0]
	} else {
		mk = defaultIncidentsAPI
	}

	cmd := &cobra.Command{
		Use:   "incidents",
		Short: "Manage Datadog incidents",
	}
	cmd.AddCommand(newIncidentsListCmd(mk))
	cmd.AddCommand(newIncidentsSearchCmd(mk))
	cmd.AddCommand(newIncidentsShowCmd(mk))
	cmd.AddCommand(newIncidentsCreateCmd(mk))
	cmd.AddCommand(newIncidentsUpdateCmd(mk))
	cmd.AddCommand(newIncidentsDeleteCmd(mk))
	cmd.AddCommand(newIncidentsTodoCmd(mk))
	cmd.AddCommand(newIncidentsIntegrationCmd(mk))
	cmd.AddCommand(newIncidentsTypeCmd(mk))
	cmd.AddCommand(newIncidentsServiceCmd(mk))
	cmd.AddCommand(newIncidentsTeamCmd(mk))
	return cmd
}

func incidentTableRows(incidents []datadogV2.IncidentResponseData) [][]string {
	rows := make([][]string, 0, len(incidents))
	for _, inc := range incidents {
		attrs := inc.GetAttributes()
		id := inc.GetId()
		title := attrs.GetTitle()
		severity := string(attrs.GetSeverity())
		state := attrs.GetState()
		created := ""
		if t := attrs.GetCreated(); !t.IsZero() {
			created = t.Format("2006-01-02 15:04:05")
		}
		commander := ""
		if rel := inc.GetRelationships(); rel.CommanderUser.IsSet() {
			if cu := rel.CommanderUser.Get(); cu != nil {
				if data := cu.Data.Get(); data != nil {
					commander = data.GetId()
				}
			}
		}
		rows = append(rows, []string{id, title, severity, state, created, commander})
	}
	return rows
}

func newIncidentsListCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var pageSize int64

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV2.NewListIncidentsOptionalParameters()
			if pageSize > 0 {
				opts = opts.WithPageSize(pageSize)
			}

			resp, httpResp, err := iapi.api.ListIncidents(iapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list incidents: %w", err)
			}

			incidents := resp.GetData()

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if incidents == nil {
					incidents = []datadogV2.IncidentResponseData{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), incidents)
			}

			headers := []string{"ID", "TITLE", "SEVERITY", "STATUS", "CREATED", "COMMANDER"}
			return output.PrintTable(cmd.OutOrStdout(), headers, incidentTableRows(incidents))
		},
	}

	cmd.Flags().Int64Var(&pageSize, "page-size", 0, "max number of incidents to return")
	return cmd
}

func printIncidentDetail(cmd *cobra.Command, d datadogV2.IncidentResponseData) error {
	attrs := d.GetAttributes()
	rows := [][]string{
		{"ID", d.GetId()},
		{"TITLE", attrs.GetTitle()},
		{"SEVERITY", string(attrs.GetSeverity())},
		{"STATUS", attrs.GetState()},
	}
	if t := attrs.GetCreated(); !t.IsZero() {
		rows = append(rows, []string{"CREATED", t.Format("2006-01-02 15:04:05")})
	}
	if t := attrs.GetResolved(); !t.IsZero() {
		rows = append(rows, []string{"RESOLVED", t.Format("2006-01-02 15:04:05")})
	}
	if rel := d.GetRelationships(); rel.CommanderUser.IsSet() {
		if cu := rel.CommanderUser.Get(); cu != nil {
			if data := cu.Data.Get(); data != nil {
				rows = append(rows, []string{"COMMANDER", data.GetId()})
			}
		}
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newIncidentsShowCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show incident details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.GetIncident(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get incident: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentDetail(cmd, d)
		},
	}
}

func newIncidentsCreateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		title     string
		severity  string
		commander string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an incident",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" {
				return errIncidentTitleRequired
			}

			attrs := datadogV2.NewIncidentCreateAttributes(false, title)
			if severity != "" {
				sev := severity
				f := datadogV2.NewIncidentFieldAttributesSingleValue()
				f.Value.Set(&sev)
				attrs.Fields = map[string]datadogV2.IncidentFieldAttributes{
					"severity": datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(f),
				}
			}

			data := datadogV2.NewIncidentCreateData(*attrs, datadogV2.INCIDENTTYPE_INCIDENTS)
			if commander != "" {
				userData := datadogV2.NewNullableRelationshipToUserData(commander, datadogV2.USERSTYPE_USERS)
				var nullableData datadogV2.NullableNullableRelationshipToUserData
				nullableData.Set(userData)
				userRel := datadogV2.NewNullableRelationshipToUser(nullableData)
				var commanderNullable datadogV2.NullableNullableRelationshipToUser
				commanderNullable.Set(userRel)
				rels := datadogV2.NewIncidentCreateRelationships(commanderNullable)
				data.Relationships = rels
			}

			body := datadogV2.NewIncidentCreateRequest(*data)

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.CreateIncident(iapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create incident: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "incident title (required)")
	cmd.Flags().StringVar(&severity, "severity", "", "severity (SEV-0..SEV-5)")
	cmd.Flags().StringVar(&commander, "commander", "", "commander user ID")
	return cmd
}

func newIncidentsUpdateCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var (
		title    string
		severity string
		status   string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			incidentID := args[0]

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			attrs := datadogV2.NewIncidentUpdateAttributes()
			if cmd.Flags().Changed("title") {
				attrs.SetTitle(title)
			}

			fields := map[string]datadogV2.IncidentFieldAttributes{}
			if cmd.Flags().Changed("severity") {
				sev := severity
				f := datadogV2.NewIncidentFieldAttributesSingleValue()
				f.Value.Set(&sev)
				fields["severity"] = datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(f)
			}
			if cmd.Flags().Changed("status") {
				st := status
				f := datadogV2.NewIncidentFieldAttributesSingleValue()
				f.Value.Set(&st)
				fields["state"] = datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(f)
			}
			if len(fields) > 0 {
				attrs.SetFields(fields)
			}

			data := datadogV2.NewIncidentUpdateData(incidentID, datadogV2.INCIDENTTYPE_INCIDENTS)
			data.Attributes = attrs
			body := datadogV2.NewIncidentUpdateRequest(*data)

			resp, httpResp, err := iapi.api.UpdateIncident(iapi.ctx, incidentID, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update incident: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			d := resp.GetData()
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), d)
			}
			return printIncidentDetail(cmd, d)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "new incident title")
	cmd.Flags().StringVar(&severity, "severity", "", "new severity (SEV-0..SEV-5)")
	cmd.Flags().StringVar(&status, "status", "", "new status (active, stable, resolved)")
	return cmd
}

func newIncidentsDeleteCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an incident",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return errIncidentYesRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.api.DeleteIncident(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete incident: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "incident %s deleted\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newIncidentsSearchCmd(mkAPI func() (*incidentsAPI, error)) *cobra.Command {
	var query string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search incidents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query == "" {
				return errIncidentQueryRequired
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.SearchIncidents(iapi.ctx, query)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("search incidents: %w", err)
			}

			searchData := resp.GetData()
			searchAttrs := searchData.GetAttributes()
			searchIncidents := searchAttrs.GetIncidents()
			incidents := make([]datadogV2.IncidentResponseData, 0, len(searchIncidents))
			for _, si := range searchIncidents {
				incidents = append(incidents, si.GetData())
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), incidents)
			}

			headers := []string{"ID", "TITLE", "SEVERITY", "STATUS", "CREATED", "COMMANDER"}
			return output.PrintTable(cmd.OutOrStdout(), headers, incidentTableRows(incidents))
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "search query (required)")
	return cmd
}
