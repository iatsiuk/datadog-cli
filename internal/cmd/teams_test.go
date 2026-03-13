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
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestTeamsAPI(srv *httptest.Server) func() (*teamsAPI, error) {
	return func() (*teamsAPI, error) {
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
		return &teamsAPI{api: datadogV2.NewTeamsApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestTeamsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestTeamsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil teamsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV2.TeamsApi")
	}
}

const mockTeamsListResponse = `{
	"data": [{
		"type": "team",
		"id": "team-abc-123",
		"attributes": {
			"name": "Platform Team",
			"handle": "platform-team",
			"user_count": 8,
			"description": "Core platform engineers"
		}
	}],
	"meta": {"total_count": 1}
}`

const mockTeamSingleResponse = `{
	"data": {
		"type": "team",
		"id": "team-abc-123",
		"attributes": {
			"name": "Platform Team",
			"handle": "platform-team",
			"user_count": 8,
			"description": "Core platform engineers"
		}
	}
}`

func buildTeamsListCmd(mkAPI func() (*teamsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	teams := &cobra.Command{Use: "teams"}
	teams.AddCommand(newTeamsListCmd(mkAPI))
	users.AddCommand(teams)
	root.AddCommand(users)
	return root, buf
}

func buildTeamsCRUDCmd(mkAPI func() (*teamsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	teams := &cobra.Command{Use: "teams"}
	teams.AddCommand(newTeamsShowCmd(mkAPI))
	teams.AddCommand(newTeamsCreateCmd(mkAPI))
	teams.AddCommand(newTeamsUpdateCmd(mkAPI))
	teams.AddCommand(newTeamsDeleteCmd(mkAPI))
	users.AddCommand(teams)
	root.AddCommand(users)
	return root, buf
}

func TestTeamsListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsListCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	checks := []string{"ID", "NAME", "HANDLE", "USER_COUNT", "DESCRIPTION",
		"team-abc-123", "Platform Team", "platform-team", "8", "Core platform engineers"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestTeamsListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsListCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"--json", "users", "teams", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 team, got %d", len(result))
	}
	if got := result[0]["id"]; got != "team-abc-123" {
		t.Errorf("id = %v, want team-abc-123", got)
	}
}

func TestTeamsListWithFilter(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("filter[keyword]")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildTeamsListCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "list", "--filter", "platform"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedQuery != "platform" {
		t.Errorf("filter[keyword] = %q, want %q", capturedQuery, "platform")
	}
}

func TestTeamsListEmptyResult(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[],"meta":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsListCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ID") {
		t.Errorf("expected header row, got: %s", got)
	}
}

func TestTeamsShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "show", "--id", "team-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	for _, want := range []string{"team-abc-123", "Platform Team", "platform-team", "Core platform engineers", "8"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestTeamsShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"--json", "users", "teams", "show", "--id", "team-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if got := result["id"]; got != "team-abc-123" {
		t.Errorf("id = %v, want team-abc-123", got)
	}
}

func TestTeamsShowMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestTeamsCreateSuccess(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "create", "--name", "Platform Team", "--handle", "platform-team"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "Created team") {
		t.Errorf("expected 'Created team' in output, got: %s", buf.String())
	}

	data, ok := capturedBody["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data in request body: %v", capturedBody)
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing attributes in request body: %v", data)
	}
	if got := attrs["name"]; got != "Platform Team" {
		t.Errorf("name = %v, want Platform Team", got)
	}
	if got := attrs["handle"]; got != "platform-team" {
		t.Errorf("handle = %v, want platform-team", got)
	}
}

func TestTeamsCreateRequiredFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing name", []string{"users", "teams", "create", "--handle", "platform-team"}},
		{"missing handle", []string{"users", "teams", "create", "--name", "Platform Team"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Fatal("expected error for missing required flag")
			}
		})
	}
}

func TestTeamsUpdateSuccess(t *testing.T) {
	t.Parallel()

	callCount := 0
	var capturedUpdateBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == http.MethodPatch {
			if err := json.NewDecoder(r.Body).Decode(&capturedUpdateBody); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "update", "--id", "team-abc-123", "--name", "New Name"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "Updated team") {
		t.Errorf("expected 'Updated team' in output, got: %s", buf.String())
	}

	// should have called GET then PATCH
	if callCount < 2 {
		t.Errorf("expected at least 2 API calls (GET + PATCH), got %d", callCount)
	}

	if capturedUpdateBody != nil {
		data, ok := capturedUpdateBody["data"].(map[string]interface{})
		if ok {
			attrs, ok := data["attributes"].(map[string]interface{})
			if ok && attrs["name"] != "New Name" {
				t.Errorf("name in update body = %v, want New Name", attrs["name"])
			}
		}
	}
}

func TestTeamsDeleteWithYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "delete", "--id", "team-abc-123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "Deleted team") {
		t.Errorf("expected 'Deleted team' in output, got: %s", buf.String())
	}
}

func TestTeamsDeleteWithoutYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildTeamsCRUDCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "delete", "--id", "team-abc-123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes not provided")
	}
}

const mockTeamMembersResponse = `{
	"data": [{
		"type": "team_memberships",
		"id": "membership-001",
		"attributes": {
			"role": "admin"
		}
	}, {
		"type": "team_memberships",
		"id": "membership-002",
		"attributes": {}
	}]
}`

const mockTeamMembershipResponse = `{
	"data": {
		"type": "team_memberships",
		"id": "membership-new",
		"attributes": {}
	}
}`

func buildTeamsMembersCmd(mkAPI func() (*teamsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	users := &cobra.Command{Use: "users"}
	teams := &cobra.Command{Use: "teams"}
	teams.AddCommand(newTeamsMembersCmd(mkAPI))
	teams.AddCommand(newTeamsAddMemberCmd(mkAPI))
	teams.AddCommand(newTeamsRemoveMemberCmd(mkAPI))
	users.AddCommand(teams)
	root.AddCommand(users)
	return root, buf
}

func TestTeamsMembersTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamMembersResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsMembersCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "members", "--id", "team-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := buf.String()
	for _, want := range []string{"MEMBERSHIP_ID", "ROLE", "membership-001", "admin", "membership-002", "member"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot: %s", want, got)
		}
	}
}

func TestTeamsMembersJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamMembersResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsMembersCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"--json", "users", "teams", "members", "--id", "team-abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 members, got %d", len(result))
	}
}

func TestTeamsMembersMissingID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildTeamsMembersCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "members"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestTeamsAddMemberSuccess(t *testing.T) {
	t.Parallel()

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTeamMembershipResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTeamsMembersCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "add-member", "--id", "team-abc-123", "--user-id", "user-xyz-456", "--role", "admin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "Added user user-xyz-456 to team team-abc-123") {
		t.Errorf("unexpected output: %s", buf.String())
	}

	data, ok := capturedBody["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing data in request body: %v", capturedBody)
	}
	rels, ok := data["relationships"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing relationships in request body: %v", data)
	}
	user, ok := rels["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing user relationship: %v", rels)
	}
	userData, ok := user["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing user data: %v", user)
	}
	if got := userData["id"]; got != "user-xyz-456" {
		t.Errorf("user id = %v, want user-xyz-456", got)
	}
}

func TestTeamsAddMemberRequiredFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing id", []string{"users", "teams", "add-member", "--user-id", "user-xyz"}},
		{"missing user-id", []string{"users", "teams", "add-member", "--id", "team-abc-123"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildTeamsMembersCmd(newTestTeamsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Fatal("expected error for missing required flag")
			}
		})
	}
}

func TestTeamsRemoveMemberWithYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildTeamsMembersCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "remove-member", "--id", "team-abc-123", "--user-id", "user-xyz-456", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "Removed user user-xyz-456 from team team-abc-123") {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestTeamsRemoveMemberWithoutYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildTeamsMembersCmd(newTestTeamsAPI(srv))
	root.SetArgs([]string{"users", "teams", "remove-member", "--id", "team-abc-123", "--user-id", "user-xyz-456"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes not provided")
	}
}
