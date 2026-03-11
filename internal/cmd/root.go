package cmd

import "github.com/spf13/cobra"

// NewRootCommand returns the root cobra command.
func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dd",
		Short:   "Datadog CLI",
		Version: version,
	}
	return cmd
}
