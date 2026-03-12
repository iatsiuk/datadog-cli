# Hosts/Tags -- Infrastructure Management

## Overview
- Implement hosts management: list, search, get totals, mute/unmute
- Implement host tags management: CRUD (list/get/create/update/delete)
- Enables CLI-based infrastructure visibility and tag management

## Context
- Existing patterns: `internal/cmd/events.go` -- API wrapper struct, DI via `mkAPI`, cobra subcommands
- APIs: `datadogV1.HostsApi`, `datadogV1.TagsApi`
- Output: `internal/output` package (PrintTable/PrintJSON)
- Tests: httptest mock server pattern, table-driven tests
- Registration: `internal/cmd/root.go` -- `NewHostsCommand()` added to root

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

### Task 1: Hosts API wrapper and command skeleton with tests
- [x] write tests for `newTestHostsAPI` helper (mock server creation)
- [x] write tests for `NewHostsCommand()` returning correct subcommands
- [x] implement `hostsAPI` struct wrapping `datadogV1.HostsApi` in `internal/cmd/hosts.go`
- [x] implement `defaultHostsAPI()` with config loading and client creation
- [x] implement `NewHostsCommand()` with subcommand registration
- [x] register `NewHostsCommand()` in `root.go`
- [x] run `make build` -- must pass

### Task 2: Host list with tests
- [ ] write tests for `hosts list` -- table output (name, id, aliases, apps, sources, up, last_reported_time)
- [ ] write tests for `hosts list` -- JSON output mode
- [ ] write tests for `hosts list` -- with `--filter` flag for search
- [ ] write tests for `hosts list` -- with `--from` flag (Unix timestamp, only active hosts since)
- [ ] write tests for `hosts list` -- empty result
- [ ] implement `newHostsListCmd` -- call `ListHosts` with optional filter/from/count/start
- [ ] run `make build` -- must pass

### Task 3: Host totals with tests
- [ ] write tests for `hosts totals` -- display total/active host counts
- [ ] write tests for `hosts totals` -- JSON output
- [ ] implement `newHostsTotalsCmd` -- call `GetHostTotals`, display total_up/total_active
- [ ] run `make build` -- must pass

### Task 4: Host mute/unmute with tests
- [ ] write tests for `hosts mute --name <hostname>` -- success, optional `--end`, `--message`, `--override`
- [ ] write tests for `hosts unmute --name <hostname>` -- success
- [ ] write tests for `hosts mute` -- missing required `--name` flag
- [ ] implement `newHostsMuteCmd` -- call `MuteHost` with optional end/message/override
- [ ] implement `newHostsUnmuteCmd` -- call `UnmuteHost`
- [ ] run `make build` -- must pass

### Task 5: Tags API wrapper and list all tags with tests
- [ ] write tests for `newTestTagsAPI` helper
- [ ] write tests for `hosts tags list` -- table output (host, tags grouped by source)
- [ ] write tests for `hosts tags list` -- JSON output
- [ ] write tests for `hosts tags list` -- with `--source` filter
- [ ] implement `tagsAPI` struct wrapping `datadogV1.TagsApi`
- [ ] implement `newTagsListCmd` -- call `ListHostTags`, display all host-tag mappings
- [ ] run `make build` -- must pass

### Task 6: Tags show/create with tests
- [ ] write tests for `hosts tags show --name <hostname>` -- tags for specific host by source
- [ ] write tests for `hosts tags create --name <hostname> --tags <tag1,tag2>` -- capture request body
- [ ] write tests for `hosts tags create` -- required flags validation (name, tags)
- [ ] implement `newTagsShowCmd` -- call `GetHostTags`
- [ ] implement `newTagsCreateCmd` -- call `CreateHostTags` with tag list and optional source
- [ ] run `make build` -- must pass

### Task 7: Tags update/delete with tests
- [ ] write tests for `hosts tags update --name <hostname> --tags <tag1,tag2>` -- capture request body
- [ ] write tests for `hosts tags delete --name <hostname> --yes` -- success
- [ ] write tests for `hosts tags delete` -- missing `--yes` flag rejected
- [ ] implement `newTagsUpdateCmd` -- call `UpdateHostTags` with tag list and optional source
- [ ] implement `newTagsDeleteCmd` -- require `--yes`, call `DeleteHostTags`
- [ ] run `make build` -- must pass

### Task 8: Verify acceptance criteria
- [ ] verify all host operations: list, totals, mute, unmute
- [ ] verify all tag operations: list, show, create, update, delete
- [ ] verify edge cases: empty results, missing flags, `--yes` confirmation, source filtering
- [ ] run full test suite: `go test -race ./...`
- [ ] run linter: `make build`

### Task 9: [Final] Update documentation
- [ ] update CLAUDE.md API reference section with Hosts/Tags APIs
- [ ] update README.md with hosts command examples

## Technical Details
- **Host fields**: name, id, aliases, apps (list of integrations), sources, up (bool), last_reported_time, meta (platform, agent_version)
- **Tags format**: `key:value` pairs, comma-separated for flags
- **Source**: tag source identifier (e.g., `users`, `datadog`, `chef`) -- optional filter
- **Mute options**: `end` (Unix timestamp), `message`, `override` (bool, override existing mute)
- **Pagination**: `count` + `start` for host listing

## Post-Completion
- Manual testing with real Datadog account
- Verify host listing with various filters
- Test tag operations with different sources
- Verify mute/unmute behavior on real hosts
