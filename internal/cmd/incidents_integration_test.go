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

func newTestIncidentsIntegrationAPI(srv *httptest.Server) func() (*incidentsAPI, error) {
	return func() (*incidentsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		cfg.SetUnstableOperationEnabled("v2.ListIncidentIntegrations", true)
		cfg.SetUnstableOperationEnabled("v2.GetIncidentIntegration", true)
		cfg.SetUnstableOperationEnabled("v2.CreateIncidentIntegration", true)
		cfg.SetUnstableOperationEnabled("v2.UpdateIncidentIntegration", true)
		cfg.SetUnstableOperationEnabled("v2.DeleteIncidentIntegration", true)
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

const mockIntegrationListResponse = `{
	"data": [
		{
			"id": "integ-111",
			"type": "incident_integrations",
			"attributes": {
				"integration_type": 1,
				"metadata": {"channels": [{"channel_id": "C123", "channel_name": "#incidents", "redirect_url": "https://slack.com"}]},
				"status": 2
			}
		},
		{
			"id": "integ-222",
			"type": "incident_integrations",
			"attributes": {
				"integration_type": 8,
				"metadata": {"issues": [{"project_key": "OPS", "issue_key": "OPS-123", "redirect_url": "https://jira.example.com"}]},
				"status": 1
			}
		}
	]
}`

const mockIntegrationSingleResponse = `{
	"data": {
		"id": "integ-111",
		"type": "incident_integrations",
		"attributes": {
			"integration_type": 1,
			"metadata": {"channels": [{"channel_id": "C123", "channel_name": "#incidents", "redirect_url": "https://slack.com"}]},
			"status": 2
		}
	}
}`

func TestIntegrationList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIntegrationListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "list", "inc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "TYPE", "STATUS", "integ-111", "slack", "integ-222", "jira"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIntegrationList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIntegrationListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "integration", "list", "inc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"integ-111", "integ-222"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIntegrationShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIntegrationSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "show", "inc-111", "integ-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"integ-111", "slack"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIntegrationShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIntegrationSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "integration", "show", "inc-111", "integ-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "integ-111") {
		t.Errorf("JSON output missing integ-111\nfull output:\n%s", out)
	}
}

func TestIntegrationCreate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIntegrationSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	meta := `{"channels":[{"channel_id":"C123","channel_name":"#incidents","redirect_url":"https://slack.com"}]}`
	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "create", "inc-111", "--type", "slack", "--metadata", meta})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"integ-111", "slack"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIntegrationCreate_MissingType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "create", "inc-111", "--metadata", "{}"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--type") {
		t.Fatalf("expected --type error, got: %v", err)
	}
}

func TestIntegrationCreate_MissingMetadata(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "create", "inc-111", "--type", "slack"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--metadata") {
		t.Fatalf("expected --metadata error, got: %v", err)
	}
}

func TestIntegrationUpdate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIntegrationSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	meta := `{"channels":[{"channel_id":"C456","channel_name":"#alerts","redirect_url":"https://slack.com"}]}`
	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "update", "inc-111", "integ-111", "--metadata", meta})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "integ-111") {
		t.Errorf("output missing integ-111\nfull output:\n%s", out)
	}
}

func TestIntegrationDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "delete", "inc-111", "integ-111", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "integ-111") {
		t.Errorf("expected deletion confirmation, got: %s", buf.String())
	}
}

func TestIntegrationDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsIntegrationAPI(srv))
	root.SetArgs([]string{"incidents", "integration", "delete", "inc-111", "integ-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got: %v", err)
	}
}
