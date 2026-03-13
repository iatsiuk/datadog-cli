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

func newTestRolesAPI(srv *httptest.Server) func() (*rolesAPI, error) {
	return func() (*rolesAPI, error) {
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
		return &rolesAPI{api: datadogV2.NewRolesApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestRolesAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestRolesAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil rolesAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV2.RolesApi")
	}
}

const mockRolesListResponse = `{
	"data": [{
		"type": "roles",
		"id": "role-abc-123",
		"attributes": {
			"name": "Admin Role",
			"user_count": 5,
			"created_at": "2024-01-15T10:30:00.000Z"
		}
	}],
	"meta": {"page": {"total_filtered_count": 1, "total_count": 1}}
}`

func buildRolesListCmd(mkAPI func() (*rolesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	roles := &cobra.Command{Use: "roles"}
	roles.AddCommand(newRolesListCmd(mkAPI))
	users.AddCommand(roles)
	root.AddCommand(users)
	return root, buf
}

func TestRolesListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRolesListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRolesListCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"users", "roles", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	checks := []string{"ID", "NAME", "USER_COUNT", "CREATED_AT",
		"role-abc-123", "Admin Role", "5"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestRolesListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRolesListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRolesListCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"--json", "users", "roles", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 role, got %d", len(result))
	}
	if got := result[0]["id"]; got != "role-abc-123" {
		t.Errorf("id = %v, want role-abc-123", got)
	}
}

func TestRolesListEmptyResult(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRolesListCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"users", "roles", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ID") {
		t.Errorf("expected header row, got: %s", got)
	}
}

const mockRoleShowResponse = `{
	"data": {
		"type": "roles",
		"id": "role-abc-123",
		"attributes": {
			"name": "Admin Role",
			"user_count": 5,
			"created_at": "2024-01-15T10:30:00.000Z"
		},
		"relationships": {
			"permissions": {
				"data": [
					{"type": "permissions", "id": "perm-001"},
					{"type": "permissions", "id": "perm-002"}
				]
			}
		}
	}
}`

func buildRolesShowCmd(mkAPI func() (*rolesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	roles := &cobra.Command{Use: "roles"}
	roles.AddCommand(newRolesShowCmd(mkAPI))
	users.AddCommand(roles)
	root.AddCommand(users)
	return root, buf
}

func TestRolesShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRoleShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRolesShowCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"users", "roles", "show", "--id", "role-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	checks := []string{"role-abc-123", "Admin Role", "5", "perm-001", "perm-002"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestRolesShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRoleShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRolesShowCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"--json", "users", "roles", "show", "--id", "role-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if got := result["id"]; got != "role-abc-123" {
		t.Errorf("id = %v, want role-abc-123", got)
	}
}

func TestRolesShowMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildRolesShowCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"users", "roles", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

const mockRoleCreateResponse = `{
	"data": {
		"type": "roles",
		"id": "new-role-id",
		"attributes": {
			"name": "New Role",
			"user_count": 0,
			"created_at": "2024-06-01T00:00:00.000Z"
		}
	}
}`

func buildRolesCreateCmd(mkAPI func() (*rolesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	roles := &cobra.Command{Use: "roles"}
	roles.AddCommand(newRolesCreateCmd(mkAPI))
	users.AddCommand(roles)
	root.AddCommand(users)
	return root, buf
}

func TestRolesCreateTableOutput(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRoleCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRolesCreateCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"users", "roles", "create", "--name", "New Role"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatalf("request body unmarshal: %v", err)
	}
	data, _ := reqBody["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "New Role" {
		t.Errorf("request name = %v, want New Role", got)
	}

	got := buf.String()
	if !strings.Contains(got, "new-role-id") || !strings.Contains(got, "New Role") {
		t.Errorf("output missing created role info\ngot: %s", got)
	}
}

func TestRolesCreateMissingName(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildRolesCreateCmd(newTestRolesAPI(srv))
	root.SetArgs([]string{"users", "roles", "create"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --name flag")
	}
}
