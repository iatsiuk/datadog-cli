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
