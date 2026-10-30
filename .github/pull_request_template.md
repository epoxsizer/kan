## Summary

## Verification

- [ ] Tests were written first and failed before the change (red → green → refactor)
- [ ] Bug fixes include a regression test that reproduces the bug
- [ ] `make check` (format, vet, tests, coverage gate, build)
- [ ] `make test-race` for changes touching concurrency, the coordinator, MCP, or sync

## Notes

Do not include private databases, logs, exports, or screenshots with private task
data.
