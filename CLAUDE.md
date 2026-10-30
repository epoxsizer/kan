# Agent instructions

These rules apply to coding agents (Claude Code, Codex, and others) working in
this repository. Human contributors follow the same process in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Test-driven development is mandatory

For every behavior change:

1. Write or extend a test first and run it. Confirm that it fails, and that it
   fails for the expected reason (not because of a compile error elsewhere or a
   typo).
2. Implement the smallest change that makes it pass.
3. Refactor with the suite green.
4. Finish with `make check`. Run `make test-race` as well when the change
   touches `internal/tasks`, `internal/mcpserver`, sync, or anything with
   goroutines.

Bug fixes start with a failing regression test. Do not weaken, skip, or delete
an existing test to make a change pass. When a test is wrong, say so and fix it
as a separate, explained step.

Pick the lowest layer that owns the behavior (the table in CONTRIBUTING.md lists
the test style for each package). Prefer real SQLite databases in
`t.TempDir()` over mocks.

## Project conventions

- Builds and tests use `CGO_ENABLED=0` (pure-Go SQLite). Only the race detector
  needs CGO.
- Domain code stays independent of Bubble Tea and SQL. TUI storage operations
  run through `tea.Cmd` messages.
- Every write from the TUI, MCP, and sync goes through `tasks.Coordinator`.
- Database changes require a new embedded forward migration; never edit a
  migration that may have shipped.
- Document user-visible commands, keys, and configuration in `README.md` and add
  an entry under `Unreleased` in `CHANGELOG.md`.
- Keep total coverage at or above `COVERAGE_MIN` in the `Makefile`, and raise
  it when your change raises coverage.
