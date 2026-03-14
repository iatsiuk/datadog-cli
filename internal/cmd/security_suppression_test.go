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

const mockSuppressionsListResponse = `{
	"data": [{
		"id": "supp-abc123",
		"type": "suppressions",
		"attributes": {
			"name": "Suppress CloudTrail noise",
			"rule_query": "source:cloudtrail",
			"suppression_query": "user:bot",
			"enabled": true,
			"expiration_date": 1893456000000,
			"description": "Suppress bot traffic"
		}
	}]
}`

const mockSuppressionShowResponse = `{
	"data": {
		"id": "supp-abc123",
		"type": "suppressions",
		"attributes": {
			"name": "Suppress CloudTrail noise",
			"rule_query": "source:cloudtrail",
			"suppression_query": "user:bot",
			"enabled": true,
			"expiration_date": 1893456000000,
			"description": "Suppress bot traffic"
		}
	}
}`

const mockSuppressionCreateResponse = `{
	"data": {
		"id": "supp-new456",
		"type": "suppressions",
		"attributes": {
			"name": "New Suppression",
			"rule_query": "source:nginx",
			"enabled": true
		}
	}
}`

func buildSuppressionCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sec.AddCommand(newSecuritySuppressionCmd(mkAPI))
	root.AddCommand(sec)
	return root, buf
}

func TestSecuritySuppressionListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSuppressionsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "RULE_QUERY", "ENABLED", "EXPIRATION", "supp-abc123", "Suppress CloudTrail noise", "source:cloudtrail", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecuritySuppressionListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSuppressionsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "list", "--json"})
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

func TestSecuritySuppressionShow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/supp-abc123") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSuppressionShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "show", "supp-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"supp-abc123", "Suppress CloudTrail noise", "source:cloudtrail", "user:bot", "true", "Suppress bot traffic"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecuritySuppressionShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSuppressionShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "show", "supp-abc123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if data, ok := result["data"].(map[string]interface{}); !ok || data["id"] != "supp-abc123" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestSecuritySuppressionCreate(t *testing.T) {
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
		fmt.Fprint(w, mockSuppressionCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "suppression", "create",
		"--name", "New Suppression",
		"--rule-query", "source:nginx",
		"--suppression-query", "user:robot",
		"--expiration", "2030-01-01",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	path := capturedPath
	mu.Unlock()

	if !strings.Contains(path, "suppressions") {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.Contains(string(body), "New Suppression") {
		t.Errorf("body missing name: %s", body)
	}
	if !strings.Contains(string(body), "source:nginx") {
		t.Errorf("body missing rule_query: %s", body)
	}
	if !strings.Contains(string(body), "user:robot") {
		t.Errorf("body missing suppression_query: %s", body)
	}
}

func TestSecuritySuppressionCreateMissingFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSuppressionCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestSecuritySuppressionCreateInvalidExpiration(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "suppression", "create",
		"--name", "Test",
		"--rule-query", "source:test",
		"--expiration", "not-a-date",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid expiration date")
	}
}

func TestSecuritySuppressionUpdate(t *testing.T) {
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
		fmt.Fprint(w, mockSuppressionShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "update", "supp-abc123", "--name", "Updated Suppression", "--rule-query", "source:updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	if !strings.Contains(string(body), "Updated Suppression") {
		t.Errorf("body missing updated name: %s", body)
	}
	if !strings.Contains(string(body), "source:updated") {
		t.Errorf("body missing updated rule_query: %s", body)
	}
}

func TestSecuritySuppressionDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "delete", "supp-abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestSecuritySuppressionDeleteWithYes(t *testing.T) {
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

	root, _ := buildSuppressionCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "suppression", "delete", "supp-abc123", "--yes"})
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
	if !strings.Contains(p, "supp-abc123") {
		t.Errorf("path missing suppression id: %s", p)
	}
}
