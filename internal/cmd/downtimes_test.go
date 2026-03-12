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
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestDowntimesAPI(srv *httptest.Server) func() (*downtimesAPI, error) {
	return func() (*downtimesAPI, error) {
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
		return &downtimesAPI{api: datadogV2.NewDowntimesApi(c), ctx: apiCtx}, nil
	}
}

const mockDowntimesListResponse = `{
	"data": [
		{
			"id": "abc-111",
			"type": "downtime",
			"attributes": {
				"scope": "env:prod",
				"status": "active",
				"monitor_identifier": {"monitor_id": 12345},
				"schedule": {"start": "2026-03-13T10:00:00Z", "end": "2026-03-14T10:00:00Z"}
			}
		},
		{
			"id": "abc-222",
			"type": "downtime",
			"attributes": {
				"scope": "service:web",
				"status": "scheduled",
				"monitor_identifier": {"monitor_id": 67890},
				"schedule": {"start": "2026-03-15T08:00:00Z"}
			}
		}
	]
}`

func buildDowntimeListCmd(mkAPI func() (*downtimesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitorsCmd := &cobra.Command{Use: "monitors"}
	monitorsCmd.AddCommand(newDowntimeCmd(mkAPI))
	root.AddCommand(monitorsCmd)
	return root, buf
}

func TestDowntimeList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimesListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "SCOPE", "MONITOR_ID", "STATUS", "abc-111", "env:prod", "12345", "active", "abc-222", "service:web", "67890", "scheduled"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestDowntimeList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimesListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "downtime", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-111", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestDowntimeList_Empty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("expected headers in empty output, got:\n%s", out)
	}
}

const mockDowntimeShowResponse = `{
	"data": {
		"id": "abc-111",
		"type": "downtime",
		"attributes": {
			"scope": "env:prod",
			"status": "active",
			"monitor_identifier": {"monitor_id": 12345},
			"schedule": {"start": "2026-03-13T10:00:00Z", "end": "2026-03-14T10:00:00Z"},
			"message": "maintenance window"
		}
	}
}`

const mockDowntimeCreateResponse = `{
	"data": {
		"id": "abc-new",
		"type": "downtime",
		"attributes": {
			"scope": "env:staging",
			"status": "scheduled",
			"monitor_identifier": {"monitor_id": 99999}
		}
	}
}`

func TestDowntimeShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimeShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "show", "--id", "abc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-111", "env:prod", "active", "12345", "maintenance window"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestDowntimeShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimeShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "downtime", "show", "--id", "abc-111"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-111", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestDowntimeShow_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "show"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--id") {
		t.Fatalf("expected --id error, got: %v", err)
	}
}

func TestDowntimeCreate_RequestBody(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimeCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{
		"monitors", "downtime", "create",
		"--scope", "env:staging",
		"--monitor-id", "99999",
		"--message", "deploy window",
		"--start", "2026-03-13T10:00:00Z",
		"--end", "2026-03-14T10:00:00Z",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	for _, want := range []string{"env:staging", "99999", "deploy window"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "abc-new") {
		t.Errorf("output missing created ID\nfull output:\n%s", out)
	}
}

func TestDowntimeCreate_MissingScope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "create"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--scope") {
		t.Fatalf("expected --scope error, got: %v", err)
	}
}

func TestDowntimeCreate_MonitorTags(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimeCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{
		"monitors", "downtime", "create",
		"--scope", "env:staging",
		"--monitor-tags", "service:web",
		"--monitor-tags", "env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	for _, want := range []string{"service:web", "env:prod"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}
}

const mockDowntimeUpdateResponse = `{
	"data": {
		"id": "abc-111",
		"type": "downtime",
		"attributes": {
			"scope": "env:updated",
			"status": "active",
			"monitor_identifier": {"monitor_id": 12345},
			"message": "updated message"
		}
	}
}`

func TestDowntimeUpdate_ChangedFields(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockDowntimeUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{
		"monitors", "downtime", "update",
		"--id", "abc-111",
		"--scope", "env:updated",
		"--message", "updated message",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	for _, want := range []string{"env:updated", "updated message", "abc-111"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "abc-111") {
		t.Errorf("output missing updated ID\nfull output:\n%s", out)
	}
}

func TestDowntimeUpdate_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "update"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--id") {
		t.Fatalf("expected --id error, got: %v", err)
	}
}

func TestDowntimeCancel_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "cancel", "--id", "abc-111", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "abc-111") {
		t.Errorf("output missing cancelled ID\nfull output:\n%s", out)
	}
}

func TestDowntimeCancel_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildDowntimeListCmd(newTestDowntimesAPI(srv))
	root.SetArgs([]string{"monitors", "downtime", "cancel", "--id", "abc-111"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got: %v", err)
	}
}
