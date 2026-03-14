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

const mockRulesListResponse = `{
	"data": [{
		"id": "rule-abc123",
		"name": "AWS CloudTrail Rule",
		"type": "log_detection",
		"isEnabled": true,
		"message": "Unauthorized access detected",
		"cases": [{"status": "high", "condition": "a > 0"}],
		"queries": [{"query": "source:cloudtrail"}],
		"options": {}
	}]
}`

const mockRuleShowResponse = `{
	"id": "rule-abc123",
	"name": "AWS CloudTrail Rule",
	"type": "log_detection",
	"isEnabled": true,
	"message": "Unauthorized access detected",
	"cases": [{"status": "high", "condition": "a > 0"}],
	"queries": [{"query": "source:cloudtrail"}],
	"options": {}
}`

const mockRuleCreateResponse = `{
	"id": "rule-new456",
	"name": "New Rule",
	"type": "log_detection",
	"isEnabled": true,
	"message": "Signal message",
	"cases": [{"status": "medium", "condition": "a > 0"}],
	"queries": [{"query": "source:nginx"}],
	"options": {}
}`

func buildRuleCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sec.AddCommand(newSecurityRuleCmd(mkAPI))
	root.AddCommand(sec)
	return root, buf
}

func TestSecurityRuleListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRulesListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "TYPE", "IS_ENABLED", "SEVERITY", "rule-abc123", "AWS CloudTrail Rule", "log_detection", "true", "high"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecurityRuleListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRulesListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "list", "--json"})
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

func TestSecurityRuleShow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rule-abc123") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRuleShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "show", "rule-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"rule-abc123", "AWS CloudTrail Rule", "log_detection", "high", "Unauthorized access detected", "source:cloudtrail"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecurityRuleShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRuleShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "show", "rule-abc123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["id"] != "rule-abc123" {
		t.Errorf("id = %v, want rule-abc123", result["id"])
	}
}

func TestSecurityRuleCreate(t *testing.T) {
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
		fmt.Fprint(w, mockRuleCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "rule", "create",
		"--name", "New Rule",
		"--query", "source:nginx",
		"--message", "Signal message",
		"--severity", "medium",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	path := capturedPath
	mu.Unlock()

	if !strings.Contains(path, "/security_monitoring/rules") {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.Contains(string(body), "New Rule") {
		t.Errorf("body missing rule name: %s", body)
	}
	if !strings.Contains(string(body), "source:nginx") {
		t.Errorf("body missing query: %s", body)
	}
	if !strings.Contains(string(body), "medium") {
		t.Errorf("body missing severity: %s", body)
	}
}

func TestSecurityRuleCreateMissingFlags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockRuleCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRuleCmd(newTestSecurityAPI(srv))
	// missing --name, --query, --message, --severity
	root.SetArgs([]string{"security", "rule", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestSecurityRuleUpdate(t *testing.T) {
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
		fmt.Fprint(w, mockRuleShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "update", "rule-abc123", "--name", "Updated Name", "--message", "New message"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	if !strings.Contains(string(body), "Updated Name") {
		t.Errorf("body missing updated name: %s", body)
	}
	if !strings.Contains(string(body), "New message") {
		t.Errorf("body missing updated message: %s", body)
	}
}

func TestSecurityRuleDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "delete", "rule-abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error without --yes flag")
	}
}

func TestSecurityRuleDeleteWithYes(t *testing.T) {
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

	root, _ := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "rule", "delete", "rule-abc123", "--yes"})
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
	if !strings.Contains(p, "rule-abc123") {
		t.Errorf("path missing rule id: %s", p)
	}
}

func TestSecurityRuleValidate(t *testing.T) {
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
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildRuleCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "rule", "validate",
		"--name", "Test Rule",
		"--query", "source:nginx",
		"--message", "Test signal",
		"--severity", "low",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	path := capturedPath
	mu.Unlock()

	if !strings.Contains(path, "validation") {
		t.Errorf("expected validation endpoint, got: %s", path)
	}
	if !strings.Contains(string(body), "Test Rule") {
		t.Errorf("body missing rule name: %s", body)
	}
	if !strings.Contains(string(body), "low") {
		t.Errorf("body missing severity: %s", body)
	}
}
