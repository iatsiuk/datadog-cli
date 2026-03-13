# Users/Teams -- Access Management

## Overview
- Implement users management: CRUD (list/get/create/update/disable), invitations
- Implement roles management: CRUD (list/get/create/update/delete), grant/revoke permissions
- Implement teams management: CRUD (list/get/create/update/delete), manage members
- Enables CLI-based access control without Datadog UI

## Context
- Existing patterns: `internal/cmd/events.go` -- API wrapper struct, DI via `mkAPI`, cobra subcommands
- APIs: `datadogV2.UsersApi`, `datadogV2.RolesApi`, `datadogV2.TeamsApi`
- Output: `internal/output` package (PrintTable/PrintJSON)
- Tests: httptest mock server pattern, table-driven tests
- Registration: `internal/cmd/root.go` -- `NewUsersCommand()` added to root

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

### Task 1: Users API wrapper and command skeleton with tests
- [x] write tests for `newTestUsersAPI` helper (mock server creation)
- [x] write tests for `NewUsersCommand()` returning correct subcommands
- [x] implement `usersAPI` struct wrapping `datadogV2.UsersApi` in `internal/cmd/users.go`
- [x] implement `defaultUsersAPI()` with config loading and client creation
- [x] implement `NewUsersCommand()` with subcommand registration
- [x] register `NewUsersCommand()` in `root.go`
- [x] run `make build` -- must pass

### Task 2: Users list with tests
- [ ] write tests for `users list` -- table output (id, email, name, handle, status, role, created_at)
- [ ] write tests for `users list` -- JSON output mode
- [ ] write tests for `users list` -- with `--filter` flag (search by email/name)
- [ ] write tests for `users list` -- empty result
- [ ] implement `newUsersListCmd` -- call `ListUsers` with optional filter/sort/pagination
- [ ] run `make build` -- must pass

### Task 3: Users show with tests
- [ ] write tests for `users show --id <id>` -- full user details including orgs and permissions
- [ ] write tests for `users show` -- JSON output
- [ ] write tests for `users show` -- missing required `--id` flag
- [ ] implement `newUsersShowCmd` -- call `GetUser`, display details
- [ ] run `make build` -- must pass

### Task 4: Users create/invite with tests
- [ ] write tests for `users create --email <email> --name <name>` -- capture request body
- [ ] write tests for `users create` -- required flags validation (email)
- [ ] write tests for `users invite --email <email>` -- send invitation
- [ ] implement `newUsersCreateCmd` -- call `CreateUser` with email, name, optional role
- [ ] implement `newUsersInviteCmd` -- call `SendInvitations`
- [ ] run `make build` -- must pass

### Task 5: Users update/disable with tests
- [ ] write tests for `users update --id <id> --name <name>` -- capture request body
- [ ] write tests for `users update` -- only changed fields sent
- [ ] write tests for `users disable --id <id> --yes` -- success
- [ ] write tests for `users disable` -- missing `--yes` flag rejected
- [ ] implement `newUsersUpdateCmd` -- call `UpdateUser`
- [ ] implement `newUsersDisableCmd` -- require `--yes`, call `DisableUser`
- [ ] run `make build` -- must pass

### Task 6: Roles API wrapper and list with tests
- [ ] write tests for `newTestRolesAPI` helper
- [ ] write tests for `users roles list` -- table output (id, name, user_count, created_at)
- [ ] write tests for `users roles list` -- JSON output
- [ ] implement `rolesAPI` struct wrapping `datadogV2.RolesApi`
- [ ] implement `newRolesListCmd` -- call `ListRoles` with optional filter/sort
- [ ] run `make build` -- must pass

### Task 7: Roles show/create with tests
- [ ] write tests for `users roles show --id <id>` -- role details with permissions list
- [ ] write tests for `users roles create --name <name>` -- capture request body
- [ ] write tests for `users roles create` -- required flags validation
- [ ] implement `newRolesShowCmd` -- call `GetRole`
- [ ] implement `newRolesCreateCmd` -- call `CreateRole` with name
- [ ] run `make build` -- must pass

### Task 8: Roles update/delete with tests
- [ ] write tests for `users roles update --id <id> --name <name>`
- [ ] write tests for `users roles delete --id <id> --yes`
- [ ] implement `newRolesUpdateCmd` -- call `UpdateRole`
- [ ] implement `newRolesDeleteCmd` -- require `--yes`, call `DeleteRole`
- [ ] run `make build` -- must pass

### Task 9: Roles permissions grant/revoke with tests
- [ ] write tests for `users roles grant-permission --role-id <id> --permission-id <id>` -- success
- [ ] write tests for `users roles revoke-permission --role-id <id> --permission-id <id> --yes` -- success
- [ ] write tests for `users roles list-permissions --role-id <id>` -- table output
- [ ] implement permission management subcommands -- call `AddPermissionToRole`, `RemovePermissionFromRole`, `ListRolePermissions`
- [ ] run `make build` -- must pass

### Task 10: Teams API wrapper and list with tests
- [ ] write tests for `newTestTeamsAPI` helper
- [ ] write tests for `users teams list` -- table output (id, name, handle, user_count, description)
- [ ] write tests for `users teams list` -- JSON output, with `--filter` flag
- [ ] implement `teamsAPI` struct wrapping `datadogV2.TeamsApi`
- [ ] implement `newTeamsListCmd` -- call `ListTeams` with optional filter/sort
- [ ] run `make build` -- must pass

### Task 11: Teams show/create/update/delete with tests
- [ ] write tests for `users teams show --id <id>` -- team details with member list
- [ ] write tests for `users teams create --name <name> --handle <handle>` -- capture request body
- [ ] write tests for `users teams update --id <id>` -- changed fields only
- [ ] write tests for `users teams delete --id <id> --yes`
- [ ] implement CRUD subcommands for teams
- [ ] run `make build` -- must pass

### Task 12: Team members management with tests
- [ ] write tests for `users teams members --id <id>` -- list team members (user_id, email, role)
- [ ] write tests for `users teams add-member --id <id> --user-id <uid> --role <role>` -- capture request body
- [ ] write tests for `users teams remove-member --id <id> --user-id <uid> --yes`
- [ ] implement member management subcommands -- call `GetTeamMemberships`, `CreateTeamMembership`, `DeleteTeamMembership`
- [ ] run `make build` -- must pass

### Task 13: Verify acceptance criteria
- [ ] verify all user operations: list, show, create, invite, update, disable
- [ ] verify all role operations: list, show, create, update, delete, grant/revoke permissions
- [ ] verify all team operations: list, show, create, update, delete, add/remove members
- [ ] verify edge cases: empty results, missing flags, `--yes` confirmation
- [ ] run full test suite: `go test -race ./...`
- [ ] run linter: `make build`

### Task 14: [Final] Update documentation
- [ ] update CLAUDE.md API reference section with Users/Roles/Teams APIs
- [ ] update README.md with users command examples

## Technical Details
- **User statuses**: `Active`, `Pending`, `Disabled`
- **User roles**: assigned via relationship, not direct field
- **Invitation**: separate API endpoint, sends email to user
- **Disable vs Delete**: Datadog uses `DisableUser` (soft delete), not hard delete
- **Role permissions**: list of permission IDs, grant/revoke individually
- **Team handles**: lowercase, alphanumeric + hyphens, must be unique
- **Team member roles**: `admin` or `member`
- **All APIs are V2**: consistent JSON:API response format with data/attributes/relationships

## Post-Completion
- Manual testing with real Datadog account (use test/sandbox org)
- Verify user creation sends invitation email
- Test role permission management
- Verify team membership operations
