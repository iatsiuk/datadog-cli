package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// NewRootCommand returns the root cobra command.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "datadog-cli",
		Short:         "Datadog CLI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	root.AddCommand(newCompletionCmd(root))
	root.AddCommand(NewAPMCommand())
	root.AddCommand(NewCICommand())
	root.AddCommand(NewDashboardsCommand())
	root.AddCommand(NewEventsCommand())
	root.AddCommand(NewIncidentsCommand())
	root.AddCommand(NewHostsCommand())
	root.AddCommand(NewLogsCommand())
	root.AddCommand(NewMetricsCommand())
	root.AddCommand(NewMonitorsCommand())
	root.AddCommand(NewRUMCommand())
	root.AddCommand(NewSecurityCommand())
	root.AddCommand(NewSLOsCommand())
	root.AddCommand(NewSyntheticsCommand())
	root.AddCommand(NewUsersCommand())
	return root
}

// ParseJSONFlag pre-scans args for --json / --json=true so that JSON error
// formatting works even when cobra fails before parsing the flag (e.g. when
// an unknown flag precedes --json in the argument list).
func ParseJSONFlag(args []string) bool {
	result := false
	for _, a := range args {
		if a == "--json" {
			result = true
		} else if strings.HasPrefix(a, "--json=") {
			if v, err := strconv.ParseBool(a[len("--json="):]); err == nil {
				result = v
			}
		} else if a == "--" {
			break
		}
	}
	return result
}

// WriteError writes err to w. When asJSON is true, the output is a single-line
// JSON object {"error":"<text>"}; otherwise the plain error text is written
// followed by a newline. Never includes a cobra Usage block.
func WriteError(w io.Writer, err error, asJSON bool) {
	if err == nil {
		return
	}
	if asJSON {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_, _ = fmt.Fprintln(w, err.Error())
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
