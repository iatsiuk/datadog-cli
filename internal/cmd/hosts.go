package cmd

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
)

type hostsAPI struct {
	api *datadogV1.HostsApi
	ctx context.Context
}

func defaultHostsAPI() (*hostsAPI, error) { //nolint:unused
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

	cmd.AddCommand(tagsCmd)
	return cmd
}
