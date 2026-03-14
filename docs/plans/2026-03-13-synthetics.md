# Synthetics: API/browser tests, global variables, private locations

## Overview

Implement the `synthetics` command group for datadog-cli, providing CLI access to
Datadog Synthetics APIs. Covers managing API/browser/mobile tests, viewing test
results, managing global variables and private locations, triggering tests, and
fetching uptime data.

Uses `github.com/DataDog/datadog-api-client-go/v2` (datadogV1.SyntheticsApi).

## Context

- SDK APIs used:
  - `datadogV1.SyntheticsApi`: ListTests, SearchTests, GetTest, GetAPITest, GetBrowserTest, GetMobileTest
  - `datadogV1.SyntheticsApi`: CreateSyntheticsAPITest, CreateSyntheticsBrowserTest, DeleteTests, PatchTest
  - `datadogV1.SyntheticsApi`: GetAPITestLatestResults, GetAPITestResult, GetBrowserTestLatestResults, GetBrowserTestResult
  - `datadogV1.SyntheticsApi`: TriggerTests, TriggerCITests, GetSyntheticsCIBatch
  - `datadogV1.SyntheticsApi`: ListGlobalVariables, GetGlobalVariable, CreateGlobalVariable, EditGlobalVariable, DeleteGlobalVariable
  - `datadogV1.SyntheticsApi`: ListLocations, GetSyntheticsDefaultLocations
  - `datadogV1.SyntheticsApi`: ListPrivateLocations (via ListLocations), GetPrivateLocation, CreatePrivateLocation, DeletePrivateLocation
  - `datadogV1.SyntheticsApi`: FetchUptimes

- Files to create:
  - `internal/cmd/synthetics.go` - main command + test list/search/show/delete
  - `internal/cmd/synthetics_test.go` - tests for above
  - `internal/cmd/synthetics_results.go` - test results subcommands
  - `internal/cmd/synthetics_results_test.go`
  - `internal/cmd/synthetics_trigger.go` - trigger/batch subcommands
  - `internal/cmd/synthetics_trigger_test.go`
  - `internal/cmd/synthetics_variable.go` - global variables CRUD
  - `internal/cmd/synthetics_variable_test.go`
  - `internal/cmd/synthetics_location.go` - locations + private locations
  - `internal/cmd/synthetics_location_test.go`

## Development Approach

- **Testing approach**: TDD (tests first)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**
- **CRITICAL: update this plan file when scope changes during implementation**

## Testing Strategy

- Mock HTTP responses using `httptest.Server`
- Table-driven tests for flag parsing and output
- Test both JSON and table output modes
- Test error cases (missing required flags, API errors)

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with + prefix
- Document issues/blockers with ! prefix

## Implementation Steps

### Task 0: Deduplicate - verify no overlap with existing commands

- [x] grep existing `internal/cmd/*.go` for any usage of `datadogV1.SyntheticsApi` or `SyntheticsApi` - must find zero matches
- [x] grep existing commands for subcommands named `synthetics`, `synthetic`, `synth` - must find zero matches
- [x] verify no existing command exposes test triggering, global variables, or private locations under different names (e.g., check `monitors`, `apm` for overlapping concepts)
- [x] check root.go registration - confirm no command already covers synthetics scope
- [x] document findings: if overlap found, update plan to reuse/extend existing code instead of creating new

### Task 1: synthetics command group and test list/search subcommands

- [x] write tests for `synthetics list`: outputs table (public_id, name, type, status, locations)
- [x] write tests for `synthetics list`: supports --page-size flag
- [x] write tests for `synthetics list`: outputs JSON with --json flag
- [x] write tests for `synthetics search`: parses --query flag, outputs same table format
- [x] implement `internal/cmd/synthetics.go` with `NewSyntheticsCommand()`, syntheticsAPI type
- [x] implement `synthetics list` using `ListTests()`
- [x] implement `synthetics search` using `SearchTests()`
- [x] register `synthetics` command in root.go
- [x] run tests - must pass before next task

### Task 2: synthetics show and delete subcommands

- [ ] write tests for `synthetics show <public-id>`: outputs API test details (type detection)
- [ ] write tests for `synthetics show <public-id>`: outputs browser test details
- [ ] write tests for `synthetics delete`: parses --id flag (comma-separated), requires --yes
- [ ] implement `synthetics show` using `GetAPITest()` / `GetBrowserTest()` / `GetMobileTest()` with fallback
- [ ] implement `synthetics delete` using `DeleteTests()`
- [ ] run tests - must pass before next task

### Task 3: synthetics create subcommands

- [ ] write tests for `synthetics create api`: parses --name, --type (http|ssl|dns|tcp|icmp|grpc|websocket), --url, --locations, --frequency, --status flags
- [ ] write tests for `synthetics create api`: creates test with correct config structure
- [ ] write tests for `synthetics create browser`: parses --name, --url, --locations, --frequency flags
- [ ] implement `synthetics create api` using `CreateSyntheticsAPITest()`
- [ ] implement `synthetics create browser` using `CreateSyntheticsBrowserTest()`
- [ ] run tests - must pass before next task

### Task 4: synthetics results subcommands

- [ ] write tests for `synthetics results <public-id>`: outputs latest results table (timestamp, location, status, duration)
- [ ] write tests for `synthetics results <public-id> --result-id <id>`: outputs single result detail
- [ ] write tests for results: handles both API and browser test types
- [ ] implement `synthetics results` in `synthetics_results.go` using `GetAPITestLatestResults()` / `GetBrowserTestLatestResults()`
- [ ] implement `synthetics results --result-id` using `GetAPITestResult()` / `GetBrowserTestResult()`
- [ ] run tests - must pass before next task

### Task 5: synthetics trigger and batch subcommands

- [ ] write tests for `synthetics trigger`: parses --id flag (comma-separated public IDs)
- [ ] write tests for `synthetics trigger`: outputs triggered results table
- [ ] write tests for `synthetics batch <batch-id>`: outputs batch details
- [ ] implement `synthetics trigger` in `synthetics_trigger.go` using `TriggerTests()`
- [ ] implement `synthetics batch` using `GetSyntheticsCIBatch()`
- [ ] run tests - must pass before next task

### Task 6: synthetics variable subcommands (global variables CRUD)

- [ ] write tests for `synthetics variable list`: outputs table (id, name, type, tags)
- [ ] write tests for `synthetics variable show <id>`: outputs detail view
- [ ] write tests for `synthetics variable create`: parses --name, --value, --type, --tags flags
- [ ] write tests for `synthetics variable update <id>`: parses update flags
- [ ] write tests for `synthetics variable delete <id>`: requires --yes flag
- [ ] implement all variable subcommands in `synthetics_variable.go`
- [ ] run tests - must pass before next task

### Task 7: synthetics location subcommands

- [ ] write tests for `synthetics location list`: outputs table (id, name, region, is_private)
- [ ] write tests for `synthetics location defaults`: outputs default locations list
- [ ] write tests for `synthetics private-location show <id>`: outputs detail view
- [ ] write tests for `synthetics private-location create`: parses --name, --tags flags
- [ ] write tests for `synthetics private-location delete <id>`: requires --yes flag
- [ ] implement location commands in `synthetics_location.go`
- [ ] run tests - must pass before next task

### Task 8: synthetics uptime subcommand

- [ ] write tests for `synthetics uptime`: parses --id (comma-separated), --from, --to flags
- [ ] write tests for `synthetics uptime`: outputs uptime percentage table
- [ ] implement `synthetics uptime` using `FetchUptimes()`
- [ ] run tests - must pass before next task

### Task 9: Verify acceptance criteria

- [ ] verify all subcommands registered and accessible
- [ ] verify edge cases (missing flags, API errors)
- [ ] run full test suite (`go test -race ./...`)
- [ ] run linter (`golangci-lint run`)
- [ ] build with `make build`

## Technical Details

- SyntheticsApi is in datadogV1, not datadogV2
- Test types: `api`, `browser`, `mobile` - use `GetTest()` first, then type-specific getter
- `DeleteTests()` accepts a batch payload with multiple public IDs
- `TriggerTests()` vs `TriggerCITests()` - use `TriggerTests()` for general triggering
- Locations are strings like "aws:us-east-1", private locations have custom IDs
- Global variables can be of type `text` or `email`

## Post-Completion

**Manual verification:**
- Test with real Datadog account: list tests, show details, trigger a test
- Verify table formatting with various test types and statuses
