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

func newTestIncidentsTypeAPI(srv *httptest.Server) func() (*incidentsAPI, error) {
	return func() (*incidentsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		cfg.SetUnstableOperationEnabled("v2.ListIncidentTypes", true)
		cfg.SetUnstableOperationEnabled("v2.GetIncidentType", true)
		cfg.SetUnstableOperationEnabled("v2.CreateIncidentType", true)
		cfg.SetUnstableOperationEnabled("v2.UpdateIncidentType", true)
		cfg.SetUnstableOperationEnabled("v2.DeleteIncidentType", true)
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &incidentsAPI{api: datadogV2.NewIncidentsApi(c), ctx: apiCtx}, nil
	}
}

const mockIncidentTypeListResponse = `{
	"data": [
		{
			"id": "type-111",
			"type": "incident_types",
			"attributes": {
				"name": "Security",
				"description": "Security incidents",
				"is_default": false
			}
		},
		{
			"id": "type-222",
			"type": "incident_types",
			"attributes": {
				"name": "Outage",
				"description": "Service outages",
				"is_default": true
			}
		}
	]
}`

const mockIncidentTypeSingleResponse = `{
	"data": {
		"id": "type-111",
		"type": "incident_types",
		"attributes": {
			"name": "Security",
			"description": "Security incidents",
			"is_default": false
		}
	}
}`

func TestIncidentTypeList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTypeListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "DESCRIPTION", "IS_DEFAULT", "type-111", "Security", "type-222", "Outage"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTypeList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTypeListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "type", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"type-111", "type-222"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTypeShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTypeSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "show", "type-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"type-111", "Security"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTypeShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTypeSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "type", "show", "type-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "type-111") {
		t.Errorf("JSON output missing type-111\nfull output:\n%s", buf.String())
	}
}

func TestIncidentTypeCreate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTypeSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "create", "--name", "Security", "--description", "Security incidents"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"type-111", "Security"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentTypeCreate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "create"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("expected --name error, got: %v", err)
	}
}

func TestIncidentTypeUpdate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentTypeSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "update", "type-111", "--name", "Security Updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "type-111") {
		t.Errorf("output missing type-111\nfull output:\n%s", buf.String())
	}
}

func TestIncidentTypeDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "delete", "type-111", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "type-111") {
		t.Errorf("expected deletion confirmation, got: %s", buf.String())
	}
}

func TestIncidentTypeDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTypeAPI(srv))
	root.SetArgs([]string{"incidents", "type", "delete", "type-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got: %v", err)
	}
}
