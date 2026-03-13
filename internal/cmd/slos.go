package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

func newSLOsHistoryCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show SLO history",
	}
}

func newSLOsCreateCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create an SLO",
	}
}

func newSLOsUpdateCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update an SLO",
	}
}

func newSLOsDeleteCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete an SLO",
	}
}

func newSLOsCanDeleteCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "can-delete",
		Short: "Check if an SLO can be deleted",
	}
}

func newSLOsCorrectionCmd(_ func() (*slosAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "correction",
		Short: "Manage SLO corrections",
	}
	return cmd
}
