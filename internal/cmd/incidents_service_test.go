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

func newTestIncidentsServiceAPI(srv *httptest.Server) func() (*incidentsAPI, error) {
	return func() (*incidentsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		cfg.SetUnstableOperationEnabled("v2.ListIncidentServices", true)
		cfg.SetUnstableOperationEnabled("v2.GetIncidentService", true)
		cfg.SetUnstableOperationEnabled("v2.CreateIncidentService", true)
		cfg.SetUnstableOperationEnabled("v2.UpdateIncidentService", true)
		cfg.SetUnstableOperationEnabled("v2.DeleteIncidentService", true)
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
			api:         datadogV2.NewIncidentsApi(c),
			servicesApi: datadogV2.NewIncidentServicesApi(c),
			ctx:         apiCtx,
		}, nil
	}
}

const mockIncidentServiceListResponse = `{
	"data": [
		{
			"id": "svc-111",
			"type": "services",
			"attributes": {
				"name": "Payments"
			}
		},
		{
			"id": "svc-222",
			"type": "services",
			"attributes": {
				"name": "Authentication"
			}
		}
	]
}`

const mockIncidentServiceSingleResponse = `{
	"data": {
		"id": "svc-111",
		"type": "services",
		"attributes": {
			"name": "Payments"
		}
	}
}`

func TestIncidentServiceList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentServiceListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "svc-111", "Payments", "svc-222", "Authentication"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentServiceList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentServiceListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "service", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"svc-111", "svc-222"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentServiceShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentServiceSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "show", "svc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"svc-111", "Payments"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentServiceShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentServiceSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "service", "show", "svc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "svc-111") {
		t.Errorf("JSON output missing svc-111\nfull output:\n%s", buf.String())
	}
}

func TestIncidentServiceCreate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentServiceSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "create", "--name", "Payments"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"svc-111", "Payments"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestIncidentServiceCreate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "create"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("expected --name error, got: %v", err)
	}
}

func TestIncidentServiceUpdate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockIncidentServiceSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "update", "svc-111", "--name", "Payments Updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "svc-111") {
		t.Errorf("output missing svc-111\nfull output:\n%s", buf.String())
	}
}

func TestIncidentServiceUpdate_MissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "update", "svc-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("expected --name error, got: %v", err)
	}
}

func TestIncidentServiceDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "delete", "svc-111", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "svc-111") {
		t.Errorf("expected deletion confirmation, got: %s", buf.String())
	}
}

func TestIncidentServiceDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsServiceAPI(srv))
	root.SetArgs([]string{"incidents", "service", "delete", "svc-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got: %v", err)
	}
}
