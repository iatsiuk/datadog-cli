# Incidents: incident management, todos, integrations, types

## Overview

Implement the `incidents` command group for datadog-cli, providing CLI access to
Datadog Incident Management APIs. Covers CRUD for incidents, searching, managing
todos, integration metadata, attachments, incident types, and incident services/teams.

Uses `github.com/DataDog/datadog-api-client-go/v2` (datadogV2).

## Context

- SDK APIs used:
  - `datadogV2.IncidentsApi`: ListIncidents, SearchIncidents, GetIncident, CreateIncident, UpdateIncident, DeleteIncident
  - `datadogV2.IncidentsApi`: ListIncidentTodos, GetIncidentTodo, CreateIncidentTodo, UpdateIncidentTodo, DeleteIncidentTodo
  - `datadogV2.IncidentsApi`: ListIncidentIntegrations, GetIncidentIntegration, CreateIncidentIntegration, UpdateIncidentIntegration, DeleteIncidentIntegration
  - `datadogV2.IncidentsApi`: ListIncidentAttachments, CreateIncidentAttachment, UpdateIncidentAttachment, DeleteIncidentAttachment
  - `datadogV2.IncidentsApi`: ListIncidentTypes, GetIncidentType, CreateIncidentType, UpdateIncidentType, DeleteIncidentType
  - `datadogV2.IncidentServicesApi`: ListIncidentServices, GetIncidentService, CreateIncidentService, UpdateIncidentService, DeleteIncidentService
  - `datadogV2.IncidentTeamsApi`: ListIncidentTeams, GetIncidentTeam, CreateIncidentTeam, UpdateIncidentTeam, DeleteIncidentTeam

- Files to create:
  - `internal/cmd/incidents.go` - main command + incident CRUD/search
  - `internal/cmd/incidents_test.go`
  - `internal/cmd/incidents_todo.go` - todo subcommands
  - `internal/cmd/incidents_todo_test.go`
  - `internal/cmd/incidents_integration.go` - integration metadata subcommands
  - `internal/cmd/incidents_integration_test.go`
  - `internal/cmd/incidents_type.go` - incident type subcommands
  - `internal/cmd/incidents_type_test.go`
  - `internal/cmd/incidents_service.go` - incident service subcommands
  - `internal/cmd/incidents_service_test.go`
  - `internal/cmd/incidents_team.go` - incident team subcommands
  - `internal/cmd/incidents_team_test.go`

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

- [x] grep existing `internal/cmd/*.go` for any usage of `IncidentsApi`, `IncidentServicesApi`, `IncidentTeamsApi` - must find zero matches
- [x] grep existing commands for subcommands named `incidents`, `incident` - must find zero matches
- [x] verify `monitors` command does not expose incident management (monitors track alerts, not incidents - but confirm no overlap)
- [x] verify `incidents service` and `incidents team` do not duplicate `users teams` functionality (incidents teams are IncidentTeamsApi, users teams are TeamsApi - different APIs, but confirm no conceptual overlap in CLI UX)
- [x] check root.go registration - confirm no command already covers incidents scope
- [x] document findings: if overlap found, update plan to reuse/extend existing code instead of creating new

### Task 1: incidents command group, list and search subcommands

- [x] write tests for `incidents list`: outputs table (id, title, severity, status, created, commander)
- [x] write tests for `incidents list`: supports --page-size flag
- [x] write tests for `incidents list`: outputs JSON with --json flag
- [x] write tests for `incidents search`: parses --query flag, outputs same table format
- [x] implement `internal/cmd/incidents.go` with `NewIncidentsCommand()`, incidentsAPI type
- [x] implement `incidents list` using `ListIncidents()`
- [x] implement `incidents search` using `SearchIncidents()`
- [x] register `incidents` command in root.go
- [x] run tests - must pass before next task

### Task 2: incidents show, create, update, delete subcommands

- [x] write tests for `incidents show <id>`: outputs detail view (title, severity, status, timeline, commander, fields)
- [x] write tests for `incidents create`: parses --title, --severity (SEV-1..SEV-5), --commander flags
- [x] write tests for `incidents update <id>`: parses --title, --severity, --status flags
- [x] write tests for `incidents delete <id>`: requires --yes flag
- [x] implement `incidents show` using `GetIncident()`
- [x] implement `incidents create` using `CreateIncident()`
- [x] implement `incidents update` using `UpdateIncident()`
- [x] implement `incidents delete` using `DeleteIncident()`
- [x] run tests - must pass before next task

### Task 3: incidents todo subcommands

- [x] write tests for `incidents todo list <incident-id>`: outputs table (id, description, assignees, completed)
- [x] write tests for `incidents todo show <incident-id> <todo-id>`: outputs detail view
- [x] write tests for `incidents todo create <incident-id>`: parses --description, --assignee flags
- [x] write tests for `incidents todo update <incident-id> <todo-id>`: parses --description, --completed flags
- [x] write tests for `incidents todo delete <incident-id> <todo-id>`: requires --yes flag
- [x] implement todo subcommands in `incidents_todo.go`
- [x] run tests - must pass before next task

### Task 4: incidents integration subcommands

- [x] write tests for `incidents integration list <incident-id>`: outputs table (id, type, status)
- [x] write tests for `incidents integration show <incident-id> <integration-id>`: outputs detail view
- [x] write tests for `incidents integration create <incident-id>`: parses --type (slack|jira), --metadata flags
- [x] write tests for `incidents integration update <incident-id> <integration-id>`: parses --metadata flag
- [x] write tests for `incidents integration delete <incident-id> <integration-id>`: requires --yes flag
- [x] implement integration subcommands in `incidents_integration.go`
- [x] run tests - must pass before next task

### Task 5: incidents type subcommands

- [x] write tests for `incidents type list`: outputs table (id, name, description, is_default)
- [x] write tests for `incidents type show <id>`: outputs detail view
- [x] write tests for `incidents type create`: parses --name, --description flags
- [x] write tests for `incidents type update <id>`: parses update flags
- [x] write tests for `incidents type delete <id>`: requires --yes flag
- [x] implement type subcommands in `incidents_type.go`
- [x] run tests - must pass before next task

### Task 6: incidents service subcommands

- [ ] write tests for `incidents service list`: outputs table (id, name)
- [ ] write tests for `incidents service show <id>`: outputs detail view
- [ ] write tests for `incidents service create`: parses --name flag
- [ ] write tests for `incidents service update <id>`: parses --name flag
- [ ] write tests for `incidents service delete <id>`: requires --yes flag
- [ ] implement service subcommands in `incidents_service.go`
- [ ] run tests - must pass before next task

### Task 7: incidents team subcommands

- [ ] write tests for `incidents team list`: outputs table (id, name)
- [ ] write tests for `incidents team show <id>`: outputs detail view
- [ ] write tests for `incidents team create`: parses --name flag
- [ ] write tests for `incidents team update <id>`: parses --name flag
- [ ] write tests for `incidents team delete <id>`: requires --yes flag
- [ ] implement team subcommands in `incidents_team.go`
- [ ] run tests - must pass before next task

### Task 8: Verify acceptance criteria

- [ ] verify all subcommands registered and accessible
- [ ] verify edge cases (missing flags, API errors)
- [ ] run full test suite (`go test -race ./...`)
- [ ] run linter (`golangci-lint run`)
- [ ] build with `make build`

## Technical Details

- All Incidents APIs are in datadogV2
- IncidentServicesApi and IncidentTeamsApi are separate API types from IncidentsApi
- Incident severity values: SEV-1, SEV-2, SEV-3, SEV-4, SEV-5
- Incident status values: active, stable, resolved
- Commander is a user handle (email)
- Todo assignees are user handles
- Integration types: slack_postings, jira_issues
- Some operations may require unstable operation flag - check at implementation time
- Attachments can be postmortem links or other attachment types

## Post-Completion

**Manual verification:**
- Test with real Datadog account: list incidents, create/update/delete
- Test todo workflow: create todo, mark completed, delete
- Verify severity and status transitions work correctly
