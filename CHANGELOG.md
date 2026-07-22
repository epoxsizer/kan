# Changelog

All notable changes to this project will be documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.18] - 2026-07-22

### Changed

- Replaced the mixed global fuzzy-search command palette with hierarchical
  `card`, `column`, `board`, `project`, `settings`, and `view` action menus.
  Card search now stays in the current board's `/` filter.

## [0.1.17] - 2026-07-11

### Added

- Optional conflict-safe S3 JSON synchronization with configurable scheduling,
  startup and shutdown reconciliation, ETag protection, and manual recovery
  commands.

### Changed

- Scheduled synchronization now shares the TUI and MCP mutation coordinator,
  preventing remote imports and snapshots from racing local edits.

## [0.1.16] - 2026-07-08

### Added

- Board planning views for today, overdue, blocked, stale, and untriaged cards,
  plus status-line board health summaries.
- Board-only card templates with TUI commands to create templates, save the
  selected card as a template, list templates, and create cards from templates.
- Markdown task-list import from the card description editor into checklist
  items with `Ctrl-T`.

## [0.1.15] - 2026-07-06

### Added

- Optional authenticated local MCP server for pairing Codex, Claude Code, and
  other Streamable HTTP clients with the running TUI.
- Safe model tools for project/board discovery and card listing, search,
  creation, patching, movement, archival, and restoration.

### Changed

- TUI and MCP writes now share a mutation coordinator, card edits use
  optimistic concurrency, and successful MCP changes refresh the active TUI.
- The minimum supported Go version is now 1.25 for the official MCP Go SDK.

## [0.1.14] - 2026-07-03

### Changed

- Selecting a board card now changes only its background by default; optional aligned metadata can be enabled with `show_selected_card_details`.
- The default `config.toml` is now created beside the executable; local backup retention is fixed at 14 days and no longer exposed in configuration.

### Removed

- All S3 integration, including backup upload, JSON synchronization, sync-state files, configuration, and CLI commands. Kan now stores its database and backups locally only.

## [0.1.13] - 2026-07-03

### Changed

- Selection now uses the prototype's light-blue accent consistently: double borders identify the focused container, while a filled blue background identifies the selected card, row, or control.
- Selected board cards now keep a compact two-line layout with core metadata; descriptions remain in the detail window.

## [0.1.12] - 2026-07-03

### Added

- Persistent board-column reordering with `Shift-Left`/`Shift-Right` and command-palette actions.

### Fixed

- Clipboard paste now normalizes line endings, strips terminal control sequences, preserves Markdown structure, and wraps wide Unicode without corrupting the TUI.

## [0.1.11] - 2026-07-02

### Added

- Markdown card descriptions with responsive edit/preview panes, terminal rendering, search, undo/redo, smart list editing, and external-editor compatibility.

### Fixed

- Stabilized due-date metadata tests so they do not fail after a fixed calendar date.

## [0.1.10] - 2026-07-02

### Added

- Discoverable column settings through `Shift+E` and `:column-settings` for editing the selected column name, WIP limit, and automatic archiving.

## [0.1.9] - 2026-07-01

### Added

- Authenticated self-upgrades for private GitHub repositories and `kan upgrade check` command syntax.

## [0.1.8] - 2026-07-01

### Added

- Configurable local and S3 backup rotation with a 14-day default retention period.
- Dedicated `:filter` card search with ranked fuzzy matching across card content and metadata.

## [0.1.7] - 2026-07-01

### Added

- Explicit card archive, archived-list, and restore workflows in the CLI, plus command-menu archival, an archived-card board view, and configurable automatic archival for active columns in the TUI.

## [0.1.6] - 2026-07-01

### Added

- `kan upgrade` installs the latest stable public GitHub release after SHA-256 verification, while `kan upgrade --check` only reports availability.
- TUI startup checks for new versions asynchronously and caches successful checks for 24 hours.
- Compact centered detail windows for projects, boards, columns, and cards, with `Shift+E` full-screen expansion.
- Mouse navigation for lists, boards, forms, controls, dialogs, and detail scrolling.

### Changed

- Release archives now use predictable lowercase operating-system and Go architecture names for cross-platform self-upgrades.
- Selections now use color without textual arrow or active-column markers, and compact forms keep the workspace visible behind them.
- New TUI and CLI cards have no due date unless one is explicitly selected.

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

[Unreleased]: https://github.com/epoxsizer/kan/compare/v0.1.18...main
[0.1.18]: https://github.com/epoxsizer/kan/compare/v0.1.17...v0.1.18
[0.1.17]: https://github.com/epoxsizer/kan/compare/v0.1.16...v0.1.17
[0.1.8]: https://github.com/epoxsizer/kan/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/epoxsizer/kan/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/epoxsizer/kan/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/epoxsizer/kan/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/epoxsizer/kan/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/epoxsizer/kan/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/epoxsizer/kan/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/epoxsizer/kan/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/epoxsizer/kan/releases/tag/v0.1.0
