# Contributing to kan

## Development setup

kan requires Go 1.25 or newer. Clone the repository, then run:

```sh
make check
./bin/kan --db /tmp/kan-dev.db seed
./bin/kan --db /tmp/kan-dev.db
```

Use `CGO_ENABLED=0` for builds and tests. Keep domain code independent of Bubble
Tea and SQL, and route TUI storage operations through `tea.Cmd` messages.

## Changes

1. Open an issue for substantial behavioral or database changes.
2. Keep changes focused and develop them test-first (see below).
3. Run `make check` before opening a pull request.
4. Document user-visible commands, keys, and configuration.

## Test-driven development

kan is developed test-first. Every behavior change follows the
red → green → refactor cycle:

1. **Red.** Write the smallest test that describes the next piece of behavior
   and run it to watch it fail for the expected reason. A test that passes
   before the change does not prove anything.
2. **Green.** Write the simplest production code that makes the test pass.
3. **Refactor.** Clean up the code and the test while the suite stays green.

Bug fixes start with a regression test that reproduces the bug. Refactorings
that do not change behavior rely on the existing tests; add tests first when
the code you are about to move is not covered.

Write the test at the lowest layer that owns the behavior:

| Layer | Package | How to test |
|---|---|---|
| Validation and models | `internal/domain` | Table-driven unit tests |
| Storage and migrations | `internal/storage/sqlite` | Real SQLite in `t.TempDir()` or an in-memory database |
| Serialized mutations | `internal/tasks` | Coordinator over a real repository; concurrency under `make test-race` |
| Model tools | `internal/mcpserver` | MCP client and server over `mcp.NewInMemoryTransports()` |
| CLI commands | `internal/cli` | Execute the Cobra command and decode its JSON output |
| TUI behavior | `internal/app` | Send key and mouse messages to the model; use `teatest` for full-program flows |

Useful targets:

```sh
make test         # fast test run
make test-race    # tests with the race detector (requires CGO)
make cover        # coverage profile and total
make cover-check  # fail when total coverage drops below COVERAGE_MIN
```

`make check` and CI enforce the coverage gate. When your change raises total
coverage, raise `COVERAGE_MIN` in the `Makefile` in the same pull request so
coverage only moves up.

Database changes require an embedded forward migration. Never rewrite a migration
that may have shipped in a release.

## Commit messages

Use short imperative subjects. Conventional prefixes such as `feat:`, `fix:`,
`docs:`, and `test:` are useful because release notes filter maintenance commits.
