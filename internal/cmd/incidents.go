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

var errIncidentQueryRequired = errors.New("--query is required")

type incidentsAPI struct {
	api *datadogV2.IncidentsApi
	ctx context.Context
}

func defaultIncidentsAPI() (*incidentsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	ddCfg := datadog.NewConfiguration()
	ddCfg.SetUnstableOperationEnabled("v2.ListIncidents", true)
	ddCfg.SetUnstableOperationEnabled("v2.SearchIncidents", true)
	c, ctx := client.NewWithConfig(ddCfg, cfg)
	return &incidentsAPI{api: datadogV2.NewIncidentsApi(c), ctx: ctx}, nil
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
