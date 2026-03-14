package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func buildSyntheticsLocationCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsLocationCmd(mkAPI))
	syn.AddCommand(newSyntheticsPrivateLocationCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

const mockSyntheticsLocationsResponse = `{
	"locations": [
		{"id": "aws:us-east-1", "name": "AWS US East (N. Virginia)"},
		{"id": "aws:eu-west-1", "name": "AWS EU (Ireland)"},
		{"id": "pl:my-private-loc-abc123", "name": "My Private Location"}
	]
}`

const mockSyntheticsDefaultLocationsResponse = `["aws:us-east-1", "aws:eu-west-1"]`

const mockSyntheticsPrivateLocationResponse = `{
	"id": "pl:my-private-loc-abc123",
	"name": "My Private Location",
	"description": "Internal network location",
	"tags": ["env:prod", "region:us"]
}`

const mockSyntheticsPrivateLocationCreationResponse = `{
	"private_location": {
		"id": "pl:my-private-loc-abc123",
		"name": "My Private Location",
		"description": "Internal network location",
		"tags": ["env:prod"]
	},
	"result_encryption": {},
	"config": {}
}`

func TestSyntheticsLocationList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/synthetics/locations" && r.Method == http.MethodGet {
			fmt.Fprint(w, mockSyntheticsLocationsResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "location", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "REGION", "PRIVATE", "aws:us-east-1", "AWS US East", "us-east-1", "pl:my-private-loc-abc123", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsLocationList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsLocationsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"--json", "synthetics", "location", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"aws:us-east-1", "AWS US East", "pl:my-private-loc-abc123"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsLocationDefaults_Output(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/synthetics/settings/default_locations" && r.Method == http.MethodGet {
			fmt.Fprint(w, mockSyntheticsDefaultLocationsResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "location", "defaults"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"aws:us-east-1", "aws:eu-west-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsPrivateLocationShow_Output(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/pl:my-private-loc-abc123") && r.Method == http.MethodGet {
			fmt.Fprint(w, mockSyntheticsPrivateLocationResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "private-location", "show", "pl:my-private-loc-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"pl:my-private-loc-abc123", "My Private Location", "Internal network location", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsPrivateLocationShow_MissingArg(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "private-location", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestSyntheticsPrivateLocationCreate_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			fmt.Fprint(w, mockSyntheticsPrivateLocationCreationResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "private-location", "create",
		"--name", "My Private Location",
		"--description", "Internal network location",
		"--tags", "env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "My Private Location") {
		t.Errorf("output missing location name\nfull output:\n%s", out)
	}
}

func TestSyntheticsPrivateLocationCreate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "private-location", "create", "--description", "test"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --name, got nil")
	}
	if !strings.Contains(err.Error(), "--name") {
		t.Errorf("error should mention --name, got: %v", err)
	}
}

func TestSyntheticsPrivateLocationDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "private-location", "delete", "pl:my-private-loc-abc123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted") {
		t.Errorf("output missing 'deleted', got: %s", out)
	}
}

func TestSyntheticsPrivateLocationDelete_RequiresYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsLocationCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "private-location", "delete", "pl:my-private-loc-abc123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error without --yes flag, got nil")
	}
}
