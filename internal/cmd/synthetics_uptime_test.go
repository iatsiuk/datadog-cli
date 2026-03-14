package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func buildSyntheticsUptimeCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsUptimeCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

const mockSyntheticsUptimesResponse = `[
	{
		"public_id": "abc-123-def",
		"from_ts": 1700000000,
		"to_ts": 1700086400,
		"overall": {
			"uptime": 0.9950,
			"group": "overall"
		}
	},
	{
		"public_id": "ghi-456-jkl",
		"from_ts": 1700000000,
		"to_ts": 1700086400,
		"overall": {
			"uptime": 0.8800
		}
	}
]`

func TestSyntheticsUptime_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/synthetics/tests/uptimes" && r.Method == http.MethodPost {
			fmt.Fprint(w, mockSyntheticsUptimesResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsUptimeCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"synthetics", "uptime",
		"--id", "abc-123-def,ghi-456-jkl",
		"--from", "now-24h",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"PUBLIC_ID", "UPTIME_%", "abc-123-def", "99.50", "ghi-456-jkl", "88.00"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsUptime_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/synthetics/tests/uptimes" && r.Method == http.MethodPost {
			fmt.Fprint(w, mockSyntheticsUptimesResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsUptimeCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{
		"--json",
		"synthetics", "uptime",
		"--id", "abc-123-def",
		"--from", "now-1h",
		"--to", "now",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-123-def", "0.995"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsUptime_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsUptimeCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "uptime", "--from", "now-1h"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --id, got nil")
	}
	if !strings.Contains(err.Error(), "--id") {
		t.Errorf("error should mention --id, got: %v", err)
	}
}

func TestSyntheticsUptime_MissingFrom(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsUptimeCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "uptime", "--id", "abc-123-def"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --from, got nil")
	}
	if !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should mention --from, got: %v", err)
	}
}
