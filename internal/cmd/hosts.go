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
	cmd.AddCommand(tagsCmd)
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
