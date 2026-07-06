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
2. Keep changes focused and add tests at the affected layer.
3. Run `make check` before opening a pull request.
4. Document user-visible commands, keys, and configuration.

Database changes require an embedded forward migration. Never rewrite a migration
that may have shipped in a release.

## Commit messages

Use short imperative subjects. Conventional prefixes such as `feat:`, `fix:`,
`docs:`, and `test:` are useful because release notes filter maintenance commits.
