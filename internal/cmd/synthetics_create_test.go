package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const mockSyntheticsCreateAPIResponse = `{
	"public_id": "new-api-001",
	"name": "My API Test",
	"type": "api",
	"subtype": "http",
	"status": "live",
	"locations": ["aws:us-east-1", "aws:eu-west-1"],
	"message": "",
	"tags": ["env:prod"],
	"config": {"request": {"method": "GET", "url": "https://example.com"}},
	"options": {"tick_every": 60}
}`

const mockSyntheticsCreateBrowserResponse = `{
	"public_id": "new-brw-001",
	"name": "My Browser Test",
	"type": "browser",
	"status": "live",
	"locations": ["aws:us-east-1"],
	"message": "",
	"tags": [],
	"config": {"assertions": [], "request": {"url": "https://example.com"}},
	"options": {"tick_every": 3600}
}`

func buildSyntheticsCreateCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsCreateCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

func TestSyntheticsCreateAPI_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsCreateAPIResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "api",
		"--name", "My API Test",
		"--url", "https://example.com",
		"--locations", "aws:us-east-1,aws:eu-west-1",
		"--type", "http",
		"--frequency", "60",
		"--tags", "env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"new-api-001", "My API Test", "api", "live"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsCreateAPI_SendsCorrectPayload(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsCreateAPIResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "api",
		"--name", "My API Test",
		"--url", "https://example.com",
		"--locations", "aws:us-east-1",
		"--type", "http",
		"--frequency", "60",
		"--status", "live",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, capturedBody)
	}

	if body["name"] != "My API Test" {
		t.Errorf("name = %v, want %q", body["name"], "My API Test")
	}

	locs, ok := body["locations"].([]interface{})
	if !ok || len(locs) == 0 {
		t.Errorf("locations missing or empty in payload")
	}
}

func TestSyntheticsCreateAPI_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsCreateAPIResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"--json",
		"synthetics", "create", "api",
		"--name", "My API Test",
		"--url", "https://example.com",
		"--locations", "aws:us-east-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"new-api-001", "My API Test"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsCreateAPI_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "api",
		"--url", "https://example.com",
		"--locations", "aws:us-east-1",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --name, got nil")
	}
}

func TestSyntheticsCreateAPI_MissingLocations(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "api",
		"--name", "My API Test",
		"--url", "https://example.com",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --locations, got nil")
	}
}

func TestSyntheticsCreateBrowser_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsCreateBrowserResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "browser",
		"--name", "My Browser Test",
		"--url", "https://example.com",
		"--locations", "aws:us-east-1",
		"--frequency", "3600",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"new-brw-001", "My Browser Test", "browser", "live"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsCreateBrowser_SendsCorrectPayload(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsCreateBrowserResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "browser",
		"--name", "My Browser Test",
		"--url", "https://example.com",
		"--locations", "aws:us-east-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, capturedBody)
	}

	if body["name"] != "My Browser Test" {
		t.Errorf("name = %v, want %q", body["name"], "My Browser Test")
	}

	cfg, ok := body["config"].(map[string]interface{})
	if !ok {
		t.Fatal("config missing in payload")
	}
	req, ok := cfg["request"].(map[string]interface{})
	if !ok {
		t.Fatal("config.request missing in payload")
	}
	if req["url"] != "https://example.com" {
		t.Errorf("config.request.url = %v, want %q", req["url"], "https://example.com")
	}
}

func TestSyntheticsCreateBrowser_MissingURL(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "browser",
		"--name", "My Browser Test",
		"--locations", "aws:us-east-1",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --url, got nil")
	}
}

func TestSyntheticsCreateBrowser_MissingLocations(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsCreateCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "create", "browser",
		"--name", "My Browser Test",
		"--url", "https://example.com",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --locations, got nil")
	}
}
