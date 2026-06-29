# Changelog

All notable changes to this project will be documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.5] - 2026-06-29

### Added

- Optional conflict-safe JSON synchronization with a fixed S3 object, configurable intervals, startup reconciliation, shutdown push, and local pre-import backups.
- `kan sync status`, `kan sync pull --yes`, `kan sync push`, and explicit forced-push commands.
- `$VISUAL` and `$EDITOR` support for editing card comments outside the TUI.
- Cursor-aware text editing shortcuts, a card destination picker, and undo for the last card move or reorder.
- Compact card metadata with relative due dates, checklist progress, tags, links, comment previews, and board due-date health.

### Changed

- Automatic SQLite backups remain local while S3 JSON synchronization is enabled.
- Forms, card movement, and small-terminal behavior now provide clearer keyboard-first feedback.

## [0.1.4] - 2026-06-26

### Changed

- First run now creates `config.toml`, `kan.db`, and `kan.log` in the current working directory when no database path is configured.
- Documentation now describes local working-directory defaults instead of XDG defaults.

## [0.1.3] - 2026-06-26

### Fixed

- Describe view now uses a full-screen scrollable pane for huge comments.
- Comment editing now uses the same bounded full-screen editor for huge text.

## [0.1.2] - 2026-06-24

### Fixed

- Describe popups now wrap full multi-line comments instead of truncating them.

## [0.1.1] - 2026-06-24

### Added

- S3-compatible backup upload while keeping the local SQLite database as the default.
- Expanded theme configuration for text, panels, status bars, help, commands, cards, and columns.
- Larger deterministic demo seed data with multiple projects, boards, columns, and cards.
- Runtime TUI settings from the `:` command palette for layout, card tags, sort, and grouping.
- Overdue due-date markers for cards.

### Changed

- Project and board table views now use the same panel style as board columns.
- TUI action wording now consistently uses Add/Edit/Delete.
- Selected columns default to green and selected cards use clearer inverted styling.

## [0.1.0] - 2026-06-24

### Added

- Local-first project, board, column, and card management in a responsive TUI.
- Card metadata including comments, priority, due dates, tags, links, and checklists.
- Typed board fields and per-card free-form fields.
- Full-text search, fuzzy command palette, sorting, and grouping.
- JSON import/export, manual backup, and periodic automatic backups.
- Theme configuration, inline shortcut help, and accessible empty states.
- Scriptable CLI commands for projects, boards, columns, cards, and data exchange.

[Unreleased]: https://github.com/epoxsizer/kan/compare/v0.1.5...main
[0.1.5]: https://github.com/epoxsizer/kan/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/epoxsizer/kan/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/epoxsizer/kan/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/epoxsizer/kan/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/epoxsizer/kan/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/epoxsizer/kan/releases/tag/v0.1.0
