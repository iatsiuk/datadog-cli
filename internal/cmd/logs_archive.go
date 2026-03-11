package cmd

import (
	"context"
	"fmt"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/client"
	"github.com/iatsiuk/datadog-cli/internal/config"
	"github.com/iatsiuk/datadog-cli/internal/output"
)

type logsArchiveAPI struct {
	api *datadogV2.LogsArchivesApi
	ctx context.Context
}

func defaultLogsArchiveAPI() (*logsArchiveAPI, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	c, ctx := client.New(cfg)
	return &logsArchiveAPI{api: datadogV2.NewLogsArchivesApi(c), ctx: ctx}, nil
}

func newLogsArchiveCmd(mkAPI func() (*logsArchiveAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Manage log archives",
	}
	cmd.AddCommand(newLogsArchiveListCmd(mkAPI))
	cmd.AddCommand(newLogsArchiveShowCmd(mkAPI))
	cmd.AddCommand(newLogsArchiveCreateCmd(mkAPI))
	cmd.AddCommand(newLogsArchiveUpdateCmd(mkAPI))
	cmd.AddCommand(newLogsArchiveDeleteCmd(mkAPI))
	return cmd
}

// archiveDestType returns destination type string from an archive definition's attributes.
func archiveDestType(attrs *datadogV2.LogsArchiveAttributes) string {
	if attrs == nil {
		return ""
	}
	dest := attrs.GetDestination()
	if dest.LogsArchiveDestinationS3 != nil {
		return "s3"
	}
	if dest.LogsArchiveDestinationGCS != nil {
		return "gcs"
	}
	if dest.LogsArchiveDestinationAzure != nil {
		return "azure"
	}
	return ""
}

// archiveDestBucket returns the bucket/container name from the destination.
func archiveDestBucket(attrs *datadogV2.LogsArchiveAttributes) string {
	if attrs == nil {
		return ""
	}
	dest := attrs.GetDestination()
	if dest.LogsArchiveDestinationS3 != nil {
		return dest.LogsArchiveDestinationS3.Bucket
	}
	if dest.LogsArchiveDestinationGCS != nil {
		return dest.LogsArchiveDestinationGCS.Bucket
	}
	if dest.LogsArchiveDestinationAzure != nil {
		return dest.LogsArchiveDestinationAzure.Container
	}
	return ""
}

func newLogsArchiveListCmd(mkAPI func() (*logsArchiveAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List log archives",
		RunE: func(cmd *cobra.Command, _ []string) error {
			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := aapi.api.ListLogsArchives(aapi.ctx)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("list log archives: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				data := resp.GetData()
				if data == nil {
					data = []datadogV2.LogsArchiveDefinition{}
				}
				return output.PrintJSON(cmd.OutOrStdout(), data)
			}

			headers := []string{"ID", "NAME", "DESTINATION"}
			var rows [][]string
			for _, def := range resp.GetData() {
				attrs := def.Attributes
				name, destType, bucket := "", "", ""
				if attrs != nil {
					name = attrs.Name
					destType = archiveDestType(attrs)
					bucket = archiveDestBucket(attrs)
				}
				dest := destType
				if bucket != "" {
					dest = destType + ":" + bucket
				}
				rows = append(rows, []string{
					def.GetId(),
					name,
					dest,
				})
			}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsArchiveShowCmd(mkAPI func() (*logsArchiveAPI, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a log archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := aapi.api.GetLogsArchive(aapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("get log archive: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), resp)
			}

			def := resp.GetData()
			attrs := def.Attributes
			name, query, destType, bucket := "", "", "", ""
			if attrs != nil {
				name = attrs.Name
				query = attrs.Query
				destType = archiveDestType(attrs)
				bucket = archiveDestBucket(attrs)
			}
			dest := destType
			if bucket != "" {
				dest = destType + ":" + bucket
			}
			headers := []string{"ID", "NAME", "QUERY", "DESTINATION"}
			rows := [][]string{{def.GetId(), name, query, dest}}
			return output.PrintTable(cmd.OutOrStdout(), headers, rows)
		},
	}
}

func newLogsArchiveCreateCmd(mkAPI func() (*logsArchiveAPI, error)) *cobra.Command {
	var (
		name           string
		query          string
		destType       string
		bucket         string
		path           string
		accountID      string
		roleName       string
		gcsClientEmail string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a log archive",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := buildArchiveDestination(destType, bucket, path, accountID, roleName, gcsClientEmail)
			if err != nil {
				return err
			}

			attrs := datadogV2.NewLogsArchiveCreateRequestAttributes(*dest, name, query)
			def := datadogV2.NewLogsArchiveCreateRequestDefinitionWithDefaults()
			def.SetAttributes(*attrs)
			body := datadogV2.NewLogsArchiveCreateRequest()
			body.SetData(*def)

			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := aapi.api.CreateLogsArchive(aapi.ctx, *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create log archive: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "archive name (required)")
	cmd.Flags().StringVar(&query, "query", "", "log filter query (required)")
	cmd.Flags().StringVar(&destType, "dest-type", "s3", "destination type: s3 or gcs")
	cmd.Flags().StringVar(&bucket, "dest-bucket", "", "destination bucket/container (required)")
	cmd.Flags().StringVar(&path, "dest-path", "", "archive path prefix")
	cmd.Flags().StringVar(&accountID, "s3-account-id", "", "AWS account ID (s3 only)")
	cmd.Flags().StringVar(&roleName, "s3-role-name", "", "IAM role name (s3 only)")
	cmd.Flags().StringVar(&gcsClientEmail, "gcs-client-email", "", "GCS service account email (gcs only)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("query")
	_ = cmd.MarkFlagRequired("dest-bucket")
	return cmd
}

func newLogsArchiveUpdateCmd(mkAPI func() (*logsArchiveAPI, error)) *cobra.Command {
	var (
		name           string
		query          string
		destType       string
		bucket         string
		path           string
		accountID      string
		roleName       string
		gcsClientEmail string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a log archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dest, err := buildArchiveDestination(destType, bucket, path, accountID, roleName, gcsClientEmail)
			if err != nil {
				return err
			}

			attrs := datadogV2.NewLogsArchiveCreateRequestAttributes(*dest, name, query)
			def := datadogV2.NewLogsArchiveCreateRequestDefinitionWithDefaults()
			def.SetAttributes(*attrs)
			body := datadogV2.NewLogsArchiveCreateRequest()
			body.SetData(*def)

			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			resp, httpResp, err := aapi.api.UpdateLogsArchive(aapi.ctx, args[0], *body)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("update log archive: %w", err)
			}

			return output.PrintJSON(cmd.OutOrStdout(), resp)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "archive name (required)")
	cmd.Flags().StringVar(&query, "query", "", "log filter query (required)")
	cmd.Flags().StringVar(&destType, "dest-type", "s3", "destination type: s3 or gcs")
	cmd.Flags().StringVar(&bucket, "dest-bucket", "", "destination bucket/container (required)")
	cmd.Flags().StringVar(&path, "dest-path", "", "archive path prefix")
	cmd.Flags().StringVar(&accountID, "s3-account-id", "", "AWS account ID (s3 only)")
	cmd.Flags().StringVar(&roleName, "s3-role-name", "", "IAM role name (s3 only)")
	cmd.Flags().StringVar(&gcsClientEmail, "gcs-client-email", "", "GCS service account email (gcs only)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("query")
	_ = cmd.MarkFlagRequired("dest-bucket")
	return cmd
}

func newLogsArchiveDeleteCmd(mkAPI func() (*logsArchiveAPI, error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a log archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of archive %q", args[0])
			}

			aapi, err := mkAPI()
			if err != nil {
				return err
			}

			httpResp, err := aapi.api.DeleteLogsArchive(aapi.ctx, args[0])
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("delete log archive: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted archive %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

// buildArchiveDestination constructs a LogsArchiveCreateRequestDestination from flags.
func buildArchiveDestination(destType, bucket, path, accountID, roleName, gcsClientEmail string) (*datadogV2.LogsArchiveCreateRequestDestination, error) {
	switch destType {
	case "s3":
		integration := datadogV2.NewLogsArchiveIntegrationS3(accountID, roleName)
		s3dest := datadogV2.NewLogsArchiveDestinationS3(bucket, *integration, datadogV2.LOGSARCHIVEDESTINATIONS3TYPE_S3)
		if path != "" {
			s3dest.SetPath(path)
		}
		d := datadogV2.LogsArchiveDestinationS3AsLogsArchiveCreateRequestDestination(s3dest)
		return &d, nil
	case "gcs":
		integration := datadogV2.NewLogsArchiveIntegrationGCS(gcsClientEmail)
		gcsdest := datadogV2.NewLogsArchiveDestinationGCS(bucket, *integration, datadogV2.LOGSARCHIVEDESTINATIONGCSTYPE_GCS)
		if path != "" {
			gcsdest.SetPath(path)
		}
		d := datadogV2.LogsArchiveDestinationGCSAsLogsArchiveCreateRequestDestination(gcsdest)
		return &d, nil
	default:
		return nil, fmt.Errorf("unsupported destination type %q: use s3 or gcs", destType)
	}
}
