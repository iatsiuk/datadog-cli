package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/spf13/cobra"
)

func newTestMetricsV1API(srv *httptest.Server) func() (*metricsV1API, error) {
	return func() (*metricsV1API, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		ctx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &metricsV1API{api: datadogV1.NewMetricsApi(c), ctx: ctx}, nil
	}
}

func buildMetricsQueryCmd(mkAPI func() (*metricsV1API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsQueryCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsQueryResponse = `{
	"status": "ok",
	"res_type": "time_series",
	"series": [
		{
			"metric": "system.cpu.user",
			"scope": "host:web-01",
			"pointlist": [[1700000000000, 42.5], [1700000060000, 43.2]]
		}
	],
	"from_date": 1700000000000,
	"to_date": 1700000060000,
	"query": "avg:system.cpu.user{*}"
}`

func TestMetricsQueryFlagsRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsQueryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsQueryCmd(newTestMetricsV1API(srv))
	// missing --query should fail
	root.SetArgs([]string{"metrics", "query", "--from", "1700000000", "--to", "1700000060"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --query is missing")
	}
}

func TestMetricsQueryFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsQueryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsQueryCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "query",
		"--query", "avg:system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedURL, "avg%3Asystem.cpu.user") && !strings.Contains(capturedURL, "system.cpu.user") {
		t.Errorf("query param not found in URL: %s", capturedURL)
	}
	if !strings.Contains(capturedURL, "1700000000") {
		t.Errorf("from param not found in URL: %s", capturedURL)
	}
}

func TestMetricsQueryTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsQueryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsQueryCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "query",
		"--query", "avg:system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "VALUE", "42.5", "43.2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsQueryJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsQueryResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsQueryCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "query",
		"--query", "avg:system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("got %d series, want 1", len(result))
	}
}

func buildMetricsSearchCmd(mkAPI func() (*metricsV1API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsSearchCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

func buildMetricsListCmd(mkAPI func() (*metricsV1API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsListCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsSearchResponse = `{
	"results": {
		"metrics": ["system.cpu.user", "system.cpu.idle", "system.cpu.iowait"]
	}
}`

const mockMetricsListResponse = `{
	"from": "1700000000",
	"metrics": ["custom.metric.one", "custom.metric.two"]
}`

func TestMetricsSearchQueryRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSearchCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "search"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --query is missing")
	}
}

func TestMetricsSearchTableOutput(t *testing.T) {
	t.Parallel()

	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsSearchCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "search", "--query", "system.cpu"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedURL, "system.cpu") {
		t.Errorf("query not found in URL: %s", capturedURL)
	}

	out := buf.String()
	for _, want := range []string{"METRIC", "system.cpu.user", "system.cpu.idle"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsSearchJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSearchResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsSearchCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "search", "--query", "system.cpu", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 3 {
		t.Errorf("got %d metrics, want 3", len(result))
	}
}

func TestMetricsListFromRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsListCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "list"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --from is missing")
	}
}

func TestMetricsListTableOutput(t *testing.T) {
	t.Parallel()

	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsListCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "list", "--from", "1700000000"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedURL, "1700000000") {
		t.Errorf("from param not found in URL: %s", capturedURL)
	}

	out := buf.String()
	for _, want := range []string{"METRIC", "custom.metric.one", "custom.metric.two"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsListCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "list", "--from", "1700000000", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 2 {
		t.Errorf("got %d metrics, want 2", len(result))
	}
}

func TestMetricsListRelativeTime(t *testing.T) {
	t.Parallel()

	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"metrics":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsListCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "list", "--from", "now-1h"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	parsed, err := url.ParseRequestURI(capturedURL)
	if err != nil {
		t.Fatalf("invalid captured URL %q: %v", capturedURL, err)
	}
	fromVal := parsed.Query().Get("from")
	if fromVal == "" {
		t.Errorf("from param not found in URL: %s", capturedURL)
	}
	if _, err := strconv.ParseInt(fromVal, 10, 64); err != nil {
		t.Errorf("from param %q is not a numeric unix timestamp: %v", fromVal, err)
	}
}

func newTestMetricsV2API(srv *httptest.Server) func() (*metricsV2API, error) {
	return func() (*metricsV2API, error) {
		cfg := datadog.NewConfiguration()
		cfg.Servers = datadog.ServerConfigurations{{URL: srv.URL}}
		cfg.Debug = false
		c := datadog.NewAPIClient(cfg)
		ctx := context.WithValue(
			context.Background(),
			datadog.ContextAPIKeys,
			map[string]datadog.APIKey{
				"apiKeyAuth": {Key: "test"},
				"appKeyAuth": {Key: "test"},
			},
		)
		return &metricsV2API{api: datadogV2.NewMetricsApi(c), ctx: ctx}, nil
	}
}

func buildMetricsScalarCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsScalarCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsScalarResponse = `{
	"data": {
		"type": "scalar_response",
		"attributes": {
			"columns": [
				{
					"name": "query1",
					"type": "number",
					"values": [42.5]
				}
			]
		}
	}
}`

func TestMetricsScalarFlagsRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsScalarResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsScalarCmd(newTestMetricsV2API(srv))
	// missing --query should fail
	root.SetArgs([]string{"metrics", "scalar", "--from", "1700000000", "--to", "1700000060"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --query is missing")
	}
}

func TestMetricsScalarFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsScalarResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsScalarCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "scalar",
		"--query", "system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "system.cpu.user") {
		t.Errorf("query not found in request body: %s", capturedBody)
	}
}

func TestMetricsScalarTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsScalarResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsScalarCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "scalar",
		"--query", "system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"NAME", "VALUE", "query1", "42.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsScalarJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsScalarResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsScalarCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "scalar",
		"--query", "system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["data"] == nil {
		t.Errorf("JSON output missing 'data' key:\n%s", buf.String())
	}
}

const mockMetricsScalarGroupByResponse = `{
	"data": {
		"type": "scalar_response",
		"attributes": {
			"columns": [
				{
					"name": "service",
					"type": "group",
					"values": [["web"], ["api"]]
				},
				{
					"name": "",
					"type": "number",
					"values": [42.5, 13.7]
				}
			]
		}
	}
}`

const mockMetricsScalarMultiGroupByResponse = `{
	"data": {
		"type": "scalar_response",
		"attributes": {
			"columns": [
				{
					"name": "service",
					"type": "group",
					"values": [["web"], ["api"]]
				},
				{
					"name": "env",
					"type": "group",
					"values": [["prod"], ["prod"]]
				},
				{
					"name": "",
					"type": "number",
					"values": [42.5, 13.7]
				}
			]
		}
	}
}`

func TestMetricsScalarGroupByTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsScalarGroupByResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsScalarCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "scalar",
		"--query", "avg:system.cpu.user{*} by {service}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"SERVICE", "web", "api", "42.5", "13.7"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsScalarMultiGroupByTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsScalarMultiGroupByResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsScalarCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "scalar",
		"--query", "avg:system.cpu.user{*} by {service,env}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"SERVICE", "ENV", "web", "api", "prod", "42.5", "13.7"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func buildMetricsTimeseriesCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsTimeseriesCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsTimeseriesResponse = `{
	"data": {
		"type": "timeseries_response",
		"attributes": {
			"series": [{"query_index": 0, "unit": null}],
			"times": [1700000000000, 1700000060000],
			"values": [[42.5, 43.2]]
		}
	}
}`

func TestMetricsTimeseriesFlagsRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTimeseriesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTimeseriesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "timeseries", "--from", "1700000000", "--to", "1700000060"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --query is missing")
	}
}

func TestMetricsTimeseriesFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTimeseriesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTimeseriesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "timeseries",
		"--query", "avg:system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "system.cpu.user") {
		t.Errorf("query not found in request body: %s", capturedBody)
	}
}

func TestMetricsTimeseriesTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTimeseriesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTimeseriesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "timeseries",
		"--query", "avg:system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"TIMESTAMP", "VALUE", "42.5", "43.2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsTimeseriesJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTimeseriesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTimeseriesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "timeseries",
		"--query", "avg:system.cpu.user{*}",
		"--from", "1700000000",
		"--to", "1700000060",
		"--json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["data"] == nil {
		t.Errorf("JSON output missing 'data' key:\n%s", buf.String())
	}
}

func buildMetricsSubmitCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsSubmitCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsSubmitResponse = `{"errors": []}`

func TestMetricsSubmitFlagsRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit", "--type", "gauge", "--points", "1700000000:42.0"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --metric is missing")
	}
}

func TestMetricsSubmitPointsRequired(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit", "--metric", "custom.test.metric", "--type", "gauge"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --points is missing")
	}
}

func TestMetricsSubmitFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.test.metric",
		"--type", "gauge",
		"--points", "1700000000:42.0",
		"--tags", "env:prod",
		"--tags", "host:web-01",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "custom.test.metric") {
		t.Errorf("metric name not found in request body: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "env:prod") {
		t.Errorf("tag not found in request body: %s", capturedBody)
	}
}

func TestMetricsSubmitGaugeType(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.gauge",
		"--type", "gauge",
		"--points", "1700000000:99.5",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// gauge type = 3 in the API
	if !strings.Contains(capturedBody, `"type":3`) {
		t.Errorf("gauge type (3) not found in body: %s", capturedBody)
	}
}

func TestMetricsSubmitCountType(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.count",
		"--type", "count",
		"--points", "1700000000:5",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// count type = 1 in the API
	if !strings.Contains(capturedBody, `"type":1`) {
		t.Errorf("count type (1) not found in body: %s", capturedBody)
	}
}

func TestMetricsSubmitRateType(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.rate",
		"--type", "rate",
		"--points", "1700000000:1.5",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// rate type = 2 in the API
	if !strings.Contains(capturedBody, `"type":2`) {
		t.Errorf("rate type (2) not found in body: %s", capturedBody)
	}
}

func TestMetricsSubmitInvalidType(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.metric",
		"--type", "invalid",
		"--points", "1700000000:42.0",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestMetricsSubmitInvalidPoints(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.metric",
		"--type", "gauge",
		"--points", "badformat",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid points format")
	}
}

func TestMetricsSubmitMultiplePoints(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsSubmitResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsSubmitCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "submit",
		"--metric", "custom.metric",
		"--type", "gauge",
		"--points", "1700000000:10.0",
		"--points", "1700000060:20.0",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "1700000060") {
		t.Errorf("second point timestamp not found in body: %s", capturedBody)
	}
}

func buildMetricsMetadataCmd(mkAPI func() (*metricsV1API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsMetadataCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsMetadataResponse = `{
	"type": "gauge",
	"description": "Average CPU usage",
	"unit": "percent",
	"per_unit": "second",
	"short_name": "cpu"
}`

func TestMetricsMetadataShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsMetadataResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsMetadataCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "metadata", "show", "system.cpu.user"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"FIELD", "VALUE", "gauge", "percent", "Average CPU usage"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsMetadataShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsMetadataResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsMetadataCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "metadata", "show", "system.cpu.user", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
}

func TestMetricsMetadataShowRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsMetadataResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsMetadataCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "metadata", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsMetadataUpdateFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsMetadataResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsMetadataCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "metadata", "update", "system.cpu.user",
		"--type", "gauge",
		"--description", "CPU usage",
		"--unit", "percent",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "gauge") {
		t.Errorf("type not found in request body: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "CPU usage") {
		t.Errorf("description not found in request body: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "percent") {
		t.Errorf("unit not found in request body: %s", capturedBody)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("output missing 'updated':\n%s", out)
	}
}

func TestMetricsMetadataUpdateRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsMetadataResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsMetadataCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "metadata", "update"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsQueryRelativeTime(t *testing.T) {
	t.Parallel()

	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","series":[]}`) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsQueryCmd(newTestMetricsV1API(srv))
	root.SetArgs([]string{"metrics", "query",
		"--query", "avg:system.cpu.user{*}",
		"--from", "now-1h",
		"--to", "now",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedURL == "" {
		t.Fatal("no request made")
	}
	// relative times should resolve to numeric unix timestamps in the URL
	parsed, err := url.ParseRequestURI(capturedURL)
	if err != nil {
		t.Fatalf("invalid captured URL %q: %v", capturedURL, err)
	}
	fromVal := parsed.Query().Get("from")
	if fromVal == "" {
		t.Errorf("from param not found in URL: %s", capturedURL)
	}
	if _, err := strconv.ParseInt(fromVal, 10, 64); err != nil {
		t.Errorf("from param %q is not a numeric unix timestamp: %v", fromVal, err)
	}
}

func buildMetricsTagConfigCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsTagConfigCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockTagConfigListResponse = `{
	"data": [
		{
			"type": "manage_tags",
			"id": "system.cpu.user",
			"attributes": {
				"metric_type": "gauge",
				"tags": ["env", "host"]
			}
		}
	]
}`

const mockTagConfigShowResponse = `{
	"data": {
		"type": "manage_tags",
		"id": "system.cpu.user",
		"attributes": {
			"metric_type": "gauge",
			"tags": ["env", "host"]
		}
	}
}`

const mockTagConfigCreateResponse = `{
	"data": {
		"type": "manage_tags",
		"id": "custom.metric",
		"attributes": {
			"metric_type": "gauge",
			"tags": ["env"]
		}
	}
}`

const mockTagConfigUpdateResponse = `{
	"data": {
		"type": "manage_tags",
		"id": "custom.metric",
		"attributes": {
			"metric_type": "gauge",
			"tags": ["env", "region"]
		}
	}
}`

func TestMetricsTagConfigListTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"METRIC", "TYPE", "TAGS", "system.cpu.user", "gauge", "env"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsTagConfigListJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigListResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(result) != 1 {
		t.Errorf("got %d items, want 1", len(result))
	}
}

func TestMetricsTagConfigShowTableOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "show", "system.cpu.user"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"FIELD", "VALUE", "gauge", "env", "host"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsTagConfigShowRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "show"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsTagConfigShowJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigShowResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "show", "system.cpu.user", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
}

func TestMetricsTagConfigCreateFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "create", "custom.metric",
		"--tags", "env",
		"--tags", "host",
		"--metric-type", "gauge",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "custom.metric") {
		t.Errorf("metric name not found in request body: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "env") {
		t.Errorf("tag not found in request body: %s", capturedBody)
	}
	if !strings.Contains(buf.String(), "created") {
		t.Errorf("output missing 'created': %s", buf.String())
	}
}

func TestMetricsTagConfigCreateRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsTagConfigCreateRequiresTags(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigCreateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "create", "custom.metric"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --tags is missing")
	}
}

func TestMetricsTagConfigUpdateFlagsParsed(t *testing.T) {
	t.Parallel()

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := &bytes.Buffer{}
		_, _ = buf.ReadFrom(r.Body)
		capturedBody = buf.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "update", "custom.metric",
		"--tags", "env",
		"--tags", "region",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedBody, "custom.metric") {
		t.Errorf("metric name not found in request body: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "region") {
		t.Errorf("tag not found in request body: %s", capturedBody)
	}
	if !strings.Contains(buf.String(), "updated") {
		t.Errorf("output missing 'updated': %s", buf.String())
	}
}

func TestMetricsTagConfigUpdateRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTagConfigUpdateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "update"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsTagConfigDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "delete", "custom.metric"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --yes is missing")
	}
}

func TestMetricsTagConfigDeleteWithYes(t *testing.T) {
	t.Parallel()

	var capturedMethod string
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, buf := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "delete", "custom.metric", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if !strings.Contains(capturedPath, "custom.metric") {
		t.Errorf("metric name not found in path: %s", capturedPath)
	}
	if !strings.Contains(buf.String(), "deleted") {
		t.Errorf("output missing 'deleted': %s", buf.String())
	}
}

func TestMetricsTagConfigDeleteRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	root, _ := buildMetricsTagConfigCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tag-config", "delete"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func buildMetricsTagsCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsTagsCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

func buildMetricsVolumesCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsVolumesCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

func buildMetricsAssetsCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsAssetsCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsTagsResponse = `{
	"data": {
		"id": "system.cpu.user",
		"type": "metrics",
		"attributes": {
			"tags": ["host:web-01", "env:prod"],
			"ingested_tags": ["region:us-east-1"]
		}
	}
}`

const mockMetricsVolumesResponse = `{
	"data": {
		"id": "system.cpu.user",
		"type": "metric_volumes",
		"attributes": {
			"ingested_volume": 12345,
			"indexed_volume": 6789
		}
	}
}`

const mockMetricsAssetsResponse = `{
	"data": {
		"id": "system.cpu.user",
		"type": "metrics"
	},
	"included": [
		{
			"type": "dashboards",
			"id": "abc-123",
			"attributes": {
				"title": "System Overview",
				"popularity": 0.5
			}
		},
		{
			"type": "monitors",
			"id": "456",
			"attributes": {
				"title": "High CPU Alert"
			}
		}
	]
}`

func TestMetricsTagsTableOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTagsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagsCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tags", "system.cpu.user"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "system.cpu.user") {
		t.Errorf("metric name not in path: %s", capturedPath)
	}

	out := buf.String()
	for _, want := range []string{"TAG", "TYPE", "host:web-01", "env:prod", "indexed", "region:us-east-1", "ingested"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsTagsJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTagsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsTagsCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tags", "system.cpu.user", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["data"] == nil {
		t.Errorf("JSON output missing 'data' key:\n%s", buf.String())
	}
}

func TestMetricsTagsRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsTagsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsTagsCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "tags"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsVolumesTableOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsVolumesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsVolumesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "volumes", "system.cpu.user"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "system.cpu.user") {
		t.Errorf("metric name not in path: %s", capturedPath)
	}

	out := buf.String()
	for _, want := range []string{"FIELD", "VALUE", "12345", "6789"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsVolumesJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsVolumesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsVolumesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "volumes", "system.cpu.user", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["data"] == nil {
		t.Errorf("JSON output missing 'data' key:\n%s", buf.String())
	}
}

func TestMetricsVolumesRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsVolumesResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsVolumesCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "volumes"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

func TestMetricsAssetsTableOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsAssetsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsAssetsCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "assets", "system.cpu.user"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "system.cpu.user") {
		t.Errorf("metric name not in path: %s", capturedPath)
	}

	out := buf.String()
	for _, want := range []string{"TYPE", "ID", "TITLE", "dashboard", "abc-123", "System Overview", "monitor", "456", "High CPU Alert"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsAssetsJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsAssetsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsAssetsCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "assets", "system.cpu.user", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["data"] == nil {
		t.Errorf("JSON output missing 'data' key:\n%s", buf.String())
	}
}

func TestMetricsAssetsRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsAssetsResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsAssetsCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "assets"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}

// metrics estimate tests

func buildMetricsEstimateCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "datadog-cli"}
	root.PersistentFlags().Bool("json", false, "output as JSON")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	metrics := &cobra.Command{Use: "metrics"}
	metrics.AddCommand(newMetricsEstimateCmd(mkAPI))
	root.AddCommand(metrics)
	return root, buf
}

const mockMetricsEstimateResponse = `{
	"data": {
		"type": "metric_cardinality",
		"id": "system.cpu.user",
		"attributes": {
			"estimate_type": "count_or_gauge",
			"estimated_output_series": 42,
			"estimated_at": "2026-03-12T00:00:00Z"
		}
	}
}`

func TestMetricsEstimateTableOutput(t *testing.T) {
	t.Parallel()

	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsEstimateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsEstimateCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "estimate", "system.cpu.user"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(capturedPath, "system.cpu.user") {
		t.Errorf("metric name not in path: %s", capturedPath)
	}

	out := buf.String()
	for _, want := range []string{"FIELD", "VALUE", "count_or_gauge", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestMetricsEstimateJSONOutput(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsEstimateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, buf := buildMetricsEstimateCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "estimate", "system.cpu.user", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["data"] == nil {
		t.Errorf("JSON output missing 'data' key:\n%s", buf.String())
	}
}

func TestMetricsEstimateRequiresArg(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMetricsEstimateResponse) //nolint:errcheck
	}))
	defer srv.Close()

	root, _ := buildMetricsEstimateCmd(newTestMetricsV2API(srv))
	root.SetArgs([]string{"metrics", "estimate"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when metric name is missing")
	}
}
