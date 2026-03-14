package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errSyntheticsLocationNameRequired = errors.New("--name is required")

func newSyntheticsLocationCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "location",
		Short: "Manage Synthetics locations",
	}
	cmd.AddCommand(newSyntheticsLocationListCmd(mkAPI))
	cmd.AddCommand(newSyntheticsLocationDefaultsCmd(mkAPI))
	return cmd
}

func newSyntheticsPrivateLocationCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "private-location",
		Short: "Manage Synthetics private locations",
	}
	cmd.AddCommand(newSyntheticsPrivateLocationShowCmd(mkAPI))
	cmd.AddCommand(newSyntheticsPrivateLocationCreateCmd(mkAPI))
	cmd.AddCommand(newSyntheticsPrivateLocationDeleteCmd(mkAPI))
	return cmd
}

// extractRegion derives region from a location ID like "aws:us-east-1".
// returns empty string for private locations (pl: prefix) as their suffix is not a region.
func extractRegion(id string) string {
	if isPrivateLocation(id) {
		return ""
	}
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// isPrivateLocation returns true if the location ID is a private location.
func isPrivateLocation(id string) bool {
	return strings.HasPrefix(id, "pl:")
}

func newSyntheticsLocationListCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all Synthetics locations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := sapi.api.ListLocations(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list locations: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			locs := resp.GetLocations()

			if asJSON {
				if locs == nil {
					locs = []datadogV1.SyntheticsLocation{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), locs)
			}

			headers := []string{"ID", "NAME", "REGION", "PRIVATE"}
			rows := make([][]string, 0, len(locs))
			for _, l := range locs {
				id := l.GetId()
				private := "false"
				if isPrivateLocation(id) {
					private = "true"
				}
				rows = append(rows, []string{
					id,
					l.GetName(),
					extractRegion(id),
					private,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newSyntheticsLocationDefaultsCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defaults",
		Short: "Show default Synthetics locations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			locs, httpResp, err := sapi.api.GetSyntheticsDefaultLocations(sapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get default locations: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				if locs == nil {
					locs = []string{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), locs)
			}

			headers := []string{"LOCATION"}
			rows := make([][]string, 0, len(locs))
			for _, l := range locs {
				rows = append(rows, []string{l})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}

func newSyntheticsPrivateLocationShowCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a private location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			locID := args[0]

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			loc, httpResp, err := sapi.api.GetPrivateLocation(sapi.ctx, locID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get private location: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), loc)
			}

			return synthPrivateLocationDetailOutput(cmd, loc)
		},
	}
	return cmd
}

func synthPrivateLocationDetailOutput(cmd *cobra.Command, loc datadogV1.SyntheticsPrivateLocation) error {
	rows := [][]string{
		{"ID", loc.GetId()},
		{"NAME", loc.GetName()},
		{"DESCRIPTION", loc.GetDescription()},
		{"TAGS", strings.Join(loc.GetTags(), ", ")},
	}
	return output.PrintTable(cmd.OutOrStdout(), []string{"FIELD", "VALUE"}, rows)
}

func newSyntheticsPrivateLocationCreateCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		name        string
		description string
		tags        string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a private location",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errSyntheticsLocationNameRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			tagList := []string{}
			if tags != "" {
				tagList = splitTrimmed(tags)
			}

			body := datadogV1.NewSyntheticsPrivateLocation(description, name, tagList)

			resp, httpResp, err := sapi.api.CreatePrivateLocation(sapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create private location: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			loc := resp.GetPrivateLocation()
			return synthPrivateLocationDetailOutput(cmd, loc)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "private location name (required)")
	cmd.Flags().StringVar(&description, "description", "", "private location description")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}

func newSyntheticsPrivateLocationDeleteCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a private location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			locID := args[0]
			if !yes {
				return errSyntheticsYesRequired
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := sapi.api.DeletePrivateLocation(sapi.ctx, locID)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete private location: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deleted private location %s\n", locID) //nolint:errcheck
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}
