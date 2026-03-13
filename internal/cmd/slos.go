package cmd

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
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

func newSLOsListCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List SLOs",
	}
}

func newSLOsShowCmd(_ func() (*slosAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show SLO details",
	}
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
