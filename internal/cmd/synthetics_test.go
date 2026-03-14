package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

const mockSyntheticsAPITestResponse = `{
	"public_id": "abc-123-def",
	"name": "Homepage API Test",
	"type": "api",
	"status": "live",
	"locations": ["aws:us-east-1"],
	"message": "alert on failure",
	"tags": ["env:prod", "team:backend"],
	"config": {"request": {"method": "GET", "url": "https://example.com"}},
	"options": {}
}`

const mockSyntheticsBrowserTestResponse = `{
	"public_id": "ghi-456-jkl",
	"name": "Login Browser Test",
	"type": "browser",
	"status": "paused",
	"locations": ["aws:us-west-2"],
	"message": "browser test failed",
	"tags": ["env:staging"],
	"config": {"assertions": [], "request": {"url": "https://example.com"}},
	"options": {}
}`

func buildSyntheticsShowCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsShowCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

func TestSyntheticsShow_APITest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// GetTest: /api/v1/synthetics/tests/abc-123-def
		// GetAPITest: /api/v1/synthetics/tests/api/abc-123-def
		if strings.HasPrefix(r.URL.Path, "/api/v1/synthetics/tests/api/") {
			fmt.Fprint(w, mockSyntheticsAPITestResponse) //nolint:errcheck
		} else {
			fmt.Fprint(w, `{"public_id":"abc-123-def","name":"Homepage API Test","type":"api","status":"live","locations":["aws:us-east-1"]}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildSyntheticsShowCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "show", "abc-123-def"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-123-def", "Homepage API Test", "api", "live"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsShow_BrowserTest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// GetTest: /api/v1/synthetics/tests/ghi-456-jkl
		// GetBrowserTest: /api/v1/synthetics/tests/browser/ghi-456-jkl
		if strings.HasPrefix(r.URL.Path, "/api/v1/synthetics/tests/browser/") {
			fmt.Fprint(w, mockSyntheticsBrowserTestResponse) //nolint:errcheck
		} else {
			fmt.Fprint(w, `{"public_id":"ghi-456-jkl","name":"Login Browser Test","type":"browser","status":"paused","locations":["aws:us-west-2"]}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildSyntheticsShowCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "show", "ghi-456-jkl"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ghi-456-jkl", "Login Browser Test", "browser", "paused"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsShow_MissingPublicID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsShowCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing public-id, got nil")
	}
}

const mockSyntheticsDeleteResponse = `{"deleted_tests": [{"public_id": "abc-123-def", "deleted_at": "2026-01-01T00:00:00Z"}, {"public_id": "ghi-456-jkl", "deleted_at": "2026-01-01T00:00:00Z"}]}`

func buildSyntheticsDeleteCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsDeleteCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

func TestSyntheticsDelete_Success(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsDeleteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsDeleteCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "delete", "--id", "abc-123-def,ghi-456-jkl", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(string(capturedBody), "abc-123-def") {
		t.Errorf("request body missing public_id, got: %s", capturedBody)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted") {
		t.Errorf("output missing 'deleted', got: %s", out)
	}
}

func TestSyntheticsDelete_RequiresYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsDeleteCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "delete", "--id", "abc-123-def"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error without --yes flag, got nil")
	}
}

func TestSyntheticsDelete_RequiresID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsDeleteCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "delete", "--yes"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error without --id flag, got nil")
	}
}
