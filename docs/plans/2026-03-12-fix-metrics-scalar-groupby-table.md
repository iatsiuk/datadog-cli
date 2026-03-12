# Fix metrics scalar group-by table output

## Overview

`datadog-cli metrics scalar --group-by service` shows empty NAME column in table mode. JSON output works correctly. The bug is in `internal/cmd/metrics.go` -- the table rendering code only handles `DataScalarColumn` (type: "number") and skips `GroupScalarColumn` (type: "group") via `if dc == nil { continue }`.

## Context

- Bug location: `internal/cmd/metrics.go:304-322` (newMetricsScalarCmd table rendering)
- Working references: `internal/cmd/apm.go` (aggregate), `internal/cmd/logs.go` (aggregate) -- both handle group-by correctly
- API types: `datadogV2.ScalarColumn` is a union of `GroupScalarColumn` (name + `[][]string` values) and `DataScalarColumn` (name + `[]*float64` values)
- Tests: `internal/cmd/metrics_test.go` -- existing `TestMetricsScalarTableOutput` only tests single number column, no group-by coverage

## Development Approach

- **Testing approach**: TDD (tests first)
- Complete each task fully before moving to the next
- Make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
- **CRITICAL: all tests must pass before starting next task** -- no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**

## Testing Strategy

- **Unit tests**: table-driven tests with `testing` package (no testify)
- Mock HTTP server returns JSON with both group and number columns
- Verify table output contains group headers, group values, and numeric values

## Implementation Steps

### Task 1: Add failing test for group-by table output

- [x] add `mockMetricsScalarGroupByResponse` constant in `metrics_test.go` with response containing both `GroupScalarColumn` (type: "group", name: "service", values: [["web"], ["api"]]) and `DataScalarColumn` (type: "number", values: [42.5, 13.7])
- [x] add `TestMetricsScalarGroupByTableOutput` test that sends `--group-by service` flag and verifies table output contains: header "SERVICE", values "web", "api", "42.5", "13.7"
- [x] add `TestMetricsScalarMultiGroupByTableOutput` test with two group columns (service + env) and verify both group headers and values appear
- [x] run tests -- new tests must FAIL (red phase), existing tests must pass

### Task 2: Fix group-by table rendering in metrics scalar

- [ ] refactor table rendering in `metrics.go:304-322`: first pass collects group columns (`GroupScalarColumn`) and data columns (`DataScalarColumn`), second pass builds headers (group names uppercase + "VALUE") and rows by zipping group values with numeric values
- [ ] handle edge case: no group columns (current behavior preserved -- NAME + VALUE)
- [ ] handle edge case: multiple group columns
- [ ] run tests -- all tests must pass (green phase), including new group-by tests from task 1

### Task 3: Verify acceptance criteria

- [ ] verify `TestMetricsScalarTableOutput` still passes (no group-by -- backward compatible)
- [ ] verify `TestMetricsScalarGroupByTableOutput` passes (single group-by)
- [ ] verify `TestMetricsScalarMultiGroupByTableOutput` passes (multiple group-by)
- [ ] verify `TestMetricsScalarJSONOutput` still passes (JSON unaffected)
- [ ] run full test suite (`go test -race ./...`)
- [ ] run linter via `make build`

## Technical Details

**Response structure with group-by:**

```json
{
  "data": {
    "attributes": {
      "columns": [
        {"name": "service", "type": "group", "values": [["web"], ["api"]]},
        {"name": "", "type": "number", "values": [42.5, 13.7]}
      ]
    }
  }
}
```

**Go types:**

- `ScalarColumn.GroupScalarColumn` -- `*GroupScalarColumn` with `Values [][]string` (each value is `[]string` with one element)
- `ScalarColumn.DataScalarColumn` -- `*DataScalarColumn` with `Values []*float64`

**Table output format (after fix):**

Without group-by (unchanged):
```
NAME    VALUE
query1  42.5
```

With `--group-by service`:
```
SERVICE  VALUE
web      42.5
api      13.7
```

With `--group-by service,env`:
```
SERVICE  ENV   VALUE
web      prod  42.5
api      prod  13.7
```

## Post-Completion

**Manual verification:**
- Test against live Datadog with `metrics scalar --query "avg:system.cpu.user{*} by {service}" --from now-15m --to now`
- Compare table output with `--json` output to verify data matches
