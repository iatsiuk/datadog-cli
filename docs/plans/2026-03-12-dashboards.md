# Dashboards -- Visualization

## Overview
- Implement dashboards management: CRUD (list/get/create/update/delete)
- Implement dashboard lists management: CRUD + add/remove dashboards from lists
- Enables CLI-based dashboard management without Datadog UI

## Context
- Existing patterns: `internal/cmd/events.go`, `internal/cmd/logs.go` -- API wrapper struct, DI via `mkAPI`, cobra subcommands
- APIs: `datadogV1.DashboardsApi`, `datadogV2.DashboardListsApi`
- Output: `internal/output` package (PrintTable/PrintJSON)
- Tests: httptest mock server pattern, table-driven tests
- Registration: `internal/cmd/root.go` -- `NewDashboardsCommand()` added to root

## Development Approach
- **Testing approach**: TDD (tests first, then implementation)
- Complete each task fully before moving to the next
- Make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
- **CRITICAL: all tests must pass before starting next task**
- **CRITICAL: update this plan file when scope changes during implementation**
- Run tests after each change
- Maintain backward compatibility

## Testing Strategy
- **Unit tests**: httptest mock server for every API operation
- **Pattern**: table-driven tests with `t.Parallel()`, request body inspection
- **Coverage**: success paths, error paths, flag validation, JSON/table output modes
- **Build**: `make build` after each task (includes linting)

## Progress Tracking
- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with + prefix
- Document issues/blockers with ! prefix
- Update plan if implementation deviates from original scope

## Implementation Steps

### Task 1: Dashboards API wrapper and command skeleton with tests
- [x] write tests for `newTestDashboardsAPI` helper (mock server creation)
- [x] write tests for `NewDashboardsCommand()` returning correct subcommands
- [x] implement `dashboardsAPI` struct wrapping `datadogV1.DashboardsApi` in `internal/cmd/dashboards.go`
- [x] implement `defaultDashboardsAPI()` with config loading and client creation
- [x] implement `NewDashboardsCommand()` with subcommand registration
- [x] register `NewDashboardsCommand()` in `root.go`
- [x] run `make build` -- must pass

### Task 2: Dashboard list with tests
- [x] write tests for `dashboards list` -- table output (id, title, layout_type, url, created_at, modified_at)
- [x] write tests for `dashboards list` -- JSON output mode
- [x] write tests for `dashboards list` -- empty result
- [x] implement `newDashboardsListCmd` -- call `ListDashboards`, format as table/JSON
- [x] run `make build` -- must pass

### Task 3: Dashboard show with tests
- [x] write tests for `dashboards show --id <id>` -- full dashboard details
- [x] write tests for `dashboards show` -- JSON output (full widget tree)
- [x] write tests for `dashboards show` -- missing required `--id` flag
- [x] implement `newDashboardsShowCmd` -- call `GetDashboard`, display details
- [x] run `make build` -- must pass

### Task 4: Dashboard create with tests
- [x] write tests for `dashboards create` -- capture request body with title, layout_type, widgets (from JSON file/string)
- [x] write tests for `dashboards create` -- required flags validation (title, layout_type)
- [x] write tests for `dashboards create` -- optional flags (description, tags, template_variables)
- [x] implement `newDashboardsCreateCmd` -- flags for title, layout_type, description, widgets (JSON), tags
- [x] run `make build` -- must pass

### Task 5: Dashboard update with tests
- [x] write tests for `dashboards update --id <id>` -- capture request body
- [x] write tests for `dashboards update` -- full replace semantics (dashboard API is PUT-based)
- [x] implement `newDashboardsUpdateCmd` -- accept full dashboard JSON or individual flags
- [x] run `make build` -- must pass

### Task 6: Dashboard delete with tests
- [x] write tests for `dashboards delete --id <id> --yes` -- success response
- [x] write tests for `dashboards delete` -- missing `--yes` flag rejected
- [x] implement `newDashboardsDeleteCmd` -- require `--yes`, call `DeleteDashboard`
- [x] run `make build` -- must pass

### Task 7: Dashboard Lists API wrapper and list with tests
- [x] write tests for `newTestDashboardListsAPI` helper
- [x] write tests for `dashboards lists list` -- table output (id, name, dashboard_count, created, modified)
- [x] write tests for `dashboards lists list` -- JSON output
- [x] implement `dashboardListsAPI` struct wrapping `datadogV2.DashboardListsApi`
- [x] implement `newDashboardListsListCmd`
- [x] run `make build` -- must pass

### Task 8: Dashboard list show/create/update/delete with tests
- [x] write tests for `dashboards lists show --id <id>` -- list details with contained dashboards
- [x] write tests for `dashboards lists create --name <name>` -- capture request body
- [x] write tests for `dashboards lists update --id <id> --name <name>`
- [x] write tests for `dashboards lists delete --id <id> --yes`
- [x] implement show/create/update/delete subcommands for dashboard lists
- [x] run `make build` -- must pass

### Task 9: Dashboard list items add/remove with tests
- [ ] write tests for `dashboards lists add-items --id <list-id> --dashboard <dash-id> --type <type>`
- [ ] write tests for `dashboards lists remove-items --id <list-id> --dashboard <dash-id> --type <type>`
- [ ] implement `newDashboardListsAddItemsCmd` -- call `CreateDashboardListItems`
- [ ] implement `newDashboardListsRemoveItemsCmd` -- call `DeleteDashboardListItems`
- [ ] run `make build` -- must pass

### Task 10: Verify acceptance criteria
- [ ] verify all dashboard operations: list, show, create, update, delete
- [ ] verify all dashboard list operations: list, show, create, update, delete, add-items, remove-items
- [ ] verify edge cases: empty results, missing flags, `--yes` confirmation
- [ ] run full test suite: `go test -race ./...`
- [ ] run linter: `make build`

### Task 11: [Final] Update documentation
- [ ] update CLAUDE.md API reference section with Dashboards/Dashboard Lists APIs
- [ ] update README.md with dashboards command examples

## Technical Details
- **Layout types**: `ordered` (grid-based) or `free` (pixel-positioned)
- **Widgets**: complex nested JSON structure -- accept via `--widgets-json` flag (file path or inline)
- **Template variables**: list of `{name, prefix, default}` objects
- **Dashboard Lists V2**: uses `datadogV2.DashboardListsApi` for items management
- **Item types**: `custom_timeboard`, `custom_screenboard`, `integration_timeboard`, `integration_screenboard`, `host_timeboard`

## Post-Completion
- Manual testing with real Datadog account
- Test dashboard creation with complex widget configurations
- Verify dashboard list membership operations
