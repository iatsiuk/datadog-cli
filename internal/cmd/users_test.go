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
	"sync"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestUsersAPI(srv *httptest.Server) func() (*usersAPI, error) {
	return func() (*usersAPI, error) {
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
		return &usersAPI{api: datadogV2.NewUsersApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestUsersAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestUsersAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil usersAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV2.UsersApi")
	}
}

func TestNewUsersCommand_Use(t *testing.T) {
	t.Parallel()
	cmd := NewUsersCommand()
	if cmd.Use != "users" {
		t.Errorf("Use = %q, want %q", cmd.Use, "users")
	}
}

func TestNewUsersCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewUsersCommand()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Use] = true
	}
	if !names["list"] {
		t.Error("expected 'list' subcommand")
	}
}

const mockUsersResponse = `{
	"data": [{
		"type": "users",
		"id": "user-abc-123",
		"attributes": {
			"email": "alice@example.com",
			"name": "Alice Smith",
			"handle": "alice.smith",
			"status": "Active",
			"created_at": "2024-01-15T10:30:00.000Z"
		},
		"relationships": {
			"roles": {
				"data": [{"type": "roles", "id": "role-xyz"}]
			}
		}
	}],
	"meta": {"page": {"total_filtered_count": 1, "total_count": 1}}
}`

func buildUsersListCmd(mkAPI func() (*usersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	users.AddCommand(newUsersListCmd(mkAPI))
	root.AddCommand(users)
	return root, buf
}

func TestUsersListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUsersResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersListCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	checks := []string{"ID", "EMAIL", "NAME", "HANDLE", "STATUS", "ROLES", "CREATED_AT",
		"user-abc-123", "alice@example.com", "Alice Smith", "alice.smith", "Active", "role-xyz"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestUsersListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUsersResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersListCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"--json", "users", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user, got %d", len(result))
	}
	if got := result[0]["id"]; got != "user-abc-123" {
		t.Errorf("id = %v, want user-abc-123", got)
	}
}

func TestUsersListFilterFlag(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildUsersListCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "list", "--filter", "alice"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("filter"); got != "alice" {
		t.Errorf("filter = %q, want %q", got, "alice")
	}
}

func TestUsersListEmptyResult(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersListCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// headers should still be printed
	got := buf.String()
	if !strings.Contains(got, "ID") {
		t.Errorf("expected header row, got: %s", got)
	}
}

func TestUsersListEmptyResultJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersListCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"--json", "users", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

const mockUserShowResponse = `{
	"data": {
		"type": "users",
		"id": "user-abc-123",
		"attributes": {
			"email": "alice@example.com",
			"name": "Alice Smith",
			"handle": "alice.smith",
			"status": "Active",
			"title": "Engineer",
			"mfa_enabled": false,
			"created_at": "2024-01-15T10:30:00.000Z"
		},
		"relationships": {
			"roles": {
				"data": [{"type": "roles", "id": "role-xyz"}]
			}
		}
	}
}`

func buildUsersShowCmd(mkAPI func() (*usersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	users.AddCommand(newUsersShowCmd(mkAPI))
	root.AddCommand(users)
	return root, buf
}

func TestUsersShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersShowCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "show", "--id", "user-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	checks := []string{"user-abc-123", "alice@example.com", "Alice Smith", "alice.smith", "Active", "Engineer", "role-xyz"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestUsersShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersShowCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"--json", "users", "show", "--id", "user-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if got := result["id"]; got != "user-abc-123" {
		t.Errorf("id = %v, want user-abc-123", got)
	}
}

func TestUsersShowMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildUsersShowCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id flag is missing")
	}
}

func buildUsersCreateCmd(mkAPI func() (*usersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	users.AddCommand(newUsersCreateCmd(mkAPI))
	root.AddCommand(users)
	return root, buf
}

func TestUsersCreateCapturesRequestBody(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		reqBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body) //nolint:errcheck
			mu.Lock()
			reqBody = body
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildUsersCreateCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "create", "--email", "bob@example.com", "--name", "Bob"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := reqBody
	mu.Unlock()
	if body == nil {
		t.Fatal("no request body captured")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	data, _ := parsed["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["email"]; got != "bob@example.com" {
		t.Errorf("email = %v, want bob@example.com", got)
	}
	if got := attrs["name"]; got != "Bob" {
		t.Errorf("name = %v, want Bob", got)
	}
}

func TestUsersCreateRequiredEmailFlag(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildUsersCreateCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "create", "--name", "Bob"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --email flag is missing")
	}
}

func buildUsersInviteCmd(mkAPI func() (*usersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	users.AddCommand(newUsersInviteCmd(mkAPI))
	root.AddCommand(users)
	return root, buf
}

const mockUserInviteResponse = `{
	"data": [{
		"type": "user_invitations",
		"id": "invite-uuid-123"
	}]
}`

func TestUsersInviteCapturesRequestBody(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		reqBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body) //nolint:errcheck
			mu.Lock()
			reqBody = body
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserInviteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersInviteCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "invite", "--id", "user-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := reqBody
	mu.Unlock()
	if body == nil {
		t.Fatal("no request body captured")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	dataArr, _ := parsed["data"].([]interface{})
	if len(dataArr) != 1 {
		t.Fatalf("expected 1 invitation in data, got %d", len(dataArr))
	}

	if !strings.Contains(buf.String(), "invite-uuid-123") {
		t.Errorf("output missing invitation ID, got: %s", buf.String())
	}
}

func TestUsersInviteMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserInviteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildUsersInviteCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "invite"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id flag is missing")
	}
}

func buildUsersUpdateCmd(mkAPI func() (*usersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	users.AddCommand(newUsersUpdateCmd(mkAPI))
	root.AddCommand(users)
	return root, buf
}

func TestUsersUpdateCapturesRequestBody(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		reqBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			body, _ := io.ReadAll(r.Body) //nolint:errcheck
			mu.Lock()
			reqBody = body
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildUsersUpdateCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "update", "--id", "user-abc-123", "--name", "Alice Updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := reqBody
	mu.Unlock()
	if body == nil {
		t.Fatal("no request body captured")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	data, _ := parsed["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	if got := attrs["name"]; got != "Alice Updated" {
		t.Errorf("name = %v, want Alice Updated", got)
	}

	if !strings.Contains(buf.String(), "Updated user") {
		t.Errorf("output missing confirmation, got: %s", buf.String())
	}
}

func TestUsersUpdateOnlyChangedFields(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		reqBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			body, _ := io.ReadAll(r.Body) //nolint:errcheck
			mu.Lock()
			reqBody = body
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockUserShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildUsersUpdateCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "update", "--id", "user-abc-123", "--name", "New Name"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := reqBody
	mu.Unlock()
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	data, _ := parsed["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	// email was not passed, should not be present
	if _, ok := attrs["email"]; ok {
		t.Errorf("email should not be in request when not specified, got: %v", attrs)
	}
}

func buildUsersDisableCmd(mkAPI func() (*usersAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	users.AddCommand(newUsersDisableCmd(mkAPI))
	root.AddCommand(users)
	return root, buf
}

func TestUsersDisableSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildUsersDisableCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "disable", "--id", "user-abc-123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "Disabled user") {
		t.Errorf("output missing confirmation, got: %s", buf.String())
	}
}

func TestUsersDisableMissingYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildUsersDisableCmd(newTestUsersAPI(srv))
	root.SetArgs([]string{"users", "disable", "--id", "user-abc-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --yes flag is missing")
	}
}
