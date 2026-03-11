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

func newTestLogsArchiveAPI(srv *httptest.Server) func() (*logsArchiveAPI, error) {
	return func() (*logsArchiveAPI, error) {
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
		return &logsArchiveAPI{api: datadogV2.NewLogsArchivesApi(c), ctx: ctx}, nil
	}
}

func buildArchiveCmd(mkAPI func() (*logsArchiveAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(newLogsArchiveCmd(mkAPI))
	return root, buf
}

const mockArchiveListResponse = `{
	"data": [
		{
			"type": "archives",
			"id": "arc-001",
			"attributes": {
				"name": "web-archive",
				"query": "service:web",
				"destination": {
					"type": "s3",
					"bucket": "my-logs-bucket",
					"integration": {"account_id": "123456789", "role_name": "DatadogRole"}
				}
			}
		},
		{
			"type": "archives",
			"id": "arc-002",
			"attributes": {
				"name": "api-archive",
				"query": "service:api",
				"destination": {
					"type": "gcs",
					"bucket": "my-gcs-bucket",
					"integration": {"client_email": "sa@project.iam.gserviceaccount.com"}
				}
			}
		}
	]
}`

const mockArchiveResponse = `{
	"data": {
		"type": "archives",
		"id": "arc-001",
		"attributes": {
			"name": "web-archive",
			"query": "service:web",
			"destination": {
				"type": "s3",
				"bucket": "my-logs-bucket",
				"integration": {"account_id": "123456789", "role_name": "DatadogRole"}
			}
		}
	}
}`

func TestLogsArchiveListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "DESTINATION", "arc-001", "web-archive", "arc-002", "api-archive"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsArchiveListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "list", "--json"})
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

func TestLogsArchiveShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "show", "arc-001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "QUERY", "DESTINATION", "arc-001", "web-archive", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsArchiveShowRequiresID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestLogsArchiveCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{
		"archive", "create",
		"--name", "web-archive",
		"--query", "service:web",
		"--dest-type", "s3",
		"--dest-bucket", "my-logs-bucket",
		"--s3-account-id", "123456789",
		"--s3-role-name", "DatadogRole",
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
	if got := attrs["name"]; got != "web-archive" {
		t.Errorf("name = %v, want web-archive", got)
	}
	if got := attrs["query"]; got != "service:web" {
		t.Errorf("query = %v, want service:web", got)
	}
}

func TestLogsArchiveCreateRequiresName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "create", "--query", "service:web", "--dest-bucket", "my-bucket"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --name flag")
	}
}

func TestLogsArchiveUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockArchiveResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{
		"archive", "update", "arc-001",
		"--name", "web-archive",
		"--query", "service:web env:prod",
		"--dest-type", "s3",
		"--dest-bucket", "my-logs-bucket",
		"--s3-account-id", "123456789",
		"--s3-role-name", "DatadogRole",
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
	if got := attrs["query"]; got != "service:web env:prod" {
		t.Errorf("query = %v, want 'service:web env:prod'", got)
	}
}

func TestLogsArchiveDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "delete", "arc-001"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestLogsArchiveDeleteWithYes(t *testing.T) {
	t.Parallel()

	var deletedID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		deletedID = parts[len(parts)-1]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildArchiveCmd(newTestLogsArchiveAPI(srv))
	root.SetArgs([]string{"archive", "delete", "arc-001", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if deletedID != "arc-001" {
		t.Errorf("deleted archive id = %q, want %q", deletedID, "arc-001")
	}
	if !strings.Contains(buf.String(), "arc-001") {
		t.Errorf("output should mention deleted archive id:\n%s", buf.String())
	}
}
