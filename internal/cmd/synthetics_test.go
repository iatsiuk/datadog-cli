package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

func newTestSyntheticsAPI(srv *httptest.Server) func() (*syntheticsAPI, error) {
	return func() (*syntheticsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &syntheticsAPI{api: datadogV1.NewSyntheticsApi(c), ctx: apiCtx}, nil
	}
}

const mockSyntheticsListResponse = `{
	"tests": [
		{
			"public_id": "abc-123-def",
			"name": "Homepage API Test",
			"type": "api",
			"status": "live",
			"locations": ["aws:us-east-1", "aws:eu-west-1"]
		},
		{
			"public_id": "ghi-456-jkl",
			"name": "Login Browser Test",
			"type": "browser",
			"status": "paused",
			"locations": ["aws:us-west-2"]
		}
	]
}`

func buildSyntheticsListCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsListCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

func TestSyntheticsList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsListCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"PUBLIC_ID", "NAME", "TYPE", "STATUS", "LOCATIONS", "abc-123-def", "Homepage API Test", "api", "live", "ghi-456-jkl", "Login Browser Test", "browser", "paused"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsList_PageSizeFlag(t *testing.T) {
	t.Parallel()
	var capturedPageSize string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPageSize = r.URL.Query().Get("page_size")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSyntheticsListCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "list", "--page-size", "25"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedPageSize != "25" {
		t.Errorf("page[size] = %q, want %q", capturedPageSize, "25")
	}
}

func TestSyntheticsList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsListCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"--json", "synthetics", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-123-def", "Homepage API Test"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

const mockSyntheticsSearchResponse = `{
	"tests": [
		{
			"public_id": "mno-789-pqr",
			"name": "Checkout Flow",
			"type": "browser",
			"status": "live",
			"locations": ["aws:us-east-1"]
		}
	]
}`

func buildSyntheticsSearchCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsSearchCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

func TestSyntheticsSearch_TableOutput(t *testing.T) {
	t.Parallel()
	var capturedText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedText = r.URL.Query().Get("text")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsSearchCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "search", "--query", "checkout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedText != "checkout" {
		t.Errorf("text query = %q, want %q", capturedText, "checkout")
	}

	out := buf.String()
	for _, want := range []string{"PUBLIC_ID", "NAME", "TYPE", "STATUS", "mno-789-pqr", "Checkout Flow", "browser", "live"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsSearch_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsSearchCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"--json", "synthetics", "search", "--query", "checkout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"mno-789-pqr", "Checkout Flow"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestNewSyntheticsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewSyntheticsCommand()
	if cmd.Use != "synthetics" {
		t.Errorf("Use = %q, want %q", cmd.Use, "synthetics")
	}
}
