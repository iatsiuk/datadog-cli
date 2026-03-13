package cmd

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
)

type usersAPI struct {
	api *datadogV2.UsersApi
	ctx context.Context
}

func defaultUsersAPI() (*usersAPI, error) { //nolint:unused
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &usersAPI{api: datadogV2.NewUsersApi(c), ctx: ctx}, nil
}

// NewUsersCommand returns the users cobra command group.
func NewUsersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage Datadog users, roles, and teams",
	}
	return cmd
}
