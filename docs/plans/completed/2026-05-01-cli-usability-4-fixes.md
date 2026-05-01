# Fix 4 CLI Usability Issues

## Overview
Fix four recurring usability/correctness issues in datadog-cli (v0.7.1) that cause downstream pipeline failures for LLM agents. Identified by analyzing 151 real Bash invocations of `datadog-cli` across local Claude Code transcripts (2026-04-17..2026-05-01, all in project `overgear`).

The four issues:
1. `metrics metadata <metric>` (without `show`) prints help instead of metadata.
2. `--json` flag is ignored on error/help path -- breaks downstream `jq` / `python json.load`.
3. `apm services --env <env> --json` returns `[]string` -- agents expect `[]object`, fail in `jq`.
4. `metrics tags <metric>` help is too terse -- agents write `--metric <name>` by analogy.

Issue 3 is a BREAKING change in JSON schema -- bump version to 0.8.0 (semver minor for pre-1.0).

## Context
- Primary files:
  - `internal/cmd/metrics.go` (lines 528-700: metadata + tags + show)
  - `internal/cmd/apm.go` (lines 484-534: services)
  - `internal/cmd/root.go` (lines 8-31: NewRootCommand)
  - `cmd/datadog-cli/main.go` (error formatting + version)
- Test files:
  - `internal/cmd/metrics_test.go`
  - `internal/cmd/apm_test.go`
  - `internal/cmd/root_test.go`
- Tests use `httptest.NewServer` with mock responses, build root via `cmd.NewRootCommand("test")`, set DD env vars, invoke via `root.SetArgs(...)` + `root.Execute()`, capture out/err with `bytes.Buffer`. Follow this exact pattern.
- SDK: `github.com/DataDog/datadog-api-client-go/v2`
- Cobra: `github.com/spf13/cobra`
- No new external deps; `encoding/json` (stdlib) is sufficient for the JSON error encoder.

## Development Approach
- **Testing approach**: TDD (tests first, then code)
- Each task: write failing test -> implement fix -> verify test passes
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**
- **CRITICAL: update this plan file when scope changes during implementation**

## Testing Strategy
- **Unit tests**: table-driven where applicable, stdlib `testing` only, `t.Parallel()` where safe
- **Build**: `make build` (includes golangci-lint)
- **Full suite**: `go test ./... -race -count=1`

## Progress Tracking
- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with + prefix
- Document issues/blockers with ! prefix

## Implementation Steps

### Task 1: Silence cobra usage and add JSON error formatting
**Root cause**: `main.go` does `fmt.Fprintln(os.Stderr, err)` regardless of `--json`; cobra also prints a Usage block on RunE error, which makes downstream JSON parsers choke on mixed plain-text output.

- [x] write test `TestErrorJSONFormat` in `internal/cmd/root_test.go`: invoke a known-failing command (e.g. `metrics tags --metric foo --json`) through a small helper that mimics `main.main` (calls `root.Execute()` then formats error per `--json`); assert stderr buffer is valid JSON of shape `{"error": "..."}` and contains no `Usage:` block
- [x] write test `TestErrorPlainFormat` in `internal/cmd/root_test.go`: same invocation without `--json`; assert stderr is plain-text starting with the cobra error string and contains no `Usage:` block
- [x] in `internal/cmd/root.go` `NewRootCommand`: set `root.SilenceUsage = true` and `root.SilenceErrors = true`
- [x] in `cmd/datadog-cli/main.go`: after `root.Execute()` returns non-nil, look up `--json` via `root.PersistentFlags().Lookup("json")` (or `GetBool`), and write either `json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})` or plain `fmt.Fprintln(os.Stderr, err)`; always `os.Exit(1)`
- [x] extract the error-formatting branch into a small helper (e.g. `func writeError(w io.Writer, err error, asJSON bool)`) inside `internal/cmd` so the test can drive it directly without spawning a process
- [x] run `go test -race ./internal/cmd/ -run "TestErrorJSONFormat|TestErrorPlainFormat"` -- must pass
- [x] run `make build` -- must pass

### Task 2: Default `metrics metadata <metric>` to `show`
**Root cause**: `newMetricsMetadataCmd` (metrics.go:528) creates a parent group with no `RunE`, so cobra prints help when called with a positional arg.

- [x] write test `TestMetricsMetadataDefaultsToShow` in `internal/cmd/metrics_test.go`: mock V1 metrics-metadata API returning a known payload, run `metrics metadata postgresql.queries.time --json`, assert stdout JSON unmarshals into the metadata object (NOT help text)
- [x] write test `TestMetricsMetadataNoArgsPrintsHelp` in `internal/cmd/metrics_test.go`: run `metrics metadata` with no args, assert exit (returned error == nil) and stdout contains `Available Commands` substring
- [x] in `internal/cmd/metrics.go` `newMetricsMetadataCmd`:
  - change `Use: "metadata"` to `Use: "metadata <metric>"`
  - set `Args: cobra.MaximumNArgs(1)`
  - capture `showCmd := newMetricsMetadataShowCmd(mkAPI)` once and reuse
  - add `RunE`: when `len(args) == 0` -> `return cmd.Help()`; otherwise `return showCmd.RunE(cmd, args)`
  - keep both `show` and `update` subcommands attached
- [x] verify existing tests still pass: `TestMetricsMetadataShowTableOutput`, `TestMetricsMetadataShowJSONOutput`, `TestMetricsMetadataShowRequiresArg`, `TestMetricsMetadataUpdateFlagsParsed`, `TestMetricsMetadataUpdateRequiresArg`
- [x] run `go test -race ./internal/cmd/ -run "TestMetricsMetadata"` -- must pass
- [x] run `make build` -- must pass

### Task 3: Switch `apm services --json` to `[]object` schema (BREAKING)
**Root cause**: `newAPMServicesCmd` (apm.go:520) emits `services []string` directly. Agents and pipelines expect a list of objects with a `name` key.

- [x] write test `TestAPMServicesJSONOutput` in `internal/cmd/apm_test.go`: mock APM API returning a service list, run `apm services --env prod --json`, parse stdout into `[]map[string]string`, assert each element has key `name` with the expected value
- [x] write test `TestAPMServicesJSONEmpty` in `internal/cmd/apm_test.go`: mock API returns empty services slice, run with `--json`, assert stdout is exactly `[]` (after JSON normalisation), not `null`
- [x] in `internal/cmd/apm.go` `newAPMServicesCmd`: replace the `--json` branch to build `items := make([]map[string]string, len(services))` with `items[i] = map[string]string{"name": svc}`, then `output.PrintJSON(cmd.OutOrStdout(), items)`; preserve empty-slice -> `[]` behavior
- [x] keep table output unchanged
- [x] bump version: edit `cmd/datadog-cli/main.go` `var version = "dev"` only if the literal version is hard-coded elsewhere -- run `grep -rn "0\\.7\\.1\\|0\\.7\\.0" .` first; update any matches to `0.8.0` (no hardcoded literals; version comes from git tag via Makefile ldflags)
- [x] verify existing tests still pass: `TestAPMServicesEnvFlag`, `TestAPMServicesTableOutput`, `TestAPMServicesRequiresEnv`
- [x] run `go test -race ./internal/cmd/ -run "TestAPMServices"` -- must pass
- [x] run `make build` -- must pass

### Task 4: Add `Example` to `metrics tags` and `metrics metadata show`
**Root cause**: cobra `Use: "tags <metric>"` alone is too terse; agents extrapolate `--metric <name>` from other CLIs.

- [x] write test `TestMetricsTagsHelpHasExample` in `internal/cmd/metrics_test.go`: run `metrics tags --help`, assert stdout contains substring `metrics tags postgresql`
- [x] write test `TestMetricsMetadataShowHelpHasExample` in `internal/cmd/metrics_test.go`: run `metrics metadata show --help`, assert stdout contains substring `metrics metadata show postgresql`
- [x] in `internal/cmd/metrics.go` `newMetricsTagsCmd`: add `Example: "  datadog-cli metrics tags postgresql.queries.time",` to the cobra.Command literal
- [x] in `internal/cmd/metrics.go` `newMetricsMetadataShowCmd`: add `Example: "  datadog-cli metrics metadata show postgresql.queries.time",` to the cobra.Command literal
- [x] run `go test -race ./internal/cmd/ -run "TestMetricsTagsHelpHasExample|TestMetricsMetadataShowHelpHasExample"` -- must pass
- [x] run `make build` -- must pass

### Task 5: Verify acceptance criteria
- [x] all four issues fixed end-to-end: re-read each task and verify implementation matches
- [x] run full test suite: `go test ./... -race -count=1` -- exit 0
- [x] run linter via `make build` -- exit 0
- [x] sanity-check `--help` for `metrics tags`, `metrics metadata`, `metrics metadata show`, `apm services` -- examples appear where expected, no Usage block leaks on error
- [x] sanity-check error path: `datadog-cli --json metrics tags --metric foo 2>&1 >/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'error' in d"` -- exit 0 (run against built binary)
- [x] verify completion still works: `datadog-cli completion bash | head -20` exits 0

## Technical Details

### Issue 1: `metrics metadata` default subcommand
- New `RunE` on the parent: zero args -> `cmd.Help()`; one arg -> delegate to captured `showCmd.RunE`.
- `Use` becomes `metadata <metric>` for self-documentation.
- `Args: cobra.MaximumNArgs(1)`.
- Behavior with extra/unknown flags: cobra rejects with `unknown flag: --description`; that's the expected signal to use `update`.

### Issue 2: `--json` on error path
- Error JSON shape: `{"error": "<text>"}` -- single field, stderr only, no exit-code field. Parseable as `jq -r .error`.
- `SilenceUsage = true` and `SilenceErrors = true` on root suppresses cobra's auto-printed Usage and error string -- formatting moves to `main.go`.
- Refactor: extract `writeError(w io.Writer, err error, asJSON bool)` into `internal/cmd` so it is testable without `os.Exit`.
- `--help` is still handled by cobra before RunE, so silencing usage/errors does not affect explicit help.

### Issue 3: `apm services --json` schema
- Old: `["web-store", "checkout"]`.
- New: `[{"name":"web-store"},{"name":"checkout"}]`.
- Empty list still serialises as `[]` (NOT `null`).
- Single-key objects keep the door open for future fields (env, last-seen, etc.) without another breaking change.
- Version bump: 0.7.1 -> 0.8.0 (semver minor for pre-1.0 breaking).

### Issue 4: `Example` strings
- cobra `Example` is shown in `--help` between the long description and the flags block.
- Two-space indent matches cobra's house style for example blocks.

## Post-Completion
*Items requiring manual intervention or external systems -- no checkboxes, informational only*

**Release / distribution**:
- Tag and release v0.8.0 from `master` after merge.
- Update Homebrew formula in `iatsiuk/homebrew-tap` to reference 0.8.0.

**External system updates**:
- Update `iatsiuk:datadog-expert` skill docs to:
  - Document `apm services --json` new schema (`[]object` with `name`).
  - Document `metrics metadata <metric>` shorthand.
  - Document `metrics tags <metric>` positional argument.
- Audit other internal consumers of `apm services --json` (if any) for the schema change.

**Manual verification** (only if any concerns surface in CI):
- Run a smoke test against a real DD account: `apm services --env prod --json | jq '.[0].name'`.
- Confirm shell completion files generated by `completion bash|zsh|fish` are unchanged in shape.

## Source
Originated from internal Claude Code transcript audit on 2026-05-01. No external ticket. Branch: `fix/datadog-cli/cli-usability-4-fixes`.
