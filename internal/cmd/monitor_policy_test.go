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

func newTestPoliciesAPI(srv *httptest.Server) func() (*policiesAPI, error) {
	return func() (*policiesAPI, error) {
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
		return &policiesAPI{api: datadogV2.NewMonitorsApi(c), ctx: apiCtx}, nil
	}
}

func TestNewTestPoliciesAPI(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	mkAPI := newTestPoliciesAPI(srv)
	api, err := mkAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if api == nil {
		t.Fatal("expected non-nil policiesAPI")
	}
	if api.api == nil {
		t.Fatal("expected non-nil datadogV2.MonitorsApi")
	}
	if api.ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

const mockPolicyListResponse = `{
	"data": [
		{
			"id": "abc-123",
			"type": "monitor-config-policy",
			"attributes": {
				"policy_type": "tag",
				"policy": {
					"tag_key": "env",
					"tag_key_required": true,
					"valid_tag_values": ["prod", "staging"]
				}
			}
		},
		{
			"id": "def-456",
			"type": "monitor-config-policy",
			"attributes": {
				"policy_type": "tag",
				"policy": {
					"tag_key": "service",
					"tag_key_required": false,
					"valid_tag_values": ["web", "api"]
				}
			}
		}
	]
}`

const mockPolicyShowResponse = `{
	"data": {
		"id": "abc-123",
		"type": "monitor-config-policy",
		"attributes": {
			"policy_type": "tag",
			"policy": {
				"tag_key": "env",
				"tag_key_required": true,
				"valid_tag_values": ["prod", "staging"]
			}
		}
	}
}`

func buildPolicyCmd(mkAPI func() (*policiesAPI, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	monitors := &cobra.Command{Use: "monitors"}
	monitors.AddCommand(newPolicyCmd(mkAPI))
	root.AddCommand(monitors)
	return root, buf
}

func TestPolicyList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPolicyListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{"monitors", "policy", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "POLICY_TYPE", "TAG_KEY", "abc-123", "env", "tag", "prod", "staging", "def-456", "service"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestPolicyList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPolicyListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "policy", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "abc-123", "env", "tag"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestPolicyShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "abc-123") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPolicyShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{"monitors", "policy", "show", "--id", "abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc-123", "tag", "env", "true", "prod", "staging"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestPolicyShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPolicyShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{"--json", "monitors", "policy", "show", "--id", "abc-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "abc-123", "env", "tag"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestPolicyShow_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"monitors", "policy", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

const mockPolicyCreateResponse = `{
	"data": {
		"id": "new-policy-id",
		"type": "monitor-config-policy",
		"attributes": {
			"policy_type": "tag",
			"policy": {
				"tag_key": "env",
				"tag_key_required": true,
				"valid_tag_values": ["prod", "staging"]
			}
		}
	}
}`

func TestPolicyCreate_RequestBody(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockPolicyCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{
		"monitors", "policy", "create",
		"--tag-key", "env",
		"--tag-key-required",
		"--valid-values", "prod",
		"--valid-values", "staging",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	for _, want := range []string{"env", "prod", "staging"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "new-policy-id") {
		t.Errorf("output missing policy ID\nfull output:\n%s", out)
	}
}

func TestPolicyCreate_MissingTagKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"monitors", "policy", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --tag-key is missing")
	}
}

const mockPolicyUpdateResponse = `{
	"data": {
		"id": "abc-123",
		"type": "monitor-config-policy",
		"attributes": {
			"policy_type": "tag",
			"policy": {
				"tag_key": "env",
				"tag_key_required": true,
				"valid_tag_values": ["prod", "staging", "dev"]
			}
		}
	}
}`

func TestPolicyUpdate_RequestBody(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockPolicyShowResponse) //nolint:errcheck
			return
		}
		if r.Method == http.MethodPatch {
			capturedBody, _ = io.ReadAll(r.Body)
		}
		fmt.Fprint(w, mockPolicyUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{
		"monitors", "policy", "update",
		"--id", "abc-123",
		"--valid-values", "prod",
		"--valid-values", "staging",
		"--valid-values", "dev",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := string(capturedBody)
	for _, want := range []string{"env", "prod", "staging", "dev"} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q\nbody: %s", want, body)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "abc-123") {
		t.Errorf("output missing policy ID\nfull output:\n%s", out)
	}
}

func TestPolicyUpdate_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"monitors", "policy", "update", "--tag-key", "env"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

func TestPolicyDelete_Success(t *testing.T) {
	t.Parallel()
	var capturedMethod string
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetArgs([]string{"monitors", "policy", "delete", "--id", "abc-123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if !strings.Contains(capturedPath, "abc-123") {
		t.Errorf("path %q does not contain policy ID", capturedPath)
	}

	out := buf.String()
	if !strings.Contains(out, "abc-123") {
		t.Errorf("output missing policy ID\nfull output:\n%s", out)
	}
}

func TestPolicyDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildPolicyCmd(newTestPoliciesAPI(srv))
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"monitors", "policy", "delete", "--id", "abc-123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes is missing")
	}
}
