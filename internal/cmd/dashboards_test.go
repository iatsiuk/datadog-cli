package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

const mockDashboardShowResponse = `{
	"id": "abc-123",
	"title": "My Dashboard",
	"layout_type": "ordered",
	"url": "/dashboard/abc-123/my-dashboard",
	"created_at": "2024-01-15T10:00:00.000Z",
	"modified_at": "2024-01-20T15:30:00.000Z",
	"author_handle": "user@example.com",
	"widgets": []
}`

const mockDashboardsListResponse = `{
	"dashboards": [
		{
			"id": "abc-123",
			"title": "My Dashboard",
			"layout_type": "ordered",
			"url": "/dashboard/abc-123/my-dashboard",
			"created_at": "2024-01-15T10:00:00.000Z",
			"modified_at": "2024-01-20T15:30:00.000Z"
		}
	]
}`

func newTestDashboardsAPI(srv *httptest.Server) func() (*dashboardsAPI, error) {
	return func() (*dashboardsAPI, error) {
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
		return &dashboardsAPI{api: datadogV1.NewDashboardsApi(c), ctx: ctx}, nil
	}
}

func buildDashboardsListCmd(mkAPI func() (*dashboardsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	dashboards.AddCommand(newDashboardsListCmd(mkAPI))
	root.AddCommand(dashboards)
	return root, buf
}

func buildDashboardsShowCmd(mkAPI func() (*dashboardsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	dashboards.AddCommand(newDashboardsShowCmd(mkAPI))
	root.AddCommand(dashboards)
	return root, buf
}

func TestDashboardsShowTableOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsShowCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "show", "--id", "abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "abc-123") {
		t.Errorf("request path %q missing dashboard id", capturedPath)
	}
	out := buf.String()
	for _, want := range []string{"ID", "TITLE", "LAYOUT", "URL", "CREATED", "MODIFIED", "abc-123", "My Dashboard", "ordered"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDashboardsShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsShowCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "show", "--id", "abc-123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["id"] != "abc-123" {
		t.Errorf("got id %q, want %q", result["id"], "abc-123")
	}
}

func TestDashboardsShowMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardsShowCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
	if !strings.Contains(err.Error(), "--id is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewDashboardsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewDashboardsCommand()
	if cmd.Use != "dashboards" {
		t.Errorf("Use = %q, want %q", cmd.Use, "dashboards")
	}
}

func TestNewTestDashboardsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestDashboardsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil api")
	}
	if api.api == nil {
		t.Fatal("expected non-nil api.api")
	}
}

func TestDashboardsListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsListCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "TITLE", "LAYOUT", "URL", "CREATED", "MODIFIED", "abc-123", "My Dashboard", "ordered", "/dashboard/abc-123"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDashboardsListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsListCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestDashboardsListEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"dashboards":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsListCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	// table headers should still appear
	if !strings.Contains(out, "ID") {
		t.Errorf("output missing headers:\n%s", out)
	}
}

func buildDashboardsCreateCmd(mkAPI func() (*dashboardsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	dashboards.AddCommand(newDashboardsCreateCmd(mkAPI))
	root.AddCommand(dashboards)
	return root, buf
}

const mockDashboardCreateResponse = `{
	"id": "new-456",
	"title": "New Dashboard",
	"layout_type": "ordered",
	"url": "/dashboard/new-456/new-dashboard",
	"widgets": []
}`

func TestDashboardsCreateRequestBody(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = buf.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsCreateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "create", "--title", "New Dashboard", "--layout-type", "ordered"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	if body["title"] != "New Dashboard" {
		t.Errorf("request body title = %q, want %q", body["title"], "New Dashboard")
	}
	if body["layout_type"] != "ordered" {
		t.Errorf("request body layout_type = %q, want %q", body["layout_type"], "ordered")
	}

	out := buf.String()
	if !strings.Contains(out, "new-456") {
		t.Errorf("output missing created dashboard id:\n%s", out)
	}
}

func TestDashboardsCreateWithOptionalFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildDashboardsCreateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{
		"dashboards", "create",
		"--title", "New Dashboard",
		"--layout-type", "ordered",
		"--description", "My desc",
		"--tags", "team:infra,env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if body["description"] != "My desc" {
		t.Errorf("description = %q, want %q", body["description"], "My desc")
	}
	tags, ok := body["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("expected 2 tags, got %v", body["tags"])
	}
}

func TestDashboardsCreateWithWidgetsJSON(t *testing.T) {
	t.Parallel()

	widgetsJSON := `[{"definition":{"type":"note","content":"hello"}}]`
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildDashboardsCreateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{
		"dashboards", "create",
		"--title", "New Dashboard",
		"--layout-type", "ordered",
		"--widgets-json", widgetsJSON,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	widgets, ok := body["widgets"].([]interface{})
	if !ok || len(widgets) != 1 {
		t.Errorf("expected 1 widget, got %v", body["widgets"])
	}
}

func buildDashboardsUpdateCmd(mkAPI func() (*dashboardsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	dashboards.AddCommand(newDashboardsUpdateCmd(mkAPI))
	root.AddCommand(dashboards)
	return root, buf
}

const mockDashboardUpdateResponse = `{
	"id": "abc-123",
	"title": "Updated Dashboard",
	"layout_type": "ordered",
	"url": "/dashboard/abc-123/updated-dashboard",
	"widgets": []
}`

func TestDashboardsUpdateRequestBody(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsUpdateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "update", "--id", "abc-123", "--title", "Updated Dashboard", "--layout-type", "ordered"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "abc-123") {
		t.Errorf("request path %q missing dashboard id", capturedPath)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	if body["title"] != "Updated Dashboard" {
		t.Errorf("request body title = %q, want %q", body["title"], "Updated Dashboard")
	}
	if body["layout_type"] != "ordered" {
		t.Errorf("request body layout_type = %q, want %q", body["layout_type"], "ordered")
	}

	out := buf.String()
	if !strings.Contains(out, "abc-123") {
		t.Errorf("output missing dashboard id:\n%s", out)
	}
}

func TestDashboardsUpdateFullReplace(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	bodyJSON := `{"title":"Updated Dashboard","layout_type":"ordered","widgets":[]}`
	root, _ := buildDashboardsUpdateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "update", "--id", "abc-123", "--body", bodyJSON})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if body["title"] != "Updated Dashboard" {
		t.Errorf("title = %q, want %q", body["title"], "Updated Dashboard")
	}
}

func TestDashboardsUpdateMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardsUpdateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "update", "--title", "Test", "--layout-type", "ordered"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
	if !strings.Contains(err.Error(), "--id is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func buildDashboardsDeleteCmd(mkAPI func() (*dashboardsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	dashboards.AddCommand(newDashboardsDeleteCmd(mkAPI))
	root.AddCommand(dashboards)
	return root, buf
}

func TestDashboardsDeleteSuccess(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"deleted_dashboard_id":"abc-123"}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardsDeleteCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "delete", "--id", "abc-123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if !strings.Contains(capturedPath, "abc-123") {
		t.Errorf("request path %q missing dashboard id", capturedPath)
	}
	out := buf.String()
	if !strings.Contains(out, "abc-123") {
		t.Errorf("output missing deleted id:\n%s", out)
	}
}

func TestDashboardsDeleteMissingYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardsDeleteCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "delete", "--id", "abc-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --yes is missing")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDashboardsCreateMissingRequiredFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing title",
			args:    []string{"dashboards", "create", "--layout-type", "ordered"},
			wantErr: "--title is required",
		},
		{
			name:    "missing layout-type",
			args:    []string{"dashboards", "create", "--title", "Test"},
			wantErr: "--layout-type is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildDashboardsCreateCmd(newTestDashboardsAPI(srv))
			root.SetArgs(tc.args)
			err := root.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q missing %q", err.Error(), tc.wantErr)
			}
		})
	}
}
