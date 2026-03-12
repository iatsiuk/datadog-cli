# Datadog-CLI Project Instructions

## Project Overview

CLI tool for querying Datadog from the terminal.

## API Reference

### Logs

- `datadogV2.LogsApi`: ListLogsGet (search, tail), AggregateLogs
- `datadogV1.LogsIndexesApi`: ListLogIndexes, GetLogsIndex, CreateLogsIndex, UpdateLogsIndex, DeleteLogsIndex
- `datadogV1.LogsPipelinesApi`: ListLogsPipelines, GetLogsPipeline, CreateLogsPipeline, UpdateLogsPipeline, DeleteLogsPipeline
- `datadogV2.LogsArchivesApi`: ListLogsArchives, GetLogsArchive, CreateLogsArchive, UpdateLogsArchive, DeleteLogsArchive
- `datadogV2.LogsMetricsApi`: ListLogsMetrics, GetLogsMetric, CreateLogsMetric, UpdateLogsMetric, DeleteLogsMetric
- `datadogV2.LogsCustomDestinationsApi`: ListLogsCustomDestinations, GetLogsCustomDestination, CreateLogsCustomDestination, UpdateLogsCustomDestination, DeleteLogsCustomDestination
- `datadogV2.LogsRestrictionQueriesApi`: ListRestrictionQueries, GetRestrictionQuery, CreateRestrictionQuery, UpdateRestrictionQuery, DeleteRestrictionQuery

### Metrics

- `datadogV1.MetricsApi`: QueryMetrics, ListMetrics, ListActiveMetrics, GetMetricMetadata, UpdateMetricMetadata
- `datadogV2.MetricsApi`: QueryScalarData, QueryTimeseriesData, SubmitMetrics, ListTagConfigurations, GetTagConfiguration, CreateTagConfiguration, UpdateTagConfiguration, DeleteTagConfiguration, ListTagsByMetricName, ListVolumesByMetricName, ListMetricAssets, EstimateMetricsOutputSeries

### APM

- `datadogV2.SpansApi`: ListSpansGet, AggregateSpans
- `datadogV2.APMApi`: GetServiceList
- `datadogV2.APMRetentionFiltersApi`: ListApmRetentionFilters, GetApmRetentionFilter, CreateApmRetentionFilter, UpdateApmRetentionFilter, DeleteApmRetentionFilter
- `datadogV2.SpansMetricsApi`: ListSpansMetrics, GetSpansMetric, CreateSpansMetric, UpdateSpansMetric, DeleteSpansMetric

### Events

- `datadogV2.EventsApi`: ListEvents, SearchEvents, GetEvent, CreateEvent
- Note: CreateEvent requires unstable operation flag: `c.GetConfig().SetUnstableOperationEnabled("v2.EventsApi.CreateEvent", true)`
- `AlertEventCustomAttributesStatus` has three values: OK, WARN, ERROR (no SUCCESS constant)

## Code Style

### Imports

Group imports in order, separated by blank lines:

1. Standard library
2. External packages
3. Local packages (`datadog-cli/...`)

```go
import (
    "context"
    "fmt"

    "github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"

    "datadog-cli/internal/config"
)
```

### Naming

- Package names: short, lowercase, no underscores (`config`, `query`, `output`)
- Exported types: PascalCase (`Config`, `LogEntry`, `SpanResult`)
- Unexported: camelCase (`buildFilter`, `formatTimestamp`)
- Acronyms: consistent case (`URL`, `HTTP`, `API` or `url`, `http`, `api`)
- Receivers: short, 1-2 letters (`c` for `*Client`, `q` for `*Query`)
- Errors: `Err` prefix for sentinel errors (`ErrAuthRequired`)

### Functions

- Early returns for error handling
- Group related functions together

### Error Handling

- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- Check all errors (enforced by `errcheck` linter)
- Use `errors.Is`/`errors.As` for error comparison
- Sentinel errors as package-level variables

### Comments

- Only for non-obvious logic
- English, lowercase, brief
- No comments for self-explanatory code

### Structs

- JSON tags on all exported fields: `json:"field_name"`
- Use `omitempty` for optional fields
- Pointer types for optional values (`*float64`, `*int`)
- Group related fields together

### Variables

- Package-level constants in `const` block
- Related constants grouped together
- Unexported package variables with `var`

### Control Flow

- Use `range` with index for modifying slices
- Prefer `for i := range n` over `for i := 0; i < n; i++` (Go 1.22+)
- Use `switch` over long `if-else` chains

### Concurrency

- Use `context.Context` as first parameter
- Use `sync.Mutex` for simple locking
- Use `errgroup` for parallel operations

## Testing

- Use table-driven tests for multiple scenarios
- Use stdlib `testing` package only (no testify)
- Test error paths: timeouts, context cancellation
- Run with race detector: `go test -race ./...`
- Use `t.Parallel()` for independent tests
- Test files: `*_test.go` in same package

## Language

All documentation, comments, and text must be in English.

## Building

- Always build with `make build` (runs linter automatically)
- Direct `go build` skips linting - avoid it

## Linting and Formatting

- Run `golangci-lint run` before committing (executed automatically via `make build`)
- Fix formatting issues with `goimports -w <file>` or `gofmt -w <file>`
- Config: `.golangci.yml` defines enabled linters
- No trailing whitespace, proper import grouping (stdlib, external, local)
