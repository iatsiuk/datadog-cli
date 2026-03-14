package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

const mockFiltersListResponse = `{
	"data": [{
		"id": "flt-abc123",
		"type": "security_filters",
		"attributes": {
			"name": "Exclude staging",
			"query": "env:staging",
			"is_enabled": true,
			"filtered_data_type": "logs",
			"is_builtin": false,
			"version": 1,
			"exclusion_filters": []
		}
	}]
}`

const mockFilterShowResponse = `{
	"data": {
		"id": "flt-abc123",
		"type": "security_filters",
		"attributes": {
			"name": "Exclude staging",
			"query": "env:staging",
			"is_enabled": true,
			"filtered_data_type": "logs",
			"is_builtin": false,
			"version": 1,
			"exclusion_filters": []
		}
	}
}`

const mockFilterCreateResponse = `{
	"data": {
		"id": "flt-new456",
		"type": "security_filters",
		"attributes": {
			"name": "New Filter",
			"query": "env:prod",
			"is_enabled": true,
			"filtered_data_type": "logs",
			"is_builtin": false,
			"version": 1,
			"exclusion_filters": []
		}
	}
}`

func buildFilterCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sec.AddCommand(newSecurityFilterCmd(mkAPI))
	root.AddCommand(sec)
	return root, buf
}

func TestSecurityFilterListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFiltersListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "QUERY", "ENABLED", "TYPE", "flt-abc123", "Exclude staging", "env:staging", "true", "logs"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecurityFilterListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFiltersListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("got %d entries, want 1", len(result))
	}
}

func TestSecurityFilterShow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/flt-abc123") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFilterShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "show", "flt-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"flt-abc123", "Exclude staging", "env:staging", "true", "logs"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecurityFilterShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFilterShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "show", "flt-abc123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if data, ok := result["data"].(map[string]interface{}); !ok || data["id"] != "flt-abc123" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestSecurityFilterCreate(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedBody []byte
		capturedPath string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		capturedPath = r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFilterCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "filter", "create",
		"--name", "New Filter",
		"--query", "env:prod",
		"--filtered-data-type", "logs",
		"--is-enabled",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	path := capturedPath
	mu.Unlock()

	if !strings.Contains(path, "security_filters") {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.Contains(string(body), "New Filter") {
		t.Errorf("body missing name: %s", body)
	}
	if !strings.Contains(string(body), "env:prod") {
		t.Errorf("body missing query: %s", body)
	}
	if !strings.Contains(string(body), "logs") {
		t.Errorf("body missing filtered_data_type: %s", body)
	}
}

func TestSecurityFilterCreateMissingFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFilterCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestSecurityFilterUpdate(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFilterShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "update", "flt-abc123", "--name", "Updated Filter", "--query", "env:updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	if !strings.Contains(string(body), "Updated Filter") {
		t.Errorf("body missing updated name: %s", body)
	}
	if !strings.Contains(string(body), "env:updated") {
		t.Errorf("body missing updated query: %s", body)
	}
}

func TestSecurityFilterDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "delete", "flt-abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestSecurityFilterDeleteWithYes(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		method  string
		urlPath string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		method = r.Method
		urlPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildFilterCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "filter", "delete", "flt-abc123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	m := method
	p := urlPath
	mu.Unlock()

	if m != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", m)
	}
	if !strings.Contains(p, "flt-abc123") {
		t.Errorf("path missing filter id: %s", p)
	}
}
