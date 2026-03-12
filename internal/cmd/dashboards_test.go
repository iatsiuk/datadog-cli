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
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestDashboardListsAPI(srv *httptest.Server) func() (*dashboardListsAPI, error) {
	return func() (*dashboardListsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.OperationServers["v2.DashboardListsApi.CreateDashboardListItems"] = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.OperationServers["v2.DashboardListsApi.DeleteDashboardListItems"] = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.OperationServers["v2.DashboardListsApi.GetDashboardListItems"] = datadog.ServerConfigurations{{URL: srv.URL}}
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
		return &dashboardListsAPI{
			api:   datadogV1.NewDashboardListsApi(c),
			v2api: datadogV2.NewDashboardListsApi(c),
			ctx:   ctx,
		}, nil
	}
}

const mockDashboardListsListResponse = `{
	"dashboard_lists": [
		{
			"id": 42,
			"name": "My List",
			"dashboard_count": 3,
			"created": "2024-01-15T10:00:00.000Z",
			"modified": "2024-01-20T15:30:00.000Z"
		}
	]
}`

func TestDashboardListsListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardListsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "COUNT", "CREATED", "MODIFIED", "42", "My List", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDashboardListsListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardListsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "list", "--json"})
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

func TestDashboardListsListEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"dashboard_lists":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("output missing headers:\n%s", out)
	}
}

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
	for _, want := range []string{"ID", "TITLE", "LAYOUT", "DESCRIPTION", "AUTHOR", "URL", "CREATED", "MODIFIED", "abc-123", "My Dashboard", "ordered", "user@example.com"} {
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
	wantSubs := []string{"list", "show", "create", "update", "delete", "lists"}
	got := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		got[sub.Use] = true
	}
	for _, name := range wantSubs {
		if !got[name] {
			t.Errorf("missing subcommand %q", name)
		}
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

func TestDashboardsCreateWithTemplateVarsJSON(t *testing.T) {
	t.Parallel()

	templateVarsJSON := `[{"name":"env","prefix":"env","defaults":["prod"]}]`
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
		"--template-vars-json", templateVarsJSON,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	tvars, ok := body["template_variables"].([]interface{})
	if !ok || len(tvars) != 1 {
		t.Errorf("expected 1 template variable, got %v", body["template_variables"])
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

func TestDashboardsUpdateInvalidBodyJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardsUpdateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "update", "--id", "abc-123", "--body", "{invalid"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --body JSON")
	}
	if !strings.Contains(err.Error(), "parse --body") {
		t.Errorf("error %q missing %q", err.Error(), "parse --body")
	}
}

func TestDashboardsCreateInvalidWidgetsJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardsCreateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "create", "--title", "Test", "--layout-type", "ordered", "--widgets-json", "{invalid"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --widgets-json")
	}
	if !strings.Contains(err.Error(), "parse --widgets-json") {
		t.Errorf("error %q missing %q", err.Error(), "parse --widgets-json")
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

func buildDashboardListsSubCmd(mkAPI func() (*dashboardListsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	dashboards := &cobra.Command{Use: "dashboards"}
	lists := newDashboardListsCmd(mkAPI)
	dashboards.AddCommand(lists)
	root.AddCommand(dashboards)
	return root, buf
}

const mockDashboardListShowResponse = `{
	"id": 42,
	"name": "My List",
	"dashboard_count": 3,
	"created": "2024-01-15T10:00:00.000Z",
	"modified": "2024-01-20T15:30:00.000Z"
}`

const mockDashboardListItemsResponse = `{
	"dashboards": [],
	"total": 0
}`

const mockDashboardListItemsWithDashboardsResponse = `{
	"dashboards": [
		{
			"id": "def-456",
			"title": "My Dashboard",
			"type": "custom_timeboard",
			"url": "/dashboard/def-456/my-dashboard"
		}
	],
	"total": 1
}`

func newTestDashboardListsShowServer(t *testing.T, listResp, itemsResp string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// v2 items endpoint: /api/v2/dashboard/lists/manual/{id}/dashboards
		if strings.Contains(r.URL.Path, "/v2/") {
			fmt.Fprint(w, itemsResp) //nolint:errcheck
		} else {
			fmt.Fprint(w, listResp) //nolint:errcheck
		}
	}))
}

func TestDashboardListsShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := newTestDashboardListsShowServer(t, mockDashboardListShowResponse, mockDashboardListItemsResponse)
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "show", "--id", "42"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "COUNT", "CREATED", "MODIFIED", "42", "My List", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDashboardListsShowWithItems(t *testing.T) {
	t.Parallel()

	srv := newTestDashboardListsShowServer(t, mockDashboardListShowResponse, mockDashboardListItemsWithDashboardsResponse)
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "show", "--id", "42"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"DASHBOARD_ID", "TITLE", "TYPE", "URL", "def-456", "My Dashboard", "custom_timeboard"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDashboardListsShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := newTestDashboardListsShowServer(t, mockDashboardListShowResponse, mockDashboardListItemsResponse)
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "show", "--id", "42", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result struct {
		List  map[string]interface{} `json:"list"`
		Items map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result.List["name"] != "My List" {
		t.Errorf("got name %q, want %q", result.List["name"], "My List")
	}
	if result.Items == nil {
		t.Error("expected items in JSON output, got nil")
	}
}

func TestDashboardListsShowMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
	if !strings.Contains(err.Error(), "--id is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

const mockDashboardListCreateResponse = `{
	"id": 99,
	"name": "New List"
}`

func TestDashboardListsCreateRequestBody(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardListCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "create", "--name", "New List"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if body["name"] != "New List" {
		t.Errorf("request body name = %q, want %q", body["name"], "New List")
	}

	out := buf.String()
	if !strings.Contains(out, "99") {
		t.Errorf("output missing created list id:\n%s", out)
	}
}

func TestDashboardListsCreateMissingName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "create"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --name is missing")
	}
	if !strings.Contains(err.Error(), "--name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

const mockDashboardListUpdateResponse = `{
	"id": 42,
	"name": "Updated List"
}`

func TestDashboardListsUpdateRequestBody(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDashboardListUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "update", "--id", "42", "--name", "Updated List"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "42") {
		t.Errorf("request path %q missing list id", capturedPath)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if body["name"] != "Updated List" {
		t.Errorf("request body name = %q, want %q", body["name"], "Updated List")
	}

	out := buf.String()
	if !strings.Contains(out, "Updated List") {
		t.Errorf("output missing updated name:\n%s", out)
	}
}

func TestDashboardListsUpdateMissingFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing id",
			args:    []string{"dashboards", "lists", "update", "--name", "Test"},
			wantErr: "--id is required",
		},
		{
			name:    "missing name",
			args:    []string{"dashboards", "lists", "update", "--id", "42"},
			wantErr: "--name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
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

func TestDashboardListsDeleteSuccess(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"deleted_dashboard_list_id":42}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "delete", "--id", "42", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if !strings.Contains(capturedPath, "42") {
		t.Errorf("request path %q missing list id", capturedPath)
	}
	out := buf.String()
	if !strings.Contains(out, "42") {
		t.Errorf("output missing deleted id:\n%s", out)
	}
}

func TestDashboardListsDeleteMissingYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "delete", "--id", "42"})
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
		{
			name:    "invalid layout-type",
			args:    []string{"dashboards", "create", "--title", "Test", "--layout-type", "grid"},
			wantErr: "--layout-type must be",
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

func TestDashboardListsAddItemsSuccess(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"added_dashboards_to_list":[{"id":"abc-123","type":"custom_timeboard"}]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "add-items", "--id", "42", "--dashboard", "abc-123", "--type", "custom_timeboard"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
	if !strings.Contains(capturedPath, "42") {
		t.Errorf("request path %q missing list id", capturedPath)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	dashboards, ok := body["dashboards"].([]interface{})
	if !ok || len(dashboards) != 1 {
		t.Errorf("expected 1 dashboard in body, got %v", body["dashboards"])
	}

	out := buf.String()
	if !strings.Contains(out, "added") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
}

func TestDashboardListsAddItemsMissingFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing id",
			args:    []string{"dashboards", "lists", "add-items", "--dashboard", "abc-123", "--type", "custom_timeboard"},
			wantErr: "--id is required",
		},
		{
			name:    "missing dashboard",
			args:    []string{"dashboards", "lists", "add-items", "--id", "42", "--type", "custom_timeboard"},
			wantErr: "--dashboard is required",
		},
		{
			name:    "missing type",
			args:    []string{"dashboards", "lists", "add-items", "--id", "42", "--dashboard", "abc-123"},
			wantErr: "--type is required",
		},
		{
			name:    "invalid type",
			args:    []string{"dashboards", "lists", "add-items", "--id", "42", "--dashboard", "abc-123", "--type", "unknown_type"},
			wantErr: "--type must be one of",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
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

func TestDashboardListsRemoveItemsSuccess(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"deleted_dashboards_from_list":[{"id":"abc-123","type":"custom_timeboard"}]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
	root.SetArgs([]string{"dashboards", "lists", "remove-items", "--id", "42", "--dashboard", "abc-123", "--type", "custom_timeboard"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if !strings.Contains(capturedPath, "42") {
		t.Errorf("request path %q missing list id", capturedPath)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	dashboards, ok := body["dashboards"].([]interface{})
	if !ok || len(dashboards) != 1 {
		t.Errorf("expected 1 dashboard in body, got %v", body["dashboards"])
	}

	out := buf.String()
	if !strings.Contains(out, "removed") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
}

func TestDashboardListsRemoveItemsMissingFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing id",
			args:    []string{"dashboards", "lists", "remove-items", "--dashboard", "abc-123", "--type", "custom_timeboard"},
			wantErr: "--id is required",
		},
		{
			name:    "missing dashboard",
			args:    []string{"dashboards", "lists", "remove-items", "--id", "42", "--type", "custom_timeboard"},
			wantErr: "--dashboard is required",
		},
		{
			name:    "missing type",
			args:    []string{"dashboards", "lists", "remove-items", "--id", "42", "--dashboard", "abc-123"},
			wantErr: "--type is required",
		},
		{
			name:    "invalid type",
			args:    []string{"dashboards", "lists", "remove-items", "--id", "42", "--dashboard", "abc-123", "--type", "unknown_type"},
			wantErr: "--type must be one of",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildDashboardListsSubCmd(newTestDashboardListsAPI(srv))
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

func TestDashboardsUpdateBodyMissingLayoutType(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDashboardsUpdateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "update", "--id", "abc-123", "--body", `{"title":"T","widgets":[]}`})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --body missing layout_type")
	}
	if !strings.Contains(err.Error(), "layout_type") {
		t.Errorf("error %q missing \"layout_type\"", err.Error())
	}
}

func TestDashboardsCreateTagsTrimmed(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := new(bytes.Buffer)
		b.ReadFrom(r.Body) //nolint:errcheck
		capturedBody = b.Bytes()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"abc-123","title":"T","layout_type":"ordered","url":"/dash/abc-123","widgets":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildDashboardsCreateCmd(newTestDashboardsAPI(srv))
	root.SetArgs([]string{"dashboards", "create", "--title", "T", "--layout-type", "ordered", "--widgets-json", "[]", "--tags", "team:infra, env:prod"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	tags, ok := body["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", body["tags"])
	}
	if tags[1] != "env:prod" {
		t.Errorf("tag[1] = %q, want %q (leading space not trimmed)", tags[1], "env:prod")
	}
}
