package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsIndexAPI struct {
	api *datadogV1.LogsIndexesApi
	ctx context.Context
}

func defaultLogsIndexAPI() (*logsIndexAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &logsIndexAPI{api: datadogV1.NewLogsIndexesApi(c), ctx: ctx}, nil
}

func newLogsIndexCmd(mkAPI func() (*logsIndexAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage log indexes",
	}
	cmd.AddCommand(newLogsIndexListCmd(mkAPI))
	cmd.AddCommand(newLogsIndexShowCmd(mkAPI))
	cmd.AddCommand(newLogsIndexCreateCmd(mkAPI))
	cmd.AddCommand(newLogsIndexUpdateCmd(mkAPI))
	cmd.AddCommand(newLogsIndexDeleteCmd(mkAPI))
	return cmd
}

func newLogsIndexListCmd(mkAPI func() (*logsIndexAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List log indexes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.ListLogIndexes(iapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list log indexes: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			indexes := resp.GetIndexes()
			if asJSON {
				if indexes == nil {
					indexes = []datadogV1.LogsIndex{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), indexes)
			}

			headers := []string{"NAME", "FILTER", "RETENTION"}
			var rows [][]string
			for _, idx := range indexes {
				retention := ""
				if idx.NumRetentionDays != nil {
					retention = strconv.FormatInt(*idx.NumRetentionDays, 10)
				}
				rows = append(rows, []string{
					idx.GetName(),
					idx.Filter.GetQuery(),
					retention,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsIndexShowCmd(mkAPI func() (*logsIndexAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a log index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := iapi.api.GetLogsIndex(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get log index: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			retention := ""
			if resp.NumRetentionDays != nil {
				retention = strconv.FormatInt(*resp.NumRetentionDays, 10)
			}
			headers := []string{"NAME", "FILTER", "RETENTION"}
			rows := [][]string{{resp.GetName(), resp.Filter.GetQuery(), retention}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsIndexCreateCmd(mkAPI func() (*logsIndexAPI, error)) *cobra.Command {
	var (
		name      string
		filter    string
		retention int64
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a log index",
		RunE: func(cmd *cobra.Command, _ []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			f := datadogV1.NewLogsFilter()
			if filter != "" {
				f.SetQuery(filter)
			}
			body := datadogV1.NewLogsIndex(*f, name)
			if retention > 0 {
				body.SetNumRetentionDays(retention)
			}

			resp, httpResp, err := iapi.api.CreateLogsIndex(iapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create log index: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "index name (required)")
	cmd.Flags().StringVar(&filter, "filter", "", "log filter query")
	cmd.Flags().Int64Var(&retention, "retention", 0, "retention in days")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newLogsIndexUpdateCmd(mkAPI func() (*logsIndexAPI, error)) *cobra.Command {
	var (
		filter    string
		retention int64
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a log index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			f := datadogV1.NewLogsFilter()
			if filter != "" {
				f.SetQuery(filter)
			}
			body := datadogV1.NewLogsIndexUpdateRequest(*f)
			if retention > 0 {
				body.SetNumRetentionDays(retention)
			}

			resp, httpResp, err := iapi.api.UpdateLogsIndex(iapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update log index: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "log filter query (required)")
	cmd.Flags().Int64Var(&retention, "retention", 0, "retention in days")
	_ = cmd.MarkFlagRequired("filter")
	return cmd
}

func newLogsIndexDeleteCmd(mkAPI func() (*logsIndexAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a log index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of index %q", args[0])
			}

			iapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := iapi.api.DeleteLogsIndex(iapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete log index: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted index %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
