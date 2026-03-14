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

### CI Visibility

- `datadogV2.CIVisibilityPipelinesApi`: ListCIAppPipelineEvents, AggregateCIAppPipelineEvents, CreateCIAppPipelineEvent
- `datadogV2.CIVisibilityTestsApi`: ListCIAppTestEvents, AggregateCIAppTestEvents

### Dashboards

- `datadogV1.DashboardsApi`: ListDashboards, GetDashboard, CreateDashboard, UpdateDashboard, DeleteDashboard
- `datadogV1.DashboardListsApi`: ListDashboardLists, GetDashboardList, CreateDashboardList, UpdateDashboardList, DeleteDashboardList
- `datadogV2.DashboardListsApi`: CreateDashboardListItems, DeleteDashboardListItems

### Events

- `datadogV2.EventsApi`: ListEvents, SearchEvents, GetEvent, CreateEvent
- Note: CreateEvent requires unstable operation flag: `c.GetConfig().SetUnstableOperationEnabled("v2.EventsApi.CreateEvent", true)`
- `AlertEventCustomAttributesStatus` has three values: OK, WARN, ERROR (no SUCCESS constant)

### Hosts

- `datadogV1.HostsApi`: ListHosts, GetHostTotals, MuteHost, UnmuteHost
- `datadogV1.TagsApi`: ListHostTags, GetHostTags, CreateHostTags, UpdateHostTags, DeleteHostTags

### Monitors

- `datadogV1.MonitorsApi`: ListMonitors, GetMonitor, SearchMonitors, CreateMonitor, UpdateMonitor, DeleteMonitor

### Downtimes

- `datadogV2.DowntimesApi`: ListDowntimes, GetDowntime, CreateDowntime, UpdateDowntime, CancelDowntime

### Monitor Config Policies

- `datadogV2.MonitorsApi`: ListMonitorConfigPolicies, GetMonitorConfigPolicy, CreateMonitorConfigPolicy, UpdateMonitorConfigPolicy, DeleteMonitorConfigPolicy

### SLOs

- `datadogV1.ServiceLevelObjectivesApi`: ListSLOs, GetSLO, CreateSLO, UpdateSLO, DeleteSLO, CheckCanDeleteSLO, GetSLOHistory
- `datadogV1.ServiceLevelObjectiveCorrectionsApi`: ListSLOCorrection, GetSLOCorrection, CreateSLOCorrection, UpdateSLOCorrection, DeleteSLOCorrection

### Synthetics

- `datadogV1.SyntheticsApi`: ListTests, SearchTests, GetTest, GetAPITest, GetBrowserTest, GetMobileTest
- `datadogV1.SyntheticsApi`: CreateSyntheticsAPITest, CreateSyntheticsBrowserTest, DeleteTests
- `datadogV1.SyntheticsApi`: GetAPITestLatestResults, GetAPITestResult, GetBrowserTestLatestResults, GetBrowserTestResult
- `datadogV1.SyntheticsApi`: TriggerTests, GetSyntheticsCIBatch
- `datadogV1.SyntheticsApi`: ListGlobalVariables, GetGlobalVariable, CreateGlobalVariable, EditGlobalVariable, DeleteGlobalVariable
- `datadogV1.SyntheticsApi`: ListLocations, GetSyntheticsDefaultLocations, GetPrivateLocation, CreatePrivateLocation, DeletePrivateLocation
- `datadogV1.SyntheticsApi`: FetchUptimes
- Note: SyntheticsApi is in datadogV1, not datadogV2

### Users

- `datadogV2.UsersApi`: ListUsers, GetUser, CreateUser, UpdateUser, DisableUser, SendInvitations

### Roles

- `datadogV2.RolesApi`: ListRoles, GetRole, CreateRole, UpdateRole, DeleteRole, ListRolePermissions, AddPermissionToRole, RemovePermissionFromRole

### Teams

- `datadogV2.TeamsApi`: ListTeams, GetTeam, CreateTeam, UpdateTeam, DeleteTeam, GetTeamMemberships, CreateTeamMembership, DeleteTeamMembership

### Incidents

- `datadogV2.IncidentsApi`: ListIncidents, SearchIncidents, GetIncident, CreateIncident, UpdateIncident, DeleteIncident
- `datadogV2.IncidentsApi`: ListIncidentTodos, GetIncidentTodo, CreateIncidentTodo, UpdateIncidentTodo, DeleteIncidentTodo
- `datadogV2.IncidentsApi`: ListIncidentIntegrations, GetIncidentIntegration, CreateIncidentIntegration, UpdateIncidentIntegration, DeleteIncidentIntegration
- `datadogV2.IncidentsApi`: ListIncidentTypes, GetIncidentType, CreateIncidentType, UpdateIncidentType, DeleteIncidentType
- `datadogV2.IncidentServicesApi`: ListIncidentServices, GetIncidentService, CreateIncidentService, UpdateIncidentService, DeleteIncidentService
- `datadogV2.IncidentTeamsApi`: ListIncidentTeams, GetIncidentTeam, CreateIncidentTeam, UpdateIncidentTeam, DeleteIncidentTeam
- Note: All Incidents API operations require unstable operation flags via `ddCfg.SetUnstableOperationEnabled("v2.<OperationName>", true)`

### Security Monitoring

- `datadogV2.SecurityMonitoringApi`: ListSecurityMonitoringSignals, GetSecurityMonitoringSignal, EditSecurityMonitoringSignalState, EditSecurityMonitoringSignalAssignee, EditSecurityMonitoringSignalIncidents
- `datadogV2.SecurityMonitoringApi`: ListSecurityMonitoringRules, GetSecurityMonitoringRule, CreateSecurityMonitoringRule, UpdateSecurityMonitoringRule, DeleteSecurityMonitoringRule, ValidateSecurityMonitoringRule
- `datadogV2.SecurityMonitoringApi`: ListSecurityMonitoringSuppressions, GetSecurityMonitoringSuppression, CreateSecurityMonitoringSuppression, UpdateSecurityMonitoringSuppression, DeleteSecurityMonitoringSuppression
- `datadogV2.SecurityMonitoringApi`: ListSecurityFilters, GetSecurityFilter, CreateSecurityFilter, UpdateSecurityFilter, DeleteSecurityFilter
- `datadogV2.SecurityMonitoringApi`: ListFindings, GetFinding, MuteFindings
- Note: Findings operations require unstable operation flags: `cfg.SetUnstableOperationEnabled("v2.ListFindings", true)`, same for `v2.GetFinding` and `v2.MuteFindings`

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
