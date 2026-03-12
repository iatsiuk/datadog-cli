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

func TestNewTestDowntimesAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestDowntimesAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil downtimesAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV2.DowntimesApi")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil context")
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
