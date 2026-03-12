package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestLogsCustomDestAPI(srv *httptest.Server) func() (*logsCustomDestAPI, error) {
	return func() (*logsCustomDestAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		ctx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &logsCustomDestAPI{api: datadogV2.NewLogsCustomDestinationsApi(c), ctx: ctx}, nil
	}
}

func buildCustomDestCmd(mkAPI func() (*logsCustomDestAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(newLogsCustomDestCmd(mkAPI))
	return root, buf
}

const mockCustomDestListResponse = `{
	"data": [
		{
			"type": "custom_destination",
			"id": "dest-abc123",
			"attributes": {
				"name": "My HTTP Destination",
				"query": "service:web",
				"enabled": true
			}
		},
		{
			"type": "custom_destination",
			"id": "dest-def456",
			"attributes": {
				"name": "Splunk Dest",
				"query": "",
				"enabled": false
			}
		}
	]
}`

const mockCustomDestResponse = `{
	"data": {
		"type": "custom_destination",
		"id": "dest-abc123",
		"attributes": {
			"name": "My HTTP Destination",
			"query": "service:web",
			"enabled": true
		}
	}
}`

func TestLogsCustomDestListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "QUERY", "ENABLED", "dest-abc123", "My HTTP Destination", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsCustomDestListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 2 {
		t.Errorf("got %d entries, want 2", len(result))
	}
}

func TestLogsCustomDestShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "show", "dest-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "dest-abc123", "My HTTP Destination", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsCustomDestShowRequiresID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestLogsCustomDestCreateFlags(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestResponse) //nolint:errcheck
	}))
	defer srv.Close()

	t.Setenv("DD_LOGS_DEST_PASSWORD", "pass")
	root, _ := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{
		"custom-destination", "create",
		"--name", "My HTTP Destination",
		"--url", "https://example.com/logs",
		"--username", "user",
		"--query", "service:web",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data in request")
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("missing attributes in request")
	}
	if got := attrs["name"]; got != "My HTTP Destination" {
		t.Errorf("name = %v, want 'My HTTP Destination'", got)
	}
	if got := attrs["query"]; got != "service:web" {
		t.Errorf("query = %v, want 'service:web'", got)
	}
}

func TestLogsCustomDestCreateRequiredFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "create", "--name", "test"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestLogsCustomDestUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockCustomDestResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{
		"custom-destination", "update", "dest-abc123",
		"--name", "Updated Name",
		"--query", "service:api",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data in request")
	}
	if got := data["id"]; got != "dest-abc123" {
		t.Errorf("id = %v, want dest-abc123", got)
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("missing attributes in request")
	}
	if got := attrs["name"]; got != "Updated Name" {
		t.Errorf("name = %v, want 'Updated Name'", got)
	}
}

func TestLogsCustomDestDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "delete", "dest-abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestLogsCustomDestDeleteWithYes(t *testing.T) {
	t.Parallel()

	var deletedID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		deletedID = parts[len(parts)-1]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildCustomDestCmd(newTestLogsCustomDestAPI(srv))
	root.SetArgs([]string{"custom-destination", "delete", "dest-abc123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if deletedID != "dest-abc123" {
		t.Errorf("deleted id = %q, want %q", deletedID, "dest-abc123")
	}
	if !strings.Contains(buf.String(), "dest-abc123") {
		t.Errorf("output should mention deleted id:\n%s", buf.String())
	}
}
