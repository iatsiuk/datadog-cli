package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestIncidentsAPI(srv *httptest.Server) func() (*incidentsAPI, error) {
	return func() (*incidentsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		cfg.SetUnstableOperationEnabled("v2.ListIncidents", true)
		cfg.SetUnstableOperationEnabled("v2.SearchIncidents", true)
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

const mockIncidentsListResponse = `{
	"data": [
		{
			"id": "inc-111",
			"type": "incidents",
			"attributes": {
				"title": "Database outage",
				"severity": "SEV-1",
				"state": "active",
				"created": "2026-03-13T10:00:00Z"
			},
			"relationships": {
				"commander_user": {
					"data": {"id": "user-abc", "type": "users"}
				}
			}
		},
		{
			"id": "inc-222",
			"type": "incidents",
			"attributes": {
				"title": "API latency spike",
				"severity": "SEV-3",
				"state": "resolved",
				"created": "2026-03-12T08:00:00Z"
			},
			"relationships": {
				"commander_user": {
					"data": null
				}
			}
		}
	]
}`

func buildIncidentsCmd(mkAPI func() (*incidentsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(NewIncidentsCommand(mkAPI))
	return root, buf
}

func TestIncidentsList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsAPI(srv))
	root.SetArgs([]string{"incidents", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "TITLE", "SEVERITY", "STATUS", "CREATED", "inc-111", "Database outage", "SEV-1", "active", "inc-222", "API latency spike", "SEV-3", "resolved"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentsList_PageSizeFlag(t *testing.T) {
	t.Parallel()
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsAPI(srv))
	root.SetArgs([]string{"incidents", "list", "--page-size", "5"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedURL, "page") {
		t.Errorf("expected page param in URL, got: %s", capturedURL)
	}
}

func TestIncidentsList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"inc-111", "Database outage"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

const mockIncidentsSearchResponse = `{
	"data": {
		"type": "incidents_search_results",
		"attributes": {
			"incidents": [
				{
					"data": {
						"id": "inc-333",
						"type": "incidents",
						"attributes": {
							"title": "Network issue",
							"severity": "SEV-2",
							"state": "stable",
							"created": "2026-03-11T12:00:00Z"
						},
						"relationships": {
							"commander_user": {
								"data": {"id": "user-xyz", "type": "users"}
							}
						}
					}
				}
			],
			"facets": {},
			"total": 1
		}
	}
}`

func TestIncidentsSearch_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsAPI(srv))
	root.SetArgs([]string{"incidents", "search", "--query", "network"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "TITLE", "SEVERITY", "STATUS", "inc-333", "Network issue", "SEV-2", "stable"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentsSearch_MissingQuery(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsAPI(srv))
	root.SetArgs([]string{"incidents", "search"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--query") {
		t.Fatalf("expected --query error, got: %v", err)
	}
}
