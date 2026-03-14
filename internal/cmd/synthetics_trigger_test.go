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

func buildSyntheticsTriggerCmd(mkAPI func() (*syntheticsAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	syn := &cobra.Command{Use: "synthetics"}
	syn.AddCommand(newSyntheticsTriggerCmd(mkAPI))
	syn.AddCommand(newSyntheticsBatchCmd(mkAPI))
	root.AddCommand(syn)
	return root, buf
}

const mockSyntheticsTriggerResponse = `{
	"batch_id": "batch-abc-001",
	"triggered_check_ids": ["abc-123-def", "ghi-456-jkl"],
	"results": [
		{
			"public_id": "abc-123-def",
			"result_id": "result-trigger-001",
			"location": 1
		},
		{
			"public_id": "ghi-456-jkl",
			"result_id": "result-trigger-002",
			"location": 2
		}
	]
}`

const mockSyntheticsBatchResponse = `{
	"data": {
		"status": "passed",
		"results": [
			{
				"test_public_id": "abc-123-def",
				"test_name": "Homepage API Test",
				"result_id": "result-trigger-001",
				"location": "aws:us-east-1",
				"status": "passed",
				"duration": 125.4
			},
			{
				"test_public_id": "ghi-456-jkl",
				"test_name": "Login Browser Test",
				"result_id": "result-trigger-002",
				"location": "aws:eu-west-1",
				"status": "failed",
				"duration": 3200.5
			}
		]
	}
}`

func TestSyntheticsTrigger_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/synthetics/tests/trigger" && r.Method == http.MethodPost {
			fmt.Fprint(w, mockSyntheticsTriggerResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsTriggerCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "trigger", "--id", "abc-123-def,ghi-456-jkl"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"PUBLIC_ID", "RESULT_ID", "BATCH_ID", "result-trigger-001", "result-trigger-002", "abc-123-def", "ghi-456-jkl"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsTrigger_MissingIDFlag(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsTriggerCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "trigger"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --id, got nil")
	}
	if !strings.Contains(err.Error(), "--id") {
		t.Errorf("error should mention --id, got: %v", err)
	}
}

func TestSyntheticsBatch_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/v1/synthetics/ci/batch/batch-abc-001") {
			fmt.Fprint(w, mockSyntheticsBatchResponse) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	root, buf := buildSyntheticsTriggerCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "batch", "batch-abc-001"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TEST_PUBLIC_ID", "RESULT_ID", "STATUS", "abc-123-def", "ghi-456-jkl", "result-trigger-001", "passed", "failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSyntheticsBatch_MissingArg(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	root, _ := buildSyntheticsTriggerCmd(newTestSyntheticsAPI(srv))
	root.SetArgs([]string{"synthetics", "batch"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing batch-id arg, got nil")
	}
}
