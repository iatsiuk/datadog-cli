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

func buildSyntheticsVariableCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsVariableCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

const mockSyntheticsVariableListResponse = `{
	"variables": [
		{
			"id": "var-001",
			"name": "API_KEY",
			"description": "API key for external service",
			"tags": ["env:prod", "team:backend"],
			"value": {"secure": false, "value": "secret123"}
		},
		{
			"id": "var-002",
			"name": "AUTH_TOKEN",
			"description": "Authentication token",
			"tags": ["env:staging"],
			"value": {"secure": true}
		}
	]
}`

const mockSyntheticsVariableResponse = `{
	"id": "var-001",
	"name": "API_KEY",
	"description": "API key for external service",
	"tags": ["env:prod", "team:backend"],
	"value": {"secure": false, "value": "secret123"}
}`

func TestSyntheticsVariableList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/synthetics/variables" && r.Method == http.MethodGet {
			fmt.Fprint(w, mockSyntheticsVariableListResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "variable", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "DESCRIPTION", "SECURE", "TAGS", "var-001", "API_KEY", "var-002", "AUTH_TOKEN"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsVariableList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsVariableListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"--json", "synthetics", "variable", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"var-001", "API_KEY", "var-002", "AUTH_TOKEN"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsVariableShow_Output(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/var-001") {
			fmt.Fprint(w, mockSyntheticsVariableResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "variable", "show", "var-001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"var-001", "API_KEY", "API key for external service", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsVariableShow_MissingArg(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "variable", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing arg, got nil")
	}
}

func TestSyntheticsVariableCreate_Success(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsVariableResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "variable", "create",
		"--name", "API_KEY",
		"--value", "secret123",
		"--description", "API key for external service",
		"--tags", "env:prod,team:backend",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if body["name"] != "API_KEY" {
		t.Errorf("name = %v, want API_KEY", body["name"])
	}

	out := buf.String()
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("output missing API_KEY\nfull output:\n%s", out)
	}
}

func TestSyntheticsVariableCreate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "variable", "create", "--value", "v"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --name, got nil")
	}
	if !strings.Contains(err.Error(), "--name") {
		t.Errorf("error should mention --name, got: %v", err)
	}
}

func TestSyntheticsVariableUpdate_Success(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSyntheticsVariableResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "variable", "update", "var-001",
		"--name", "API_KEY",
		"--value", "newsecret",
		"--description", "updated desc",
		"--tags", "env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("output missing API_KEY\nfull output:\n%s", out)
	}
}

func TestSyntheticsVariableDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "variable", "delete", "var-001", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted") {
		t.Errorf("output missing 'deleted', got: %s", out)
	}
}

func TestSyntheticsVariableDelete_RequiresYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsVariableCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "variable", "delete", "var-001"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error without --yes flag, got nil")
	}
}
