package cmd

import (
	"context"
	"fmt"
	"sort"
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

func defaultTagsAPI() (*tagsAPI, error) {
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
	tagsCmd.AddCommand(newTagsListCmd(defaultTagsAPI))
	tagsCmd.AddCommand(newTagsShowCmd(defaultTagsAPI))
	tagsCmd.AddCommand(newTagsCreateCmd(defaultTagsAPI))
	tagsCmd.AddCommand(newTagsUpdateCmd(defaultTagsAPI))
	tagsCmd.AddCommand(newTagsDeleteCmd(defaultTagsAPI))

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
		count  int64
		start  int64
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
			if count != 0 {
				opts = opts.WithCount(count)
			}
			if start != 0 {
				opts = opts.WithStart(start)
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
	cmd.Flags().Int64Var(&count, "count", 0, "max number of hosts to return")
	cmd.Flags().Int64Var(&start, "start", 0, "starting offset for pagination")
	return cmd
}

func newTagsShowCmd(mkAPI func() (*tagsAPI, error)) *cobra.Command {
	var (
		name   string
		source string
	)

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show tags for a specific host",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewGetHostTagsOptionalParameters()
			if source != "" {
				opts = opts.WithSource(source)
			}

			resp, httpResp, err := tapi.api.GetHostTags(tapi.ctx, name, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get host tags: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"HOST", "TAGS"}
			rows := [][]string{{resp.GetHost(), strings.Join(resp.GetTags(), ", ")}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "hostname to show tags for (required)")
	cmd.Flags().StringVar(&source, "source", "", "filter tags by source (e.g. users, datadog, chef)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newTagsCreateCmd(mkAPI func() (*tagsAPI, error)) *cobra.Command {
	var (
		name   string
		tags   string
		source string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Add tags to a host",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			tagList := parseTags(tags)
			if len(tagList) == 0 {
				return fmt.Errorf("--tags: no valid tags provided")
			}
			body := datadogV1.NewHostTags()
			body.SetTags(tagList)

			opts := datadogV1.NewCreateHostTagsOptionalParameters()
			if source != "" {
				opts = opts.WithSource(source)
			}

			resp, httpResp, err := tapi.api.CreateHostTags(tapi.ctx, name, *body, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create host tags: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"HOST", "TAGS"}
			rows := [][]string{{resp.GetHost(), strings.Join(resp.GetTags(), ", ")}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "hostname to add tags to (required)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated list of tags (required)")
	cmd.Flags().StringVar(&source, "source", "", "tag source (e.g. users, datadog, chef)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("tags")
	return cmd
}

func newTagsUpdateCmd(mkAPI func() (*tagsAPI, error)) *cobra.Command {
	var (
		name   string
		tags   string
		source string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update tags for a host (replaces existing tags)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			tagList := parseTags(tags)
			if len(tagList) == 0 {
				return fmt.Errorf("--tags: no valid tags provided")
			}
			body := datadogV1.NewHostTags()
			body.SetTags(tagList)

			opts := datadogV1.NewUpdateHostTagsOptionalParameters()
			if source != "" {
				opts = opts.WithSource(source)
			}

			resp, httpResp, err := tapi.api.UpdateHostTags(tapi.ctx, name, *body, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update host tags: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"HOST", "TAGS"}
			rows := [][]string{{resp.GetHost(), strings.Join(resp.GetTags(), ", ")}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "hostname to update tags for (required)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated list of tags (required)")
	cmd.Flags().StringVar(&source, "source", "", "tag source (e.g. users, datadog, chef)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("tags")
	return cmd
}

func newTagsDeleteCmd(mkAPI func() (*tagsAPI, error)) *cobra.Command {
	var (
		name   string
		yes    bool
		source string
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete all tags from a host",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("must pass --yes to confirm deletion")
			}

			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewDeleteHostTagsOptionalParameters()
			if source != "" {
				opts = opts.WithSource(source)
			}

			httpResp, err := tapi.api.DeleteHostTags(tapi.ctx, name, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete host tags: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "hostname to delete tags from (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	cmd.Flags().StringVar(&source, "source", "", "tag source to delete (e.g. users, datadog, chef)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newTagsListCmd(mkAPI func() (*tagsAPI, error)) *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all host tags",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tapi, err := mkAPI()
			if err != nil {
				return err
			}

			opts := datadogV1.NewListHostTagsOptionalParameters()
			if source != "" {
				opts = opts.WithSource(source)
			}

			resp, httpResp, err := tapi.api.ListHostTags(tapi.ctx, *opts)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list host tags: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			tags := resp.GetTags()
			keys := make([]string, 0, len(tags))
			for tag := range tags {
				keys = append(keys, tag)
			}
			sort.Strings(keys)
			headers := []string{"TAG", "HOSTS"}
			rows := make([][]string, 0, len(tags))
			for _, tag := range keys {
				rows = append(rows, []string{tag, strings.Join(tags[tag], ", ")})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "filter tags by source (e.g. users, datadog, chef)")
	return cmd
}

// parseTags splits a comma-separated tag string and filters empty elements.
func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	result := parts[:0]
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
