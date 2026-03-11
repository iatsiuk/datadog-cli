package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	root := &cobra.Command{Use: "dd"}
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
	root := &cobra.Command{Use: "dd"}
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
	root := &cobra.Command{Use: "dd"}
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

	if !strings.Contains(capturedURL, "from=") {
		t.Errorf("from param not found in URL: %s", capturedURL)
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
	root := &cobra.Command{Use: "dd"}
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

	var result interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
}

func buildMetricsTimeseriesCmd(mkAPI func() (*metricsV2API, error)) (*cobra.Command, *bytes.Buffer) {
	root := &cobra.Command{Use: "dd"}
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

	var result interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
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
	if !strings.Contains(capturedURL, "from=") {
		t.Errorf("from param not found in URL: %s", capturedURL)
	}
}
