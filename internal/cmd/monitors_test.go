package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

func newTestMonitorsAPI(srv *httptest.Server) func() (*monitorsAPI, error) {
	return func() (*monitorsAPI, error) {
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
		return &monitorsAPI{api: datadogV1.NewMonitorsApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestMonitorsAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestMonitorsAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil monitorsAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV1.MonitorsApi")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestNewMonitorsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewMonitorsCommand()
	if cmd.Use != "monitors" {
		t.Errorf("Use = %q, want %q", cmd.Use, "monitors")
	}
}

const mockMonitorsListResponse = `[
	{
		"id": 12345,
		"name": "CPU High",
		"type": "metric alert",
		"overall_state": "Alert",
		"query": "avg(last_5m):avg:system.cpu.user{*} > 90"
	},
	{
		"id": 67890,
		"name": "Disk Full",
		"type": "metric alert",
		"overall_state": "OK",
		"query": "avg(last_5m):avg:system.disk.in_use{*} > 0.9"
	}
]`

func buildMonitorsListCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsListCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

func TestMonitorsList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsListCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "TYPE", "STATUS", "12345", "CPU High", "metric alert", "Alert", "67890", "Disk Full"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestMonitorsList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsListCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"id"`) {
		t.Errorf("JSON output missing id field\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "CPU High") {
		t.Errorf("JSON output missing monitor name\nfull output:\n%s", out)
	}
}

func TestMonitorsList_Empty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]") //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsListCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	// headers should still appear, no rows
	if !strings.Contains(out, "ID") {
		t.Errorf("expected headers in empty output, got:\n%s", out)
	}
}

const mockMonitorShowResponse = `{
	"id": 12345,
	"name": "CPU High",
	"type": "metric alert",
	"overall_state": "Alert",
	"query": "avg(last_5m):avg:system.cpu.user{*} > 90",
	"message": "CPU is too high",
	"tags": ["env:prod", "service:web"]
}`

func buildMonitorsShowCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsShowCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

func TestMonitorsShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "12345") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsShowCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "show", "--id", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"12345", "CPU High", "metric alert", "Alert", "CPU is too high"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestMonitorsShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsShowCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "show", "--id", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "CPU High", "metric alert"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestMonitorsShow_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildMonitorsShowCmd(newTestMonitorsAPI(srv))
	errBuf := &bytes.Buffer{}
	root.SetErr(errBuf)
	root.SetArgs([]string{"monitors", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

const mockMonitorsSearchResponse = `{
	"monitors": [
		{
			"id": 11111,
			"name": "CPU High",
			"type": "metric alert",
			"status": "Alert",
			"query": "avg(last_5m):avg:system.cpu.user{*} > 90"
		},
		{
			"id": 22222,
			"name": "Disk Full",
			"type": "metric alert",
			"status": "OK",
			"query": "avg(last_5m):avg:system.disk.in_use{*} > 0.9"
		}
	],
	"metadata": {"page": 0, "page_count": 1, "per_page": 30, "total_count": 2}
}`

func buildMonitorsSearchCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsSearchCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

func TestMonitorsSearch_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "type:metric" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsSearchCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "search", "--query", "type:metric"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "TYPE", "STATUS", "11111", "CPU High", "metric alert", "Alert", "22222", "Disk Full"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestMonitorsSearch_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsSearchCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "search"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "CPU High", "metric alert"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestMonitorsSearch_Pagination(t *testing.T) {
	t.Parallel()
	var capturedPage, capturedPerPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPage = r.URL.Query().Get("page")
		capturedPerPage = r.URL.Query().Get("per_page")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMonitorsSearchCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "search", "--page", "2", "--per-page", "10"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedPage != "2" {
		t.Errorf("page = %q, want %q", capturedPage, "2")
	}
	if capturedPerPage != "10" {
		t.Errorf("per_page = %q, want %q", capturedPerPage, "10")
	}
}
