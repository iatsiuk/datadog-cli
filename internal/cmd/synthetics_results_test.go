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

const mockSyntheticsAPITestLatestResultsResponse = `{
	"last_timestamp_fetched": 1700000100000,
	"results": [
		{
			"result_id": "result-api-001",
			"probe_dc": "aws:us-east-1",
			"status": 0,
			"check_time": 1700000000000,
			"result": {
				"passed": true,
				"timings": {"total": 125.4}
			}
		},
		{
			"result_id": "result-api-002",
			"probe_dc": "aws:eu-west-1",
			"status": 1,
			"check_time": 1700000060000,
			"result": {
				"passed": false,
				"timings": {"total": 542.1}
			}
		}
	]
}`

const mockSyntheticsBrowserTestLatestResultsResponse = `{
	"last_timestamp_fetched": 1700000100000,
	"results": [
		{
			"result_id": "result-browser-001",
			"probe_dc": "aws:us-west-2",
			"status": 0,
			"check_time": 1700000000000,
			"result": {
				"duration": 3200.5,
				"errorCount": 0,
				"stepCountCompleted": 5,
				"stepCountTotal": 5
			}
		}
	]
}`

const mockSyntheticsAPITestResultFullResponse = `{
	"result_id": "result-api-001",
	"probe_dc": "aws:us-east-1",
	"status": 0,
	"check_time": 1700000000000,
	"result": {
		"passed": true,
		"timings": {"total": 125.4}
	}
}`

const mockSyntheticsBrowserTestResultFullResponse = `{
	"result_id": "result-browser-001",
	"probe_dc": "aws:us-west-2",
	"status": 0,
	"check_time": 1700000000000,
	"result": {
		"duration": 3200.5,
		"errorCount": 0,
		"stepCountCompleted": 5,
		"stepCountTotal": 5
	}
}`

func buildSyntheticsResultsCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsResultsCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

func TestSyntheticsResults_APITestLatest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/synthetics/tests/abc-123-def":
			fmt.Fprint(w, `{"public_id":"abc-123-def","name":"Homepage API Test","type":"api","status":"live"}`) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/api/v1/synthetics/tests/abc-123-def/results"):
			fmt.Fprint(w, mockSyntheticsAPITestLatestResultsResponse) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root, buf := buildSyntheticsResultsCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "results", "abc-123-def"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"RESULT_ID", "LOCATION", "STATUS", "TIMESTAMP", "result-api-001", "aws:us-east-1", "result-api-002"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsResults_BrowserTestLatest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/synthetics/tests/ghi-456-jkl":
			fmt.Fprint(w, `{"public_id":"ghi-456-jkl","name":"Login Browser Test","type":"browser","status":"live"}`) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/api/v1/synthetics/tests/browser/ghi-456-jkl/results"):
			fmt.Fprint(w, mockSyntheticsBrowserTestLatestResultsResponse) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root, buf := buildSyntheticsResultsCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "results", "ghi-456-jkl"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"RESULT_ID", "LOCATION", "STATUS", "TIMESTAMP", "result-browser-001", "aws:us-west-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsResults_APITestSingleResult(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/synthetics/tests/abc-123-def":
			fmt.Fprint(w, `{"public_id":"abc-123-def","name":"Homepage API Test","type":"api","status":"live"}`) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/api/v1/synthetics/tests/abc-123-def/results/result-api-001"):
			fmt.Fprint(w, mockSyntheticsAPITestResultFullResponse) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root, buf := buildSyntheticsResultsCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "results", "abc-123-def", "--result-id", "result-api-001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"result-api-001", "aws:us-east-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsResults_BrowserTestSingleResult(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/synthetics/tests/ghi-456-jkl":
			fmt.Fprint(w, `{"public_id":"ghi-456-jkl","name":"Login Browser Test","type":"browser","status":"live"}`) //nolint:errcheck
		case strings.Contains(r.URL.Path, "/api/v1/synthetics/tests/browser/ghi-456-jkl/results/result-browser-001"):
			fmt.Fprint(w, mockSyntheticsBrowserTestResultFullResponse) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	root, buf := buildSyntheticsResultsCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "results", "ghi-456-jkl", "--result-id", "result-browser-001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"result-browser-001", "aws:us-west-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsResults_MissingPublicID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsResultsCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "results"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing public-id, got nil")
	}
}
