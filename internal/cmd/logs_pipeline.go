package cmd

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsPipelineAPI struct {
	api *datadogV1.LogsPipelinesApi
	ctx context.Context
}

func defaultLogsPipelineAPI() (*logsPipelineAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &logsPipelineAPI{api: datadogV1.NewLogsPipelinesApi(c), ctx: ctx}, nil
}

func newLogsPipelineCmd(mkAPI func() (*logsPipelineAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage log pipelines",
	}
	cmd.AddCommand(newLogsPipelineListCmd(mkAPI))
	cmd.AddCommand(newLogsPipelineShowCmd(mkAPI))
	cmd.AddCommand(newLogsPipelineCreateCmd(mkAPI))
	cmd.AddCommand(newLogsPipelineUpdateCmd(mkAPI))
	cmd.AddCommand(newLogsPipelineDeleteCmd(mkAPI))
	return cmd
}

func newLogsPipelineListCmd(mkAPI func() (*logsPipelineAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List log pipelines",
		RunE: func(cmd *cobra.Command, _ []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := papi.api.ListLogsPipelines(papi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list log pipelines: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				if resp == nil {
					resp = []datadogV1.LogsPipeline{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"ID", "NAME", "ENABLED", "FILTER"}
			var rows [][]string
			for _, p := range resp {
				enabled := fmt.Sprintf("%v", p.GetIsEnabled())
				filter := ""
				if p.Filter != nil {
					filter = p.Filter.GetQuery()
				}
				rows = append(rows, []string{
					p.GetId(),
					p.GetName(),
					enabled,
					filter,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsPipelineShowCmd(mkAPI func() (*logsPipelineAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a log pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := papi.api.GetLogsPipeline(papi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get log pipeline: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			filter := ""
			if resp.Filter != nil {
				filter = resp.Filter.GetQuery()
			}
			headers := []string{"ID", "NAME", "ENABLED", "FILTER"}
			rows := [][]string{{resp.GetId(), resp.GetName(), fmt.Sprintf("%v", resp.GetIsEnabled()), filter}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsPipelineCreateCmd(mkAPI func() (*logsPipelineAPI, error)) *cobra.Command {
	var (
		name    string
		filter  string
		enabled bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a log pipeline",
		RunE: func(cmd *cobra.Command, _ []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}

			body := datadogV1.NewLogsPipeline(name)
			body.SetIsEnabled(enabled)
			if filter != "" {
				f := datadogV1.NewLogsFilter()
				f.SetQuery(filter)
				body.SetFilter(*f)
			}

			resp, httpResp, err := papi.api.CreateLogsPipeline(papi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create log pipeline: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "pipeline name (required)")
	cmd.Flags().StringVar(&filter, "filter", "", "log filter query")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "whether pipeline is enabled")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newLogsPipelineUpdateCmd(mkAPI func() (*logsPipelineAPI, error)) *cobra.Command {
	var (
		name    string
		filter  string
		enabled bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a log pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			papi, err := mkAPI()
			if err != nil {
				return err
			}

			// fetch existing pipeline to preserve processors (PUT replaces entire object)
			existing, httpResp, err := papi.api.GetLogsPipeline(papi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get log pipeline: %w", err)
			}

			pipelineName := name
			if !cmd.Flags().Changed("name") {
				pipelineName = existing.GetName()
			}
			body := datadogV1.NewLogsPipeline(pipelineName)
			if existing.HasProcessors() {
				body.SetProcessors(existing.GetProcessors())
			}
			if cmd.Flags().Changed("enabled") {
				body.SetIsEnabled(enabled)
			} else {
				body.SetIsEnabled(existing.GetIsEnabled())
			}
			if filter != "" {
				f := datadogV1.NewLogsFilter()
				f.SetQuery(filter)
				body.SetFilter(*f)
			} else if existing.Filter != nil {
				body.SetFilter(*existing.Filter)
			}

			resp, httpResp, err := papi.api.UpdateLogsPipeline(papi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update log pipeline: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "pipeline name")
	cmd.Flags().StringVar(&filter, "filter", "", "log filter query")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "whether pipeline is enabled")
	return cmd
}

func newLogsPipelineDeleteCmd(mkAPI func() (*logsPipelineAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a log pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of pipeline %q", args[0])
			}

			papi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := papi.api.DeleteLogsPipeline(papi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete log pipeline: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted pipeline %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
