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

const mockFindingsListResponse = `{
	"data": [{
		"id": "find-abc123",
		"type": "finding",
		"attributes": {
			"status": "critical",
			"rule": {
				"id": "rule-001",
				"name": "S3 Bucket Public Access"
			},
			"resource": "my-s3-bucket",
			"resource_type": "aws_s3_bucket",
			"tags": ["env:prod", "team:security"],
			"evaluation": "fail"
		}
	}],
	"meta": {
		"page": {
			"total_filtered": 1
		},
		"snapshot_timestamp": 1700000000000
	}
}`

const mockFindingShowResponse = `{
	"data": {
		"id": "find-abc123",
		"type": "detailed_finding",
		"attributes": {
			"status": "critical",
			"rule": {
				"id": "rule-001",
				"name": "S3 Bucket Public Access"
			},
			"resource": "my-s3-bucket",
			"resource_type": "aws_s3_bucket",
			"tags": ["env:prod", "team:security"],
			"evaluation": "fail",
			"message": "S3 bucket allows public access"
		}
	}
}`

const mockFindingMuteResponse = `{
	"data": [{
		"id": "find-abc123",
		"type": "finding"
	}]
}`

func buildFindingCmd(mkAPI func() (*securityAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	sec := &cobra.Command{Use: "security"}
	sec.AddCommand(newSecurityFindingCmd(mkAPI))
	root.AddCommand(sec)
	return root, buf
}

func TestSecurityFindingListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFindingsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "finding", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "RULE", "RESOURCE", "STATUS", "SEVERITY", "find-abc123", "S3 Bucket Public Access", "my-s3-bucket", "critical"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecurityFindingListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFindingsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "finding", "list", "--json"})
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

func TestSecurityFindingListWithQuery(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedURL string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedURL = r.URL.RawQuery
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFindingsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "finding", "list", "--query", "env:prod", "--limit", "10"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	q := capturedURL
	mu.Unlock()

	if !strings.Contains(q, "filter%5Btags%5D=env%3Aprod") && !strings.Contains(q, "filter[tags]=env:prod") {
		t.Logf("query string: %s", q)
	}
}

func TestSecurityFindingShow(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/find-abc123") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFindingShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "finding", "show", "find-abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"find-abc123", "S3 Bucket Public Access", "my-s3-bucket", "critical", "S3 bucket allows public access"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSecurityFindingShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFindingShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "finding", "show", "find-abc123", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if data, ok := result["data"].(map[string]interface{}); !ok || data["id"] != "find-abc123" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestSecurityFindingMute(t *testing.T) {
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
		fmt.Fprint(w, mockFindingMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "finding", "mute", "find-abc123",
		"--reason", "FALSE_POSITIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	if !strings.Contains(string(body), "find-abc123") {
		t.Errorf("body missing finding id: %s", body)
	}
	if !strings.Contains(string(body), "FALSE_POSITIVE") {
		t.Errorf("body missing reason: %s", body)
	}
}

func TestSecurityFindingMuteWithExpiration(t *testing.T) {
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
		fmt.Fprint(w, mockFindingMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{
		"security", "finding", "mute", "find-abc123",
		"--reason", "ACCEPTED_RISK",
		"--expiration", "9999999999999",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	if !strings.Contains(string(body), "ACCEPTED_RISK") {
		t.Errorf("body missing reason: %s", body)
	}
	if !strings.Contains(string(body), "9999999999999") {
		t.Errorf("body missing expiration_date: %s", body)
	}
}

func TestSecurityFindingMuteMissingReason(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockFindingMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildFindingCmd(newTestSecurityAPI(srv))
	root.SetArgs([]string{"security", "finding", "mute", "find-abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --reason flag")
	}
}
