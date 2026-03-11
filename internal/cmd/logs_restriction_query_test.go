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

func newTestLogsRQAPI(srv *httptest.Server) func() (*logsRestrictionQueryAPI, error) {
	return func() (*logsRestrictionQueryAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		for _, op := range []string{
			"v2.ListRestrictionQueries",
			"v2.GetRestrictionQuery",
			"v2.CreateRestrictionQuery",
			"v2.UpdateRestrictionQuery",
			"v2.DeleteRestrictionQuery",
		} {
			cfg.SetUnstableOperationEnabled(op, true)
		}
		c := datadog.NewAPIClient(cfg)
		ctx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &logsRestrictionQueryAPI{api: datadogV2.NewLogsRestrictionQueriesApi(c), ctx: ctx}, nil
	}
}

func buildRQCmd(mkAPI func() (*logsRestrictionQueryAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(newLogsRestrictionQueryCmd(mkAPI))
	return root, buf
}

const mockRQListResponse = `{
	"data": [
		{
			"type": "logs_restriction_queries",
			"id": "rq-abc123",
			"attributes": {
				"restriction_query": "service:web",
				"role_count": 2,
				"user_count": 5
			}
		},
		{
			"type": "logs_restriction_queries",
			"id": "rq-def456",
			"attributes": {
				"restriction_query": "env:prod",
				"role_count": 1,
				"user_count": 0
			}
		}
	]
}`

const mockRQResponse = `{
	"data": {
		"type": "logs_restriction_queries",
		"id": "rq-abc123",
		"attributes": {
			"restriction_query": "service:web",
			"role_count": 2,
			"user_count": 5
		}
	}
}`

func TestLogsRQListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "QUERY", "ROLES", "USERS", "rq-abc123", "service:web", "2", "5"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsRQListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 2 {
		t.Errorf("got %d entries, want 2", len(result))
	}
}

func TestLogsRQShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "show", "rq-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "QUERY", "rq-abc123", "service:web"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLogsRQShowRequiresID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestLogsRQCreateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(capturedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{
		"restriction-query", "create",
		"--query", "service:web",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data in request")
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("missing attributes in request")
	}
	if got := attrs["restriction_query"]; got != "service:web" {
		t.Errorf("restriction_query = %v, want 'service:web'", got)
	}
}

func TestLogsRQCreateRequiredFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestLogsRQUpdateFlags(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(capturedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRQResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{
		"restriction-query", "update", "rq-abc123",
		"--query", "env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("no request body captured")
	}
	var req map[string]interface{}
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("invalid request body JSON: %v", err)
	}
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing data in request")
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("missing attributes in request")
	}
	if got := attrs["restriction_query"]; got != "env:prod" {
		t.Errorf("restriction_query = %v, want 'env:prod'", got)
	}
}

func TestLogsRQDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "delete", "rq-abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestLogsRQDeleteWithYes(t *testing.T) {
	t.Parallel()

	var deletedID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		deletedID = parts[len(parts)-1]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildRQCmd(newTestLogsRQAPI(srv))
	root.SetArgs([]string{"restriction-query", "delete", "rq-abc123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if deletedID != "rq-abc123" {
		t.Errorf("deleted id = %q, want %q", deletedID, "rq-abc123")
	}
	if !strings.Contains(buf.String(), "rq-abc123") {
		t.Errorf("output should mention deleted id:\n%s", buf.String())
	}
}
