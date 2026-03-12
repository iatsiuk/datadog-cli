package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type dashboardsAPI struct {
	api *datadogV1.DashboardsApi
	ctx context.Context
}

func defaultDashboardsAPI() (*dashboardsAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &dashboardsAPI{api: datadogV1.NewDashboardsApi(c), ctx: ctx}, nil
}

// NewDashboardsCommand returns the dashboards cobra command group.
func NewDashboardsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Manage Datadog dashboards",
	}
	cmd.AddCommand(newDashboardsListCmd(defaultDashboardsAPI))
	cmd.AddCommand(newDashboardsShowCmd(defaultDashboardsAPI))
	cmd.AddCommand(newDashboardsCreateCmd(defaultDashboardsAPI))
	cmd.AddCommand(newDashboardsUpdateCmd(defaultDashboardsAPI))
	cmd.AddCommand(newDashboardsDeleteCmd(defaultDashboardsAPI))
	return cmd
}

func newDashboardsShowCmd(mkAPI func() (*dashboardsAPI, error)) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show a dashboard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.GetDashboard(dapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get dashboard: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			created := ""
			if t := resp.CreatedAt; t != nil {
				created = t.UTC().Format("2006-01-02")
			}
			modified := ""
			if t := resp.ModifiedAt; t != nil {
				modified = t.UTC().Format("2006-01-02")
			}
			headers := []string{"ID", "TITLE", "LAYOUT", "URL", "CREATED", "MODIFIED"}
			rows := [][]string{{
				resp.GetId(),
				resp.GetTitle(),
				string(resp.GetLayoutType()),
				resp.GetUrl(),
				created,
				modified,
			}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "dashboard ID (required)")
	return cmd
}

func newDashboardsCreateCmd(mkAPI func() (*dashboardsAPI, error)) *cobra.Command {
	var (
		title       string
		layoutType  string
		description string
		tags        string
		widgetsJSON string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a dashboard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			if layoutType == "" {
				return fmt.Errorf("--layout-type is required")
			}

			body := datadogV1.Dashboard{
				Title:      title,
				LayoutType: datadogV1.DashboardLayoutType(layoutType),
			}

			if description != "" {
				body.Description = *datadog.NewNullableString(&description)
			}

			if tags != "" {
				tagList := strings.Split(tags, ",")
				body.Tags = *datadog.NewNullableList(&tagList)
			}

			if widgetsJSON != "" {
				var widgets []datadogV1.Widget
				if err := json.Unmarshal([]byte(widgetsJSON), &widgets); err != nil {
					return fmt.Errorf("parse --widgets-json: %w", err)
				}
				body.Widgets = widgets
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.CreateDashboard(dapi.ctx, body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create dashboard: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"ID", "TITLE", "LAYOUT", "URL"}
			rows := [][]string{{
				resp.GetId(),
				resp.GetTitle(),
				string(resp.GetLayoutType()),
				resp.GetUrl(),
			}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "dashboard title (required)")
	cmd.Flags().StringVar(&layoutType, "layout-type", "", "layout type: ordered or free (required)")
	cmd.Flags().StringVar(&description, "description", "", "dashboard description")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags (e.g. team:infra,env:prod)")
	cmd.Flags().StringVar(&widgetsJSON, "widgets-json", "", "widgets as JSON array (inline or @file)")
	return cmd
}

func newDashboardsUpdateCmd(mkAPI func() (*dashboardsAPI, error)) *cobra.Command {
	var (
		id          string
		bodyJSON    string
		title       string
		layoutType  string
		description string
		tags        string
		widgetsJSON string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a dashboard (full replace)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}

			var body datadogV1.Dashboard

			if bodyJSON != "" {
				if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
					return fmt.Errorf("parse --body: %w", err)
				}
			} else {
				if title == "" {
					return fmt.Errorf("--title is required (or use --body for full JSON)")
				}
				if layoutType == "" {
					return fmt.Errorf("--layout-type is required (or use --body for full JSON)")
				}
				body.Title = title
				body.LayoutType = datadogV1.DashboardLayoutType(layoutType)

				if description != "" {
					body.Description = *datadog.NewNullableString(&description)
				}
				if tags != "" {
					tagList := strings.Split(tags, ",")
					body.Tags = *datadog.NewNullableList(&tagList)
				}
				if widgetsJSON != "" {
					var widgets []datadogV1.Widget
					if err := json.Unmarshal([]byte(widgetsJSON), &widgets); err != nil {
						return fmt.Errorf("parse --widgets-json: %w", err)
					}
					body.Widgets = widgets
				}
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.UpdateDashboard(dapi.ctx, id, body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update dashboard: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			headers := []string{"ID", "TITLE", "LAYOUT", "URL"}
			rows := [][]string{{
				resp.GetId(),
				resp.GetTitle(),
				string(resp.GetLayoutType()),
				resp.GetUrl(),
			}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "dashboard ID (required)")
	cmd.Flags().StringVar(&bodyJSON, "body", "", "full dashboard JSON (replaces all other flags)")
	cmd.Flags().StringVar(&title, "title", "", "dashboard title")
	cmd.Flags().StringVar(&layoutType, "layout-type", "", "layout type: ordered or free")
	cmd.Flags().StringVar(&description, "description", "", "dashboard description")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	cmd.Flags().StringVar(&widgetsJSON, "widgets-json", "", "widgets as JSON array")
	return cmd
}

func newDashboardsDeleteCmd(mkAPI func() (*dashboardsAPI, error)) *cobra.Command {
	var (
		id  string
		yes bool
	)
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a dashboard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			if !yes {
				return fmt.Errorf("--yes is required to confirm deletion")
			}

			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.DeleteDashboard(dapi.ctx, id)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete dashboard: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "deleted dashboard: %s\n", resp.GetDeletedDashboardId()) //nolint:errcheck
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "dashboard ID (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

func newDashboardsListCmd(mkAPI func() (*dashboardsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := dapi.api.ListDashboards(dapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list dashboards: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}

			dashboards := resp.GetDashboards()
			if asJSON {
				if dashboards == nil {
					dashboards = []datadogV1.DashboardSummaryDefinition{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), dashboards)
			}

			headers := []string{"ID", "TITLE", "LAYOUT", "URL", "CREATED", "MODIFIED"}
			var rows [][]string
			for _, d := range dashboards {
				id := d.GetId()
				title := d.GetTitle()
				layout := string(d.GetLayoutType())
				url := d.GetUrl()
				created := ""
				if t := d.CreatedAt; t != nil {
					created = t.UTC().Format("2006-01-02")
				}
				modified := ""
				if t := d.ModifiedAt; t != nil {
					modified = t.UTC().Format("2006-01-02")
				}
				rows = append(rows, []string{id, title, layout, url, created, modified})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
	return cmd
}
