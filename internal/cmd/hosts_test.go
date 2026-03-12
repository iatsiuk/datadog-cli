package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const mockHostMuteResponse = `{"action":"Muted","hostname":"web-01","message":"maintenance"}`
const mockHostUnmuteResponse = `{"action":"Unmuted","hostname":"web-01"}`

func buildHostsMuteCmd(mkAPI func() (*hostsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	hosts.AddCommand(newHostsMuteCmd(mkAPI))
	hosts.AddCommand(newHostsUnmuteCmd(mkAPI))
	root.AddCommand(hosts)
	return root, buf
}

func TestHostsMuteSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsMuteCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "mute", "--name", "web-01"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected hostname in output, got: %s", out)
	}
}

func TestHostsMuteWithOptions(t *testing.T) {
	t.Parallel()

	var (
		mu           sync.Mutex
		capturedReq  *http.Request
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedReq = r
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildHostsMuteCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "mute", "--name", "web-01", "--end", "1700000000", "--message", "maintenance", "--override"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	body := capturedBody
	mu.Unlock()

	if req == nil {
		t.Fatal("no request made to mock server")
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if got := settings["end"]; got != float64(1700000000) {
		t.Errorf("end = %v, want 1700000000", got)
	}
	if got := settings["message"]; got != "maintenance" {
		t.Errorf("message = %v, want maintenance", got)
	}
	if got := settings["override"]; got != true {
		t.Errorf("override = %v, want true", got)
	}
}

func TestHostsMuteMissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildHostsMuteCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "mute"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --name is missing, got nil")
	}
}

func TestHostsUnmuteSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostUnmuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildHostsMuteCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "unmute", "--name", "web-01"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected hostname in output, got: %s", out)
	}
}

func TestHostsUnmuteMissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockHostUnmuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildHostsMuteCmd(newTestHostsAPI(srv))
	root.SetArgs([]string{"hosts", "unmute"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --name is missing, got nil")
	}
}

const mockTagsHostResponse = `{"host":"web-01","tags":["env:prod","role:frontend"]}`

func buildTagsShowCmd(mkAPI func() (*tagsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	tagsCmd := &cobra.Command{Use: "tags"}
	tagsCmd.AddCommand(newTagsShowCmd(mkAPI))
	hosts.AddCommand(tagsCmd)
	root.AddCommand(hosts)
	return root, buf
}

func buildTagsCreateCmd(mkAPI func() (*tagsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	tagsCmd := &cobra.Command{Use: "tags"}
	tagsCmd.AddCommand(newTagsCreateCmd(mkAPI))
	hosts.AddCommand(tagsCmd)
	root.AddCommand(hosts)
	return root, buf
}

func TestTagsShowTable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTagsShowCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "show", "--name", "web-01"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected hostname in output, got: %s", out)
	}
	if !strings.Contains(out, "env:prod") {
		t.Errorf("expected tag in output, got: %s", out)
	}
}

func TestTagsShowJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTagsShowCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"--json", "hosts", "tags", "show", "--name", "web-01"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	if got := result["host"]; got != "web-01" {
		t.Errorf("host = %v, want web-01", got)
	}
}

func TestTagsShowMissingName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildTagsShowCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "show"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --name is missing, got nil")
	}
}

func TestTagsCreateSuccess(t *testing.T) {
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
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTagsCreateCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "create", "--name", "web-01", "--tags", "env:prod,role:frontend"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected hostname in output, got: %s", out)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	tagsList, ok := reqBody["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags array in request body, got: %v", reqBody)
	}
	if len(tagsList) != 2 {
		t.Errorf("expected 2 tags in request, got %d", len(tagsList))
	}
}

func TestTagsCreateMissingFlags(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing name", []string{"hosts", "tags", "create", "--tags", "env:prod"}},
		{"missing tags", []string{"hosts", "tags", "create", "--name", "web-01"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildTagsCreateCmd(newTestTagsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func buildTagsUpdateCmd(mkAPI func() (*tagsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	tagsCmd := &cobra.Command{Use: "tags"}
	tagsCmd.AddCommand(newTagsUpdateCmd(mkAPI))
	hosts.AddCommand(tagsCmd)
	root.AddCommand(hosts)
	return root, buf
}

func buildTagsDeleteCmd(mkAPI func() (*tagsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	tagsCmd := &cobra.Command{Use: "tags"}
	tagsCmd.AddCommand(newTagsDeleteCmd(mkAPI))
	hosts.AddCommand(tagsCmd)
	root.AddCommand(hosts)
	return root, buf
}

func TestTagsUpdateSuccess(t *testing.T) {
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
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTagsUpdateCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "update", "--name", "web-01", "--tags", "env:prod,role:frontend"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected hostname in output, got: %s", out)
	}

	mu.Lock()
	body := capturedBody
	mu.Unlock()

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	tagsList, ok := reqBody["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags array in request body, got: %v", reqBody)
	}
	if len(tagsList) != 2 {
		t.Errorf("expected 2 tags in request, got %d", len(tagsList))
	}
}

func TestTagsUpdateMissingFlags(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsHostResponse) //nolint:errcheck
	}))
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing name", []string{"hosts", "tags", "update", "--tags", "env:prod"}},
		{"missing tags", []string{"hosts", "tags", "update", "--name", "web-01"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildTagsUpdateCmd(newTestTagsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestTagsDeleteSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildTagsDeleteCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "delete", "--name", "web-01", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestTagsDeleteMissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildTagsDeleteCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "delete", "--name", "web-01"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when --yes is missing, got nil")
	}
}

const mockTagsListResponse = `{"tags":{"env:prod":["web-01","web-02"],"role:frontend":["web-01"]}}`

func buildTagsListCmd(mkAPI func() (*tagsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	hosts := &cobra.Command{Use: "hosts"}
	tagsCmd := &cobra.Command{Use: "tags"}
	tagsCmd.AddCommand(newTagsListCmd(mkAPI))
	hosts.AddCommand(tagsCmd)
	root.AddCommand(hosts)
	return root, buf
}

func TestTagsListTable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTagsListCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "env:prod") {
		t.Errorf("expected tag in output, got: %s", out)
	}
	if !strings.Contains(out, "web-01") {
		t.Errorf("expected host in output, got: %s", out)
	}
}

func TestTagsListJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildTagsListCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"--json", "hosts", "tags", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, buf.String())
	}
	tags, ok := result["tags"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tags' object in response, got: %v", result)
	}
	if _, exists := tags["env:prod"]; !exists {
		t.Errorf("expected 'env:prod' tag in response")
	}
}

func TestTagsListSource(t *testing.T) {
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
		fmt.Fprint(w, `{"tags":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildTagsListCmd(newTestTagsAPI(srv))
	root.SetArgs([]string{"hosts", "tags", "list", "--source", "users"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	mu.Lock()
	req := capturedReq
	mu.Unlock()
	if req == nil {
		t.Fatal("no request made to mock server")
	}
	if got := req.URL.Query().Get("source"); got != "users" {
		t.Errorf("source = %q, want %q", got, "users")
	}
}
