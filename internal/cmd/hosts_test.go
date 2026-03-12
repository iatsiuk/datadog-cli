package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

func newTestHostsAPI(srv *httptest.Server) func() (*hostsAPI, error) {
	return func() (*hostsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &hostsAPI{api: datadogV1.NewHostsApi(c), ctx: apiCtx}, nil
	}
}

func newTestTagsAPI(srv *httptest.Server) func() (*tagsAPI, error) {
	return func() (*tagsAPI, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		apiCtx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &tagsAPI{api: datadogV1.NewTagsApi(c), ctx: apiCtx}, nil
	}
}

func TestNewHostsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewHostsCommand()
	if cmd.Use != "hosts" {
		t.Errorf("Use = %q, want %q", cmd.Use, "hosts")
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}
	if !subNames["tags"] {
		t.Error("expected 'tags' subcommand")
	}
}

func TestNewTestHostsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestHostsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil hostsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil api.api")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil api.ctx")
	}
}

func TestNewTestTagsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestTagsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil tagsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil api.api")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil api.ctx")
	}
}

const mockHostsListResponse = `{
	"host_list": [
		{
			"name": "web-01",
			"id": 12345,
			"aliases": ["web-01.internal"],
			"apps": ["agent", "nginx"],
			"sources": ["datadog"],
			"up": true,
			"last_reported_time": 1700000000
		}
	],
	"total_matching": 1,
	"total_returned": 1
}`

func buildHostsListCmd(mkAPI func() (*hostsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	hosts.AddCommand(newHostsListCmd(mkAPI))
	root.AddCommand(hosts)
	return root, buf
}

func TestHostsListTable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsListCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected host name in output, got: %s", out)
	}
	if !strings.Contains(out, "12345") {
		t.Errorf("expected host id in output, got: %s", out)
	}
	if !strings.Contains(out, "agent, nginx") {
		t.Errorf("expected apps in output, got: %s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected up=true in output, got: %s", out)
	}
}

func TestHostsListJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsListCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"--json", "hosts", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("expected 1 host, got %d", len(result))
	}
}

func TestHostsListFilter(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"host_list":[],"total_matching":0,"total_returned":0}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildHostsListCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "list", "--filter", "web"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("filter"); got != "web" {
		t.Errorf("filter = %q, want %q", got, "web")
	}
}

func TestHostsListFrom(t *testing.T) {
	t.Parallel()

	var (
		mu          sync.Mutex
		capturedReq *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"host_list":[],"total_matching":0,"total_returned":0}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildHostsListCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "list", "--from", "1700000000"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("from"); got != "1700000000" {
		t.Errorf("from = %q, want %q", got, "1700000000")
	}
}

const mockHostsTotalsResponse = `{"total_active":42,"total_up":38}`

func buildHostsTotalsCmd(mkAPI func() (*hostsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	hosts.AddCommand(newHostsTotalsCmd(mkAPI))
	root.AddCommand(hosts)
	return root, buf
}

func TestHostsTotalsTable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostsTotalsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsTotalsCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "totals"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "42") {
		t.Errorf("expected total_active in output, got: %s", out)
	}
	if !strings.Contains(out, "38") {
		t.Errorf("expected total_up in output, got: %s", out)
	}
}

func TestHostsTotalsJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostsTotalsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsTotalsCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"--json", "hosts", "totals"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if got := result["total_active"]; got != float64(42) {
		t.Errorf("total_active = %v, want 42", got)
	}
	if got := result["total_up"]; got != float64(38) {
		t.Errorf("total_up = %v, want 38", got)
	}
}

func TestHostsListEmpty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"host_list":[],"total_matching":0,"total_returned":0}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsListCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	// only headers line expected
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected headers in output, got: %s", out)
	}
}
