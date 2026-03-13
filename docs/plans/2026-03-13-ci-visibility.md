# CI Visibility: pipeline and test events search, aggregation

## Overview

Implement the `ci` command group for datadog-cli, providing CLI access to
Datadog CI Visibility APIs. Covers searching and aggregating CI pipeline events
and CI test events.

Uses `github.com/DataDog/datadog-api-client-go/v2` (datadogV2).

## Context

- SDK APIs used:
  - `datadogV2.CIVisibilityPipelinesApi`: ListCIAppPipelineEvents, SearchCIAppPipelineEvents, AggregateCIAppPipelineEvents, CreateCIAppPipelineEvent
  - `datadogV2.CIVisibilityTestsApi`: ListCIAppTestEvents, SearchCIAppTestEvents, AggregateCIAppTestEvents

- Files to create:
  - `internal/cmd/ci.go` - main command + pipeline subcommands
  - `internal/cmd/ci_test.go`
  - `internal/cmd/ci_tests.go` - CI test events subcommands
  - `internal/cmd/ci_tests_test.go`

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
- Reuse patterns from `apm search` / `apm aggregate` (similar event search APIs)

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with + prefix
- Document issues/blockers with ! prefix

## Implementation Steps

### Task 0: Deduplicate - verify no overlap with existing commands

- [x] grep existing `internal/cmd/*.go` for any usage of `CIVisibilityPipelinesApi`, `CIVisibilityTestsApi`, or `CIApp` - must find zero matches
- [x] grep existing commands for subcommands named `ci`, `pipeline`, `cicd` - must find zero matches
- [x] verify `apm` command does not already expose CI pipeline or CI test events (separate Datadog domain, but confirm)
- [x] verify `events` command does not overlap with CI pipeline events (events API is generic EventsApi, CI uses dedicated CIVisibility APIs)
- [x] check root.go registration - confirm no command already covers CI visibility scope
- [x] document findings: if overlap found, update plan to reuse/extend existing code instead of creating new

### Task 1: ci command group and pipeline search subcommand

- [x] write tests for `ci pipeline search`: parses --query, --from, --to, --limit, --sort flags
- [x] write tests for `ci pipeline search`: formats pipeline events as table (timestamp, pipeline_name, status, duration, git_branch)
- [x] write tests for `ci pipeline search`: outputs JSON with --json flag
- [x] write tests for `ci pipeline search`: defaults --from to "now-1h" when omitted
- [x] implement `internal/cmd/ci.go` with `NewCICommand()`, pipelinesAPI type
- [x] implement `ci pipeline search` using `ListCIAppPipelineEvents()`
- [x] register `ci` command in root.go
- [x] run tests - must pass before next task

### Task 2: ci pipeline tail subcommand

- [x] write tests for `ci pipeline tail`: parses --query flag
- [x] write tests for `ci pipeline tail`: polls API and prints new pipeline events
- [x] implement `ci pipeline tail` using `ListCIAppPipelineEvents()` in a poll loop
- [x] run tests - must pass before next task

### Task 3: ci pipeline aggregate subcommand

- [x] write tests for `ci pipeline aggregate`: parses --query, --from, --to, --group-by, --compute flags
- [x] write tests for `ci pipeline aggregate`: formats aggregation results as table
- [x] write tests for `ci pipeline aggregate`: outputs JSON with --json flag
- [x] implement `ci pipeline aggregate` using `AggregateCIAppPipelineEvents()`
- [x] run tests - must pass before next task

### Task 4: ci pipeline create subcommand

- [x] write tests for `ci pipeline create`: parses --pipeline-name, --status, --level, --git-branch, --git-sha flags
- [x] write tests for `ci pipeline create`: sends correct request structure
- [x] implement `ci pipeline create` using `CreateCIAppPipelineEvent()`
- [x] run tests - must pass before next task

### Task 5: ci test search subcommand

- [x] write tests for `ci test search`: parses --query, --from, --to, --limit, --sort flags
- [x] write tests for `ci test search`: formats test events as table (timestamp, test_name, suite, status, duration, service)
- [x] write tests for `ci test search`: outputs JSON with --json flag
- [x] implement `internal/cmd/ci_tests.go` with testsAPI type
- [x] implement `ci test search` using `ListCIAppTestEvents()`
- [x] run tests - must pass before next task

### Task 6: ci test tail subcommand

- [ ] write tests for `ci test tail`: parses --query flag
- [ ] write tests for `ci test tail`: polls and prints new test events
- [ ] implement `ci test tail` using `ListCIAppTestEvents()` in a poll loop
- [ ] run tests - must pass before next task

### Task 7: ci test aggregate subcommand

- [ ] write tests for `ci test aggregate`: parses --query, --from, --to, --group-by, --compute flags
- [ ] write tests for `ci test aggregate`: formats aggregation results as table
- [ ] implement `ci test aggregate` using `AggregateCIAppTestEvents()`
- [ ] run tests - must pass before next task

### Task 8: Verify acceptance criteria

- [ ] verify all subcommands registered and accessible
- [ ] verify edge cases (missing flags, API errors)
- [ ] run full test suite (`go test -race ./...`)
- [ ] run linter (`golangci-lint run`)
- [ ] build with `make build`

## Technical Details

- CIVisibilityPipelinesApi and CIVisibilityTestsApi are separate API types in datadogV2
- Pipeline events have attributes: pipeline_name, status (success/error/canceled), duration, git info
- Test events have attributes: test_name, suite, status (pass/fail/skip), duration, service
- Search uses `ListCIAppPipelineEvents()` with filter parameters (simpler than POST search)
- Aggregation uses same pattern as `apm aggregate` and `logs aggregate`
- `CreateCIAppPipelineEvent()` allows sending custom pipeline events
- Time parameters use same relative time syntax as logs/apm ("now-15m", "now-1h")

## Post-Completion

**Manual verification:**
- Test with real CI pipeline data: search events, aggregate by status/branch
- Verify table formatting with various pipeline/test statuses
