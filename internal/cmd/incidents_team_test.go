package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

func newTestIncidentsTeamAPI(srv *httptest.Server) func() (*incidentsAPI, error) {
	return func() (*incidentsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		cfg.SetUnstableOperationEnabled("v2.ListIncidentTeams", true)
		cfg.SetUnstableOperationEnabled("v2.GetIncidentTeam", true)
		cfg.SetUnstableOperationEnabled("v2.CreateIncidentTeam", true)
		cfg.SetUnstableOperationEnabled("v2.UpdateIncidentTeam", true)
		cfg.SetUnstableOperationEnabled("v2.DeleteIncidentTeam", true)
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &incidentsAPI{
			api:      datadogV2.NewIncidentsApi(c),
			teamsApi: datadogV2.NewIncidentTeamsApi(c),
			ctx:      apiCtx,
		}, nil
	}
}

const mockIncidentTeamListResponse = `{
	"data": [
		{
			"id": "team-111",
			"type": "teams",
			"attributes": {
				"name": "Platform"
			}
		},
		{
			"id": "team-222",
			"type": "teams",
			"attributes": {
				"name": "Backend"
			}
		}
	]
}`

const mockIncidentTeamSingleResponse = `{
	"data": {
		"id": "team-111",
		"type": "teams",
		"attributes": {
			"name": "Platform"
		}
	}
}`

func TestIncidentTeamList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "team-111", "Platform", "team-222", "Backend"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTeamList_Filter(t *testing.T) {
	t.Parallel()
	var gotFilter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFilter = r.URL.Query().Get("filter")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "list", "--filter", "Platform"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotFilter != "Platform" {
		t.Errorf("expected filter %q, got %q", "Platform", gotFilter)
	}
}

func TestIncidentTeamList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "team", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"team-111", "team-222"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTeamShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "show", "team-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"team-111", "Platform"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTeamShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "team", "show", "team-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "team-111") {
		t.Errorf("JSON output missing team-111\nfull output:\n%s", buf.String())
	}
}

func TestIncidentTeamCreate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "create", "--name", "Platform"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"team-111", "Platform"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTeamCreate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "create"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("expected --name error, got: %v", err)
	}
}

func TestIncidentTeamUpdate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTeamSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "update", "team-111", "--name", "Platform Updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "team-111") {
		t.Errorf("output missing team-111\nfull output:\n%s", buf.String())
	}
}

func TestIncidentTeamUpdate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "update", "team-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("expected --name error, got: %v", err)
	}
}

func TestIncidentTeamDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "delete", "team-111", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "team-111") {
		t.Errorf("expected deletion confirmation, got: %s", buf.String())
	}
}

func TestIncidentTeamDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTeamAPI(srv))
	root.SetArgs([]string{"incidents", "team", "delete", "team-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got: %v", err)
	}
}
