package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCommand returns the root cobra command.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "datadog-cli",
		Short:   "Datadog CLI",
		Version: version,
	}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	root.AddCommand(newCompletionCmd(root))
	root.AddCommand(NewAPMCommand())
	root.AddCommand(NewDashboardsCommand())
	root.AddCommand(NewEventsCommand())
	root.AddCommand(NewHostsCommand())
	root.AddCommand(NewLogsCommand())
	root.AddCommand(NewMetricsCommand())
	root.AddCommand(NewMonitorsCommand())
	root.AddCommand(NewRUMCommand())
	root.AddCommand(NewSLOsCommand())
	root.AddCommand(NewUsersCommand())
	return root
}

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "bash",
			Short: "Generate bash completion script",
			RunE: func(c *cobra.Command, args []string) error {
				return root.GenBashCompletion(c.OutOrStdout())
			},
		},
		&cobra.Command{
			Use:   "zsh",
			Short: "Generate zsh completion script",
			RunE: func(c *cobra.Command, args []string) error {
				return root.GenZshCompletion(c.OutOrStdout())
			},
		},
		&cobra.Command{
			Use:   "fish",
			Short: "Generate fish completion script",
			RunE: func(c *cobra.Command, args []string) error {
				return root.GenFishCompletion(c.OutOrStdout(), true)
			},
		},
		&cobra.Command{
			Use:   "powershell",
			Short: "Generate powershell completion script",
			RunE: func(c *cobra.Command, args []string) error {
				return root.GenPowerShellCompletionWithDesc(c.OutOrStdout())
			},
		},
	)
	return cmd
}
