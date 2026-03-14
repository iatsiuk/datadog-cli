package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"

	"github.com/iatsiuk/datadog-cli/internal/output"
)

var errSyntheticsNameRequired = errors.New("--name is required")
var errSyntheticsURLRequired = errors.New("--url is required")
var errSyntheticsLocationsRequired = errors.New("--locations is required")

func newSyntheticsCreateCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Synthetic test",
	}
	cmd.AddCommand(newSyntheticsCreateAPICmd(mkAPI))
	cmd.AddCommand(newSyntheticsCreateBrowserCmd(mkAPI))
	return cmd
}

func newSyntheticsCreateAPICmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		name      string
		subtype   string
		url       string
		locations string
		frequency int64
		status    string
		tags      string
	)

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Create a Synthetic API test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errSyntheticsNameRequired
			}
			if locations == "" {
				return errSyntheticsLocationsRequired
			}

			locs := strings.Split(locations, ",")
			for i, l := range locs {
				locs[i] = strings.TrimSpace(l)
			}

			req := datadogV1.NewSyntheticsTestRequest()
			if url != "" {
				req.SetUrl(url)
			}
			if subtype == "http" || subtype == "" {
				method := "GET"
				req.SetMethod(method)
			}

			cfg := datadogV1.NewSyntheticsAPITestConfig()
			cfg.SetRequest(*req)

			opts := datadogV1.NewSyntheticsTestOptions()
			if frequency > 0 {
				opts.SetTickEvery(frequency)
			}

			test := datadogV1.NewSyntheticsAPITest(
				*cfg,
				locs,
				"",
				name,
				*opts,
				datadogV1.SYNTHETICSAPITESTTYPE_API,
			)

			if subtype != "" {
				st, err := datadogV1.NewSyntheticsTestDetailsSubTypeFromValue(subtype)
				if err != nil {
					return fmt.Errorf("invalid --type value %q: %w", subtype, err)
				}
				test.SetSubtype(*st)
			}

			if status != "" {
				ps, err := datadogV1.NewSyntheticsTestPauseStatusFromValue(status)
				if err != nil {
					return fmt.Errorf("invalid --status value %q: %w", status, err)
				}
				test.SetStatus(*ps)
			}

			if tags != "" {
				tagList := strings.Split(tags, ",")
				for i, tg := range tagList {
					tagList[i] = strings.TrimSpace(tg)
				}
				test.SetTags(tagList)
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			created, httpResp, err := sapi.api.CreateSyntheticsAPITest(sapi.ctx, *test)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create synthetics api test: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), created)
			}
			return synthShowAPITest(cmd, created)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "test name (required)")
	cmd.Flags().StringVar(&subtype, "type", "http", "API test subtype (http|ssl|dns|tcp|icmp|grpc|websocket)")
	cmd.Flags().StringVar(&url, "url", "", "URL to test")
	cmd.Flags().StringVar(&locations, "locations", "", "comma-separated list of locations (required)")
	cmd.Flags().Int64Var(&frequency, "frequency", 60, "test frequency in seconds (tick_every)")
	cmd.Flags().StringVar(&status, "status", "live", "test status (live|paused)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}

func newSyntheticsCreateBrowserCmd(mkAPI func() (*syntheticsAPI, error)) *cobra.Command {
	var (
		name      string
		url       string
		locations string
		frequency int64
		tags      string
	)

	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Create a Synthetic browser test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return errSyntheticsNameRequired
			}
			if url == "" {
				return errSyntheticsURLRequired
			}
			if locations == "" {
				return errSyntheticsLocationsRequired
			}

			locs := strings.Split(locations, ",")
			for i, l := range locs {
				locs[i] = strings.TrimSpace(l)
			}

			req := datadogV1.NewSyntheticsTestRequest()
			req.SetUrl(url)

			cfg := datadogV1.NewSyntheticsBrowserTestConfig(
				[]datadogV1.SyntheticsAssertion{},
				*req,
			)

			opts := datadogV1.NewSyntheticsTestOptions()
			if frequency > 0 {
				opts.SetTickEvery(frequency)
			}

			test := datadogV1.NewSyntheticsBrowserTest(
				*cfg,
				locs,
				"",
				name,
				*opts,
				datadogV1.SYNTHETICSBROWSERTESTTYPE_BROWSER,
			)

			if tags != "" {
				tagList := strings.Split(tags, ",")
				for i, tg := range tagList {
					tagList[i] = strings.TrimSpace(tg)
				}
				test.SetTags(tagList)
			}

			sapi, err := mkAPI()
			if err != nil {
				return err
			}

			created, httpResp, err := sapi.api.CreateSyntheticsBrowserTest(sapi.ctx, *test)
			if httpResp != nil {
				_ = httpResp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("create synthetics browser test: %w", err)
			}

			asJSON := false
			if f := cmd.Root().PersistentFlags().Lookup("json"); f != nil {
				asJSON = f.Value.String() == "true"
			}
			if asJSON {
				return output.PrintJSON(cmd.OutOrStdout(), created)
			}
			return synthShowBrowserTest(cmd, created)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "test name (required)")
	cmd.Flags().StringVar(&url, "url", "", "URL to test (required)")
	cmd.Flags().StringVar(&locations, "locations", "", "comma-separated list of locations (required)")
	cmd.Flags().Int64Var(&frequency, "frequency", 3600, "test frequency in seconds (tick_every)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}
