# Fix E2E Test Errors

## Overview
Fix 2 bugs in CLI code identified during E2E testing:
1. `rum playlist update` -- 409 Conflict (missing ID in update body)
2. `rum app create --type invalid` -- silently defaults to "browser" (no client-side validation)


## Context
- Primary file: `internal/cmd/rum.go`
- Test file: `internal/cmd/rum_test.go`
- Tests use `httptest.Server` with mock responses and factory functions
- SDK: `github.com/DataDog/datadog-api-client-go/v2@v2.56.0`

## Development Approach
- **Testing approach**: TDD (tests first, then code)
- Each task: write failing test -> implement fix -> verify test passes
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**
- **CRITICAL: update this plan file when scope changes during implementation**

## Testing Strategy
- **Unit tests**: table-driven, stdlib `testing` only, `t.Parallel()`
- **Build**: `make build` (includes linter)

## Progress Tracking
- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with + prefix
- Document issues/blockers with ! prefix

## Implementation Steps

### Task 1: Fix playlist update -- set ID in request body
**Root cause**: `newRUMPlaylistUpdateCmd` creates `PlaylistData` without setting `Id`, causing 409 Conflict from the API.

- [x] write test: playlist update sends ID in PUT request body (capture and verify JSON body contains `data.id`)
- [x] write test: playlist update without changed flags returns error
- [x] implement fix in `newRUMPlaylistUpdateCmd`: set `data.SetId(pid)` before sending update
- [x] run `go test -race ./internal/cmd/ -run TestRUMPlaylist` -- must pass
- [x] run `make build` -- must pass

### Task 2: Add client-side validation for rum app --type flag
**Root cause**: `newRUMAppCreateCmd` passes any `--type` value to API without validation; API silently defaults to "browser".

- [ ] write test: `rum app create --type invalid` returns error with list of valid types
- [ ] write test: `rum app create --type browser` succeeds (valid type)
- [ ] write test: `rum app create --type ios` succeeds (valid type)
- [ ] implement validation: check `--type` against allowed values before API call
- [ ] allowed types: browser, ios, android, react-native, flutter, roku, electron, unity, kotlin-multiplatform
- [ ] run `go test -race ./internal/cmd/ -run TestRUMApp` -- must pass
- [ ] run `make build` -- must pass

### Task 3: Verify acceptance criteria
- [ ] verify both bugs are fixed
- [ ] run full test suite: `go test -race ./...`
- [ ] run linter: `make build`
- [ ] update e2e-errors.md with resolution status

## Technical Details

### Bug 1: Playlist Update
- `PlaylistData` has optional `Id *string` field
- Current code at rum.go:1557 creates data without ID
- Fix: `data.SetId(formatPlaylistID(pid))` or equivalent string conversion
- `pid` is `int64` from `parsePlaylistID(args[0])`

### Bug 2: RUM App Type Validation
- Valid types listed in flag help text at rum.go:582
- Add validation before `attrs.SetType(appType)` at rum.go:548-549
- Return error: `fmt.Errorf("invalid --type %q; valid types: browser, ios, android, react-native, flutter, roku, electron, unity, kotlin-multiplatform", appType)`

