package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type hostsAPI struct {
	api *datadogV1.HostsApi
	ctx context.Context
}

func defaultHostsAPI() (*hostsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &hostsAPI{api: datadogV1.NewHostsApi(c), ctx: ctx}, nil
}

type tagsAPI struct {
	api *datadogV1.TagsApi
	ctx context.Context
}

func defaultTagsAPI() (*tagsAPI, error) { //nolint:unused
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &tagsAPI{api: datadogV1.NewTagsApi(c), ctx: ctx}, nil
}

// NewHostsCommand returns the hosts cobra command group.
func NewHostsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "Manage Datadog hosts",
	}

	tagsCmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage host tags",
	}

	cmd.AddCommand(newHostsListCmd(defaultHostsAPI))
	cmd.AddCommand(newHostsTotalsCmd(defaultHostsAPI))
	cmd.AddCommand(newHostsMuteCmd(defaultHostsAPI))
	cmd.AddCommand(newHostsUnmuteCmd(defaultHostsAPI))
	cmd.AddCommand(tagsCmd)
	return cmd
}

func newHostsTotalsCmd(mkAPI func() (*hostsAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "totals",
		Short: "Get total and active host counts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			hapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := hapi.api.GetHostTotals(hapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get host totals: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"TOTAL_ACTIVE", "TOTAL_UP"}
			rows := [][]string{
				{
					strconv.FormatInt(resp.GetTotalActive(), 10),
					strconv.FormatInt(resp.GetTotalUp(), 10),
				},
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newHostsMuteCmd(mkAPI func() (*hostsAPI, error)) *cobra.Command {
	var (
		name     string
		end      int64
		message  string
		override bool
	)

	cmd := &cobra.Command{
		Use:   "mute",
		Short: "Mute a host",
		RunE: func(cmd *cobra.Command, _ []string) error {
			hapi, err := mkAPI()
			if err != nil {
				return err
			}

			settings := datadogV1.NewHostMuteSettings()
			if end != 0 {
				settings.SetEnd(end)
			}
			if message != "" {
				settings.SetMessage(message)
			}
			if override {
				settings.SetOverride(true)
			}

			resp, httpResp, err := hapi.api.MuteHost(hapi.ctx, name, *settings)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("mute host: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"HOSTNAME", "ACTION", "MESSAGE"}
			rows := [][]string{{resp.GetHostname(), resp.GetAction(), resp.GetMessage()}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "hostname to mute (required)")
	cmd.Flags().Int64Var(&end, "end", 0, "Unix timestamp when mute ends")
	cmd.Flags().StringVar(&message, "message", "", "message associated with the mute")
	cmd.Flags().BoolVar(&override, "override", false, "override existing mute settings")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHostsUnmuteCmd(mkAPI func() (*hostsAPI, error)) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "unmute",
		Short: "Unmute a host",
		RunE: func(cmd *cobra.Command, _ []string) error {
			hapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := hapi.api.UnmuteHost(hapi.ctx, name)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("unmute host: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"HOSTNAME", "ACTION"}
			rows := [][]string{{resp.GetHostname(), resp.GetAction()}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "hostname to unmute (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newHostsListCmd(mkAPI func() (*hostsAPI, error)) *cobra.Command {
	var (
		filter string
		from   int64
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List hosts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			hapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewListHostsOptionalParameters()
			if filter != "" {
				opts = opts.WithFilter(filter)
			}
			if from != 0 {
				opts = opts.WithFrom(from)
			}

			resp, httpResp, err := hapi.api.ListHosts(hapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list hosts: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			hosts := resp.GetHostList()
			if asJSON {
				if hosts == nil {
					hosts = []datadogV1.Host{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), hosts)
			}

			headers := []string{"NAME", "ID", "ALIASES", "APPS", "SOURCES", "UP", "LAST_REPORTED"}
			rows := make([][]string, len(hosts))
			for i, h := range hosts {
				lastReported := ""
				if ts := h.GetLastReportedTime(); ts != 0 {
					lastReported = time.Unix(ts, 0).UTC().Format(time.RFC3339)
				}
				rows[i] = []string{
					h.GetName(),
					strconv.FormatInt(h.GetId(), 10),
					strings.Join(h.GetAliases(), ", "),
					strings.Join(h.GetApps(), ", "),
					strings.Join(h.GetSources(), ", "),
					strconv.FormatBool(h.GetUp()),
					lastReported,
				}
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "", "filter hosts by name, alias, or tag")
	cmd.Flags().Int64Var(&from, "from", 0, "only show hosts active since this Unix timestamp")
	return cmd
}
