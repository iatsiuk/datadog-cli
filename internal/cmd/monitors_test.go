package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

func buildMonitorsCreateCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsCreateCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

const mockMonitorCreateResponse = `{
	"id": 99999,
	"name": "My Monitor",
	"type": "metric alert",
	"overall_state": "No Data",
	"query": "avg(last_5m):avg:system.cpu.user{*} > 90",
	"message": "Alert message",
	"tags": ["env:prod"],
	"priority": 2
}`

func TestMonitorsCreate_RequestBody(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMonitorsCreateCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{
		"monitors", "create",
		"--name", "My Monitor",
		"--type", "metric alert",
		"--query", "avg(last_5m):avg:system.cpu.user{*} > 90",
		"--message", "Alert message",
		"--tags", "env:prod",
		"--priority", "2",
		"--thresholds", `{"critical":90,"warning":80}`,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	for _, want := range []string{"My Monitor", "metric alert", "avg(last_5m)", "Alert message", "env:prod"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}
}

func TestMonitorsCreate_RequiredFlags(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing name",
			args: []string{"monitors", "create", "--type", "metric alert", "--query", "avg(last_5m):avg:system.cpu.user{*} > 90"},
		},
		{
			name: "missing type",
			args: []string{"monitors", "create", "--name", "My Monitor", "--query", "avg(last_5m):avg:system.cpu.user{*} > 90"},
		},
		{
			name: "missing query",
			args: []string{"monitors", "create", "--name", "My Monitor", "--type", "metric alert"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildMonitorsCreateCmd(newTestMonitorsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestMonitorsCreate_OptionalFlags(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMonitorCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMonitorsCreateCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{
		"monitors", "create",
		"--name", "My Monitor",
		"--type", "metric alert",
		"--query", "avg(last_5m):avg:system.cpu.user{*} > 90",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// optional fields absent -- body should still be valid
	if len(capturedBody) == 0 {
		t.Error("expected non-empty request body")
	}
}

func buildMonitorsUpdateCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsUpdateCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

const mockMonitorUpdateResponse = `{
	"id": 12345,
	"name": "CPU High Updated",
	"type": "metric alert",
	"overall_state": "No Data",
	"query": "avg(last_5m):avg:system.cpu.user{*} > 95",
	"message": "Updated message",
	"tags": ["env:prod"]
}`

func TestMonitorsUpdate_RequestBody(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
			return
		}
		if r.Method == http.MethodPut {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		fmt.Fprint(w, mockMonitorUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMonitorsUpdateCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{
		"monitors", "update",
		"--id", "12345",
		"--name", "CPU High Updated",
		"--query", "avg(last_5m):avg:system.cpu.user{*} > 95",
		"--message", "Updated message",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	// > is HTML-escaped to \u003e in Go JSON output
	for _, want := range []string{"CPU High Updated", "system.cpu.user", "95", "Updated message"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}
}

func TestMonitorsUpdate_PreservesUnchangedFields(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
			return
		}
		if r.Method == http.MethodPut {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		fmt.Fprint(w, mockMonitorUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	// only change name, query and message should come from the existing monitor
	root, _ := buildMonitorsUpdateCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{
		"monitors", "update",
		"--id", "12345",
		"--name", "CPU High Updated",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	// > is HTML-escaped to \u003e in Go JSON; check for surrounding context instead
	if !strings.Contains(body, "system.cpu.user") || !strings.Contains(body, "90") {
		t.Errorf("request body should preserve original query\nbody: %s", body)
	}
	if !strings.Contains(body, "CPU High Updated") {
		t.Errorf("request body missing updated name\nbody: %s", body)
	}
}

func TestMonitorsUpdate_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildMonitorsUpdateCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "update", "--name", "New Name"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

func buildMonitorsDeleteCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsDeleteCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

func TestMonitorsDelete_Success(t *testing.T) {
	t.Parallel()
	var capturedMethod string
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "{}") //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsDeleteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "delete", "--id", "12345", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if !strings.Contains(capturedPath, "12345") {
		t.Errorf("path %q does not contain monitor ID", capturedPath)
	}

	out := buf.String()
	if !strings.Contains(out, "12345") {
		t.Errorf("output missing monitor ID\nfull output:\n%s", out)
	}
}

func TestMonitorsDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildMonitorsDeleteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "delete", "--id", "12345"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes is missing")
	}
}

func buildMonitorsMuteCmd(mkAPI func() (*monitorsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newMonitorsMuteCmd(mkAPI))
	monitors.AddCommand(newMonitorsUnmuteCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

const mockMonitorMuteResponse = `{
	"id": 12345,
	"name": "CPU High",
	"type": "metric alert",
	"overall_state": "Alert",
	"query": "avg(last_5m):avg:system.cpu.user{*} > 90",
	"message": "CPU is too high",
	"tags": ["env:prod"],
	"options": {"silenced": {"*": 0}}
}`

func TestMonitorsMute_Success(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
			return
		}
		if r.Method == http.MethodPut {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		fmt.Fprint(w, mockMonitorMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsMuteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "mute", "--id", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "12345") {
		t.Errorf("output missing monitor ID\nfull output:\n%s", out)
	}

	body := string(capturedBody)
	if !strings.Contains(body, "silenced") {
		t.Errorf("request body missing silenced field\nbody: %s", body)
	}
}

func TestMonitorsMute_WithEnd(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
			return
		}
		if r.Method == http.MethodPut {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		fmt.Fprint(w, mockMonitorMuteResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMonitorsMuteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "mute", "--id", "12345", "--end", "9999999999"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	if !strings.Contains(body, "silenced") {
		t.Errorf("request body missing silenced field\nbody: %s", body)
	}
	if !strings.Contains(body, "9999999999") {
		t.Errorf("request body missing end timestamp\nbody: %s", body)
	}
}

func TestMonitorsMute_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildMonitorsMuteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "mute"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

func TestMonitorsUnmute_Success(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockMonitorMuteResponse) //nolint:errcheck
			return
		}
		if r.Method == http.MethodPut {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		fmt.Fprint(w, mockMonitorShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMonitorsMuteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "unmute", "--id", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "12345") {
		t.Errorf("output missing monitor ID\nfull output:\n%s", out)
	}

	// silenced should be empty map after unmute
	body := string(capturedBody)
	if !strings.Contains(body, "silenced") {
		t.Errorf("request body missing silenced field\nbody: %s", body)
	}
}

func TestMonitorsUnmute_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildMonitorsMuteCmd(newTestMonitorsAPI(srv))
	root.SetArgs([]string{"monitors", "unmute"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id is missing")
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
