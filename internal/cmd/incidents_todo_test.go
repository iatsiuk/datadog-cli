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

func newTestIncidentsTodoAPI(srv *httptest.Server) func() (*incidentsAPI, error) {
	return func() (*incidentsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		cfg.SetUnstableOperationEnabled("v2.ListIncidents", true)
		cfg.SetUnstableOperationEnabled("v2.SearchIncidents", true)
		cfg.SetUnstableOperationEnabled("v2.GetIncident", true)
		cfg.SetUnstableOperationEnabled("v2.CreateIncident", true)
		cfg.SetUnstableOperationEnabled("v2.UpdateIncident", true)
		cfg.SetUnstableOperationEnabled("v2.DeleteIncident", true)
		cfg.SetUnstableOperationEnabled("v2.ListIncidentTodos", true)
		cfg.SetUnstableOperationEnabled("v2.GetIncidentTodo", true)
		cfg.SetUnstableOperationEnabled("v2.CreateIncidentTodo", true)
		cfg.SetUnstableOperationEnabled("v2.UpdateIncidentTodo", true)
		cfg.SetUnstableOperationEnabled("v2.DeleteIncidentTodo", true)
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

const mockTodoListResponse = `{
	"data": [
		{
			"id": "todo-111",
			"type": "incident_todos",
			"attributes": {
				"content": "Fix the database",
				"assignees": ["@alice"],
				"completed": null
			}
		},
		{
			"id": "todo-222",
			"type": "incident_todos",
			"attributes": {
				"content": "Notify stakeholders",
				"assignees": [],
				"completed": "2026-03-13T11:00:00Z"
			}
		}
	]
}`

const mockTodoSingleResponse = `{
	"data": {
		"id": "todo-111",
		"type": "incident_todos",
		"attributes": {
			"content": "Fix the database",
			"assignees": ["@alice"],
			"completed": null
		}
	}
}`

func TestTodoList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTodoListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "list", "inc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "CONTENT", "ASSIGNEES", "COMPLETED", "todo-111", "Fix the database", "todo-222", "Notify stakeholders"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestTodoList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTodoListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "todo", "list", "inc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"todo-111", "Fix the database"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestTodoShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTodoSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "show", "inc-111", "todo-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"todo-111", "Fix the database"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestTodoShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTodoSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"--json", "incidents", "todo", "show", "inc-111", "todo-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"todo-111", "Fix the database"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestTodoCreate_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTodoSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "create", "inc-111", "--description", "Fix the database", "--assignee", "@alice"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"todo-111", "Fix the database"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestTodoCreate_MissingDescription(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "create", "inc-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--description") {
		t.Fatalf("expected --description error, got: %v", err)
	}
}

func TestTodoUpdate_TableOutput(t *testing.T) {
	t.Parallel()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		// first call is GET (fetch current), second is PATCH (update)
		fmt.Fprint(w, mockTodoSingleResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "update", "inc-111", "todo-111", "--description", "Updated content", "--completed"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "todo-111") {
		t.Errorf("output missing todo ID\nfull output:\n%s", out)
	}
}

func TestTodoUpdate_NoArgs(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "update", "inc-111"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing todo ID, got nil")
	}
}

func TestTodoDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "delete", "inc-111", "todo-111", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(buf.String(), "todo-111") {
		t.Errorf("expected deletion confirmation, got: %s", buf.String())
	}
}

func TestTodoDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildIncidentsCmd(newTestIncidentsTodoAPI(srv))
	root.SetArgs([]string{"incidents", "todo", "delete", "inc-111", "todo-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got: %v", err)
	}
}
