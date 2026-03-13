package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/spf13/cobra"
)

func newTestSLOsAPI(srv *httptest.Server) func() (*slosAPI, error) {
	return func() (*slosAPI, error) {
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
		return &slosAPI{
			api:         datadogV1.NewServiceLevelObjectivesApi(c),
			corrections: datadogV1.NewServiceLevelObjectiveCorrectionsApi(c),
			ctx:         apiCtx,
		}, nil
	}
}

func buildSLOsSubCmd(sub *cobra.Command) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	slos := &cobra.Command{Use: "slos"}
	slos.AddCommand(sub)
	root.AddCommand(slos)
	return root, buf
}

func buildSLOsListCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsListCmd(mkAPI))
}

const mockSLOsListResponse = `{
	"data": [
		{
			"id": "abc123",
			"name": "API Availability",
			"type": "metric",
			"thresholds": [{"timeframe": "30d", "target": 99.9}],
			"tags": ["env:prod", "service:api"]
		},
		{
			"id": "def456",
			"name": "Login Success",
			"type": "monitor",
			"thresholds": [{"timeframe": "7d", "target": 99.0}],
			"tags": ["env:prod"]
		}
	]
}`

func TestSLOsList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "TYPE", "TARGET", "TIMEFRAME", "abc123", "API Availability", "metric", "99.9", "30d", "def456", "Login Success", "monitor", "99", "7d"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"id"`) {
		t.Errorf("JSON output missing id field\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "API Availability") {
		t.Errorf("JSON output missing SLO name\nfull output:\n%s", out)
	}
}

func TestSLOsList_WithQueryFilter(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list", "--query", "env:prod"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotQuery != "env:prod" {
		t.Errorf("query param = %q, want %q", gotQuery, "env:prod")
	}
}

func TestSLOsList_WithTagsFilter(t *testing.T) {
	t.Parallel()
	var gotTags string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTags = r.URL.Query().Get("tags_query")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list", "--tags", "service:web"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotTags != "service:web" {
		t.Errorf("tags_query param = %q, want %q", gotTags, "service:web")
	}
}

func TestSLOsList_Empty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsListCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Errorf("expected headers in empty output, got:\n%s", out)
	}
}

func buildSLOsShowCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsShowCmd(mkAPI))
}

const mockSLOShowResponse = `{
	"data": {
		"id": "abc123",
		"name": "API Availability",
		"type": "metric",
		"description": "Tracks API uptime",
		"thresholds": [
			{"timeframe": "30d", "target": 99.9, "target_display": "99.9"},
			{"timeframe": "7d", "target": 99.5, "target_display": "99.5", "warning": 99.7}
		],
		"tags": ["env:prod", "service:api"],
		"created_at": 1700000000,
		"modified_at": 1700001000
	}
}`

func TestSLOsShow_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsShowCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "show", "--id", "abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"abc123", "API Availability", "metric", "Tracks API uptime", "30d", "99.9", "7d", "99.5", "env:prod"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsShowCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "show", "--id", "abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "API Availability", "metric"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsShow_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsShowCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "show"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func buildSLOsHistoryCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsHistoryCmd(mkAPI))
}

const mockSLOHistoryResponse = `{
	"data": {
		"from_ts": 1700000000,
		"to_ts": 1700604800,
		"type": "metric",
		"overall": {
			"sli_value": 99.75,
			"error_budget_remaining": {"30d": 75.0}
		},
		"thresholds": {
			"30d": {"timeframe": "30d", "target": 99.9}
		}
	}
}`

func TestSLOsHistory_TableOutput(t *testing.T) {
	t.Parallel()
	var gotSLOID string
	var gotFrom, gotTo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path: /api/v1/slo/{slo_id}/history
		parts := strings.Split(r.URL.Path, "/")
		for i, p := range parts {
			if p == "slo" && i+2 < len(parts) {
				gotSLOID = parts[i+1]
			}
		}
		gotFrom = r.URL.Query().Get("from_ts")
		gotTo = r.URL.Query().Get("to_ts")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOHistoryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsHistoryCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "history", "--id", "abc123", "--from", "1700000000", "--to", "1700604800"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotSLOID != "abc123" {
		t.Errorf("SLO ID = %q, want %q", gotSLOID, "abc123")
	}
	if gotFrom != "1700000000" {
		t.Errorf("from_ts = %q, want %q", gotFrom, "1700000000")
	}
	if gotTo != "1700604800" {
		t.Errorf("to_ts = %q, want %q", gotTo, "1700604800")
	}

	out := buf.String()
	for _, want := range []string{"SLI", "99.75", "ERROR BUDGET", "30d", "75"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsHistory_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOHistoryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsHistoryCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "history", "--id", "abc123", "--from", "1700000000", "--to", "1700604800"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"sli_value"`, "99.75", `"type"`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsHistory_RelativeTime(t *testing.T) {
	t.Parallel()
	var gotFrom, gotTo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFrom = r.URL.Query().Get("from_ts")
		gotTo = r.URL.Query().Get("to_ts")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOHistoryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsHistoryCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "history", "--id", "abc123", "--from", "now-7d", "--to", "now"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotFrom == "" {
		t.Error("from_ts was not sent")
	}
	if gotTo == "" {
		t.Error("to_ts was not sent")
	}
	// from should be roughly 7 days ago (within 60s tolerance)
	fromUnix, err := strconv.ParseInt(gotFrom, 10, 64)
	if err != nil {
		t.Fatalf("from_ts not a unix timestamp: %v", err)
	}
	expected := time.Now().Add(-7 * 24 * time.Hour).Unix()
	diff := fromUnix - expected
	if diff < -60 || diff > 60 {
		t.Errorf("from_ts %d not within 60s of expected %d", fromUnix, expected)
	}
}

func TestSLOsHistory_MissingFlags(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing id", []string{"slos", "history", "--from", "1700000000", "--to", "1700604800"}},
		{"missing from", []string{"slos", "history", "--id", "abc123", "--to", "1700604800"}},
		{"missing to", []string{"slos", "history", "--id", "abc123", "--from", "1700000000"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildSLOsHistoryCmd(newTestSLOsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Fatal("expected error for missing flags")
			}
		})
	}
}

func buildSLOsCreateCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsCreateCmd(mkAPI))
}

const mockSLOCreateResponse = `{
	"data": [
		{
			"id": "new123",
			"name": "API Uptime",
			"type": "metric",
			"thresholds": [{"timeframe": "30d", "target": 99.9}],
			"tags": ["env:prod"]
		}
	]
}`

func TestSLOsCreate_MetricSLO(t *testing.T) {
	t.Parallel()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCreateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"slos", "create",
		"--name", "API Uptime",
		"--type", "metric",
		"--thresholds", `[{"timeframe":"30d","target":99.9}]`,
		"--numerator", "sum:requests.success{*}.as_count()",
		"--denominator", "sum:requests.total{*}.as_count()",
		"--tags", "env:prod",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotBody["name"] != "API Uptime" {
		t.Errorf("name = %v, want API Uptime", gotBody["name"])
	}
	if gotBody["type"] != "metric" {
		t.Errorf("type = %v, want metric", gotBody["type"])
	}
	query, ok := gotBody["query"].(map[string]interface{})
	if !ok {
		t.Fatalf("query field missing or wrong type: %v", gotBody["query"])
	}
	if query["numerator"] != "sum:requests.success{*}.as_count()" {
		t.Errorf("numerator = %v", query["numerator"])
	}
	if query["denominator"] != "sum:requests.total{*}.as_count()" {
		t.Errorf("denominator = %v", query["denominator"])
	}

	out := buf.String()
	for _, want := range []string{"new123", "API Uptime", "metric", "99.9", "30d"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsCreate_MonitorSLO(t *testing.T) {
	t.Parallel()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"mon123","name":"Login SLO","type":"monitor","thresholds":[{"timeframe":"7d","target":99.0}],"tags":[]}]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsCreateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"slos", "create",
		"--name", "Login SLO",
		"--type", "monitor",
		"--thresholds", `[{"timeframe":"7d","target":99.0}]`,
		"--monitor-ids", "1234,5678",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotBody["type"] != "monitor" {
		t.Errorf("type = %v, want monitor", gotBody["type"])
	}
	ids, ok := gotBody["monitor_ids"].([]interface{})
	if !ok {
		t.Fatalf("monitor_ids missing or wrong type: %v", gotBody["monitor_ids"])
	}
	if len(ids) != 2 {
		t.Errorf("monitor_ids len = %d, want 2", len(ids))
	}
	// verify specific values (JSON numbers are float64)
	if ids[0] != float64(1234) {
		t.Errorf("monitor_ids[0] = %v, want 1234", ids[0])
	}
	if ids[1] != float64(5678) {
		t.Errorf("monitor_ids[1] = %v, want 5678", ids[1])
	}
}

func TestSLOsCreate_RequiredFlags(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{
			"missing name",
			[]string{"slos", "create", "--type", "metric", "--thresholds", `[{"timeframe":"30d","target":99.9}]`, "--numerator", "n", "--denominator", "d"},
		},
		{
			"missing type",
			[]string{"slos", "create", "--name", "X", "--thresholds", `[{"timeframe":"30d","target":99.9}]`},
		},
		{
			"missing thresholds",
			[]string{"slos", "create", "--name", "X", "--type", "metric"},
		},
		{
			"metric missing numerator/denominator",
			[]string{"slos", "create", "--name", "X", "--type", "metric", "--thresholds", `[{"timeframe":"30d","target":99.9}]`},
		},
		{
			"monitor missing monitor-ids",
			[]string{"slos", "create", "--name", "X", "--type", "monitor", "--thresholds", `[{"timeframe":"7d","target":99.0}]`},
		},
		{
			"empty thresholds array",
			[]string{"slos", "create", "--name", "X", "--type", "metric", "--thresholds", `[]`, "--numerator", "n", "--denominator", "d"},
		},
		{
			"invalid thresholds json",
			[]string{"slos", "create", "--name", "X", "--type", "metric", "--thresholds", `not-json`, "--numerator", "n", "--denominator", "d"},
		},
		{
			"unsupported type",
			[]string{"slos", "create", "--name", "X", "--type", "time_slice", "--thresholds", `[{"timeframe":"30d","target":99.9}]`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildSLOsCreateCmd(newTestSLOsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Fatalf("expected error for %q", tc.name)
			}
		})
	}
}

func TestSLOsCreate_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCreateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"--json", "slos", "create",
		"--name", "API Uptime",
		"--type", "metric",
		"--thresholds", `[{"timeframe":"30d","target":99.9}]`,
		"--numerator", "sum:requests.success{*}.as_count()",
		"--denominator", "sum:requests.total{*}.as_count()",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "new123", "API Uptime"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func buildSLOsUpdateCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsUpdateCmd(mkAPI))
}

const mockSLOGetForUpdate = `{
	"data": {
		"id": "abc123",
		"name": "API Availability",
		"type": "metric",
		"description": "Old description",
		"thresholds": [{"timeframe": "30d", "target": 99.9}],
		"tags": ["env:prod"],
		"query": {"numerator": "sum:requests.success{*}.as_count()", "denominator": "sum:requests.total{*}.as_count()"}
	}
}`

const mockSLOUpdateResponse = `{
	"data": [
		{
			"id": "abc123",
			"name": "API Availability Updated",
			"type": "metric",
			"description": "New description",
			"thresholds": [{"timeframe": "30d", "target": 99.95}],
			"tags": ["env:prod", "team:platform"]
		}
	]
}`

func TestSLOsUpdate_RequestBody(t *testing.T) {
	t.Parallel()
	var requestBodies []map[string]interface{}
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockSLOGetForUpdate) //nolint:errcheck
		} else {
			callCount++
			body, _ := io.ReadAll(r.Body)
			var rb map[string]interface{}
			_ = json.Unmarshal(body, &rb)
			requestBodies = append(requestBodies, rb)
			fmt.Fprint(w, mockSLOUpdateResponse) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"slos", "update",
		"--id", "abc123",
		"--name", "API Availability Updated",
		"--description", "New description",
		"--thresholds", `[{"timeframe":"30d","target":99.95}]`,
		"--tags", "env:prod,team:platform",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if callCount != 1 {
		t.Errorf("UpdateSLO called %d times, want 1", callCount)
	}
	if len(requestBodies) == 0 {
		t.Fatal("no request body captured")
	}
	rb := requestBodies[0]
	if rb["name"] != "API Availability Updated" {
		t.Errorf("name = %v, want %q", rb["name"], "API Availability Updated")
	}
	if rb["description"] != "New description" {
		t.Errorf("description = %v, want %q", rb["description"], "New description")
	}
	ths, ok := rb["thresholds"].([]interface{})
	if !ok || len(ths) == 0 {
		t.Fatalf("thresholds missing or wrong type: %v", rb["thresholds"])
	}
	th0, ok := ths[0].(map[string]interface{})
	if !ok {
		t.Fatalf("thresholds[0] wrong type: %v", ths[0])
	}
	if th0["target"] != float64(99.95) {
		t.Errorf("thresholds[0].target = %v, want 99.95", th0["target"])
	}
	if th0["timeframe"] != "30d" {
		t.Errorf("thresholds[0].timeframe = %v, want 30d", th0["timeframe"])
	}

	out := buf.String()
	for _, want := range []string{"abc123", "API Availability Updated", "99.95"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsUpdate_PreservesUnchangedFields(t *testing.T) {
	t.Parallel()
	var updateBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockSLOGetForUpdate) //nolint:errcheck
		} else {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &updateBody)
			fmt.Fprint(w, mockSLOUpdateResponse) //nolint:errcheck
		}
	}))
	defer srv.Close()

	// only update name, leave everything else unchanged
	root, _ := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"slos", "update",
		"--id", "abc123",
		"--name", "New Name Only",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if updateBody["name"] != "New Name Only" {
		t.Errorf("name = %v, want %q", updateBody["name"], "New Name Only")
	}
	// original description preserved
	if updateBody["description"] != "Old description" {
		t.Errorf("description = %v, want original %q", updateBody["description"], "Old description")
	}
	// original tags preserved
	tags, ok := updateBody["tags"].([]interface{})
	if !ok || len(tags) == 0 {
		t.Errorf("tags not preserved: %v", updateBody["tags"])
	}
}

func TestSLOsUpdate_NumeratorOnMonitorSLO(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"id":"abc123","name":"Monitor SLO","type":"monitor","thresholds":[{"timeframe":"30d","target":99.9}],"monitor_ids":[1]}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "update", "--id", "abc123", "--numerator", "sum:requests.success{*}.as_count()"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when setting numerator on monitor SLO")
	}
}

func TestSLOsUpdate_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "update", "--name", "New Name"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestSLOsUpdate_TagsWithSpacesTrimmed(t *testing.T) {
	t.Parallel()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockSLOGetForUpdate) //nolint:errcheck
		} else {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			fmt.Fprint(w, mockSLOUpdateResponse) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, _ := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "update", "--id", "abc123", "--tags", "env:prod, service:api"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	tags, ok := gotBody["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Fatalf("tags = %v, want 2 elements", gotBody["tags"])
	}
	if tags[1] != "service:api" {
		t.Errorf("tags[1] = %q, want %q (space should be trimmed)", tags[1], "service:api")
	}
}

func TestSLOsUpdate_ClearTags(t *testing.T) {
	t.Parallel()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockSLOGetForUpdate) //nolint:errcheck
		} else {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			fmt.Fprint(w, mockSLOUpdateResponse) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, _ := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "update", "--id", "abc123", "--tags", ""})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	tags, ok := gotBody["tags"].([]interface{})
	if !ok {
		t.Fatalf("tags field missing or wrong type: %v", gotBody["tags"])
	}
	if len(tags) != 0 {
		t.Errorf("tags = %v, want empty slice when --tags \"\" is passed", tags)
	}
}

func TestSLOsUpdate_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, mockSLOGetForUpdate) //nolint:errcheck
		} else {
			fmt.Fprint(w, mockSLOUpdateResponse) //nolint:errcheck
		}
	}))
	defer srv.Close()

	root, buf := buildSLOsUpdateCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "update", "--id", "abc123", "--name", "Updated"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "abc123"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func buildSLOsDeleteCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsDeleteCmd(mkAPI))
}

func buildSLOsCanDeleteCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsCanDeleteCmd(mkAPI))
}

func TestSLOsDelete_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":["abc123"],"errors":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "delete", "--id", "abc123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "abc123") {
		t.Errorf("output missing deleted ID\nfull output:\n%s", buf.String())
	}
}

func TestSLOsDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "delete", "--id", "abc123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes not provided")
	}
}

func TestSLOsDelete_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "delete", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id not provided")
	}
}

func TestSLOsCanDelete_TableOutput(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("ids")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"ok":["abc123"]},"errors":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCanDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "can-delete", "--id", "abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotQuery != "abc123" {
		t.Errorf("ids query param = %q, want %q", gotQuery, "abc123")
	}
	out := buf.String()
	for _, want := range []string{"abc123", "can delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsCanDelete_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsCanDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "can-delete"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id not provided")
	}
}

func TestSLOsCanDelete_BlockedSLOs(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"ok":[]},"errors":{"abc123":"used by monitor 42"}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCanDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "can-delete", "--id", "abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"abc123", "blocked"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOsCanDelete_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"ok":["abc123"]},"errors":{}}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCanDeleteCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "can-delete", "--id", "abc123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`"data"`, "abc123"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func buildSLOsCorrectionCmd(mkAPI func() (*slosAPI, error)) (*cobra.Command, *bytes.Buffer) {
	return buildSLOsSubCmd(newSLOsCorrectionCmd(mkAPI))
}

const mockSLOCorrectionListResponse = `{
	"data": [
		{
			"id": "corr123",
			"type": "correction",
			"attributes": {
				"slo_id": "abc123",
				"category": "Scheduled Maintenance",
				"description": "weekly maintenance",
				"start": 1700000000,
				"end": 1700003600,
				"timezone": "UTC"
			}
		},
		{
			"id": "corr456",
			"type": "correction",
			"attributes": {
				"slo_id": "def456",
				"category": "Deployment",
				"start": 1700100000,
				"end": 1700103600,
				"timezone": "UTC"
			}
		}
	]
}`

const mockSLOCorrectionShowResponse = `{
	"data": {
		"id": "corr123",
		"type": "correction",
		"attributes": {
			"slo_id": "abc123",
			"category": "Scheduled Maintenance",
			"description": "weekly maintenance",
			"start": 1700000000,
			"end": 1700003600,
			"timezone": "UTC"
		}
	}
}`

func TestSLOCorrectionList_TableOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "SLO_ID", "CATEGORY", "START", "END", "corr123", "abc123", "Scheduled Maintenance", "corr456", "def456", "Deployment"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionList_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "correction", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "corr123", "Scheduled Maintenance"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionShow_TableOutput(t *testing.T) {
	t.Parallel()
	var gotCorrectionID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path: /api/v1/slo/corrections/{id}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 0 {
			gotCorrectionID = parts[len(parts)-1]
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "show", "--id", "corr123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotCorrectionID != "corr123" {
		t.Errorf("correction ID = %q, want %q", gotCorrectionID, "corr123")
	}

	out := buf.String()
	for _, want := range []string{"corr123", "abc123", "Scheduled Maintenance", "weekly maintenance"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionShow_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "correction", "show", "--id", "corr123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "corr123", "Scheduled Maintenance"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionShow_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

const mockSLOCorrectionCreateResponse = `{
	"data": {
		"id": "corrNew",
		"type": "correction",
		"attributes": {
			"slo_id": "abc123",
			"category": "Deployment",
			"description": "deploy window",
			"start": 1700000000,
			"end": 1700003600,
			"timezone": "UTC"
		}
	}
}`

func TestSLOCorrectionCreate_RequestBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"slos", "correction", "create",
		"--slo-id", "abc123",
		"--category", "Deployment",
		"--start", "1700000000",
		"--end", "1700003600",
		"--description", "deploy window",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, ok := gotBody["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data field missing: %v", gotBody)
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("attributes field missing: %v", data)
	}
	if attrs["slo_id"] != "abc123" {
		t.Errorf("slo_id = %v, want abc123", attrs["slo_id"])
	}
	if attrs["category"] != "Deployment" {
		t.Errorf("category = %v, want Deployment", attrs["category"])
	}
	if attrs["description"] != "deploy window" {
		t.Errorf("description = %v, want deploy window", attrs["description"])
	}

	out := buf.String()
	for _, want := range []string{"corrNew", "abc123", "Deployment", "deploy window"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionCreate_RequiredFlags(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tests := []struct {
		name string
		args []string
	}{
		{"missing slo-id", []string{"slos", "correction", "create", "--category", "Deployment", "--start", "1700000000"}},
		{"missing category", []string{"slos", "correction", "create", "--slo-id", "abc123", "--start", "1700000000"}},
		{"missing start", []string{"slos", "correction", "create", "--slo-id", "abc123", "--category", "Deployment"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
			root.SetArgs(tc.args)
			if err := root.Execute(); err == nil {
				t.Fatalf("expected error for %q", tc.name)
			}
		})
	}
}

func TestSLOCorrectionCreate_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"--json", "slos", "correction", "create",
		"--slo-id", "abc123",
		"--category", "Deployment",
		"--start", "1700000000",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{`"id"`, "corrNew", "Deployment"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

const mockSLOCorrectionUpdateResponse = `{
	"data": {
		"id": "corr123",
		"type": "correction",
		"attributes": {
			"slo_id": "abc123",
			"category": "Deployment",
			"description": "updated desc",
			"start": 1700000000,
			"end": 1700007200,
			"timezone": "UTC"
		}
	}
}`

func TestSLOCorrectionUpdate_RequestBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{
		"slos", "correction", "update",
		"--id", "corr123",
		"--category", "Deployment",
		"--description", "updated desc",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, ok := gotBody["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data field missing: %v", gotBody)
	}
	attrs, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("attributes field missing: %v", data)
	}
	if attrs["category"] != "Deployment" {
		t.Errorf("category = %v, want Deployment", attrs["category"])
	}
	if attrs["description"] != "updated desc" {
		t.Errorf("description = %v, want updated desc", attrs["description"])
	}

	out := buf.String()
	for _, want := range []string{"corr123", "Deployment", "updated desc"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionUpdate_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "update", "--category", "Deployment"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestSLOCorrectionUpdate_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockSLOCorrectionUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"--json", "slos", "correction", "update", "--id", "corr123", "--category", "Deployment"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`"id"`, "corr123"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestSLOCorrectionDelete_Success(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "delete", "--id", "corr123", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(gotPath, "corr123") {
		t.Errorf("path %q does not contain correction ID", gotPath)
	}
	if !strings.Contains(buf.String(), "corr123") {
		t.Errorf("output missing deleted correction ID\nfull output:\n%s", buf.String())
	}
}

func TestSLOCorrectionDelete_MissingYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "delete", "--id", "corr123"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes not provided")
	}
}

func TestSLOCorrectionDelete_MissingID(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil)
	defer srv.Close()

	root, _ := buildSLOsCorrectionCmd(newTestSLOsAPI(srv))
	root.SetArgs([]string{"slos", "correction", "delete", "--yes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --id not provided")
	}
}

func TestNewSLOsCommand_Subcommands(t *testing.T) {
	t.Parallel()
	cmd := NewSLOsCommand()
	if cmd.Use != "slos" {
		t.Errorf("Use = %q, want %q", cmd.Use, "slos")
	}

	want := []string{"list", "show", "history", "create", "update", "delete", "can-delete", "correction"}
	found := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, name := range want {
		if !found[name] {
			t.Errorf("subcommand %q not found", name)
		}
	}
}
