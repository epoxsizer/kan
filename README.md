# kan

[![CI](https://github.com/epoxsizer/kan/actions/workflows/ci.yml/badge.svg)](https://github.com/epoxsizer/kan/actions/workflows/ci.yml)
[![Release](https://github.com/epoxsizer/kan/actions/workflows/release.yml/badge.svg)](https://github.com/epoxsizer/kan/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`kan` is a local-first task tracker with a terminal interface. It stores data in
SQLite and does not require a server, account, or network connection.

Data hierarchy:

```text
Project -> Board -> Column -> Card
```

The app includes full-text search, tags, priorities, due dates, Markdown card
descriptions, checklists, custom fields, linked cards, JSON import/export,
automatic local backups, and optional S3 JSON synchronization.

Current version: `0.1.17`.

## Interface

![kan board screen with colored columns and cards](docs/kan-board.svg)

The interface adapts to terminal size. The active column and selected card are
highlighted, and contextual key hints are shown at the bottom of the screen.
Selecting a card changes only its background by default, without changing its
text, indentation, or height. Set `show_selected_card_details = true` to add an
aligned second line with priority, due information, checklist progress, tags,
and related-card count. Markdown descriptions remain in the scrollable detail
window. Board lists summarize overdue or nearest-due work.

## Quick Start From Source

Go 1.25 or newer is required.

```sh
git clone https://github.com/epoxsizer/kan.git
cd kan
make build
./bin/kan seed
./bin/kan
```

The `seed` command creates deterministic demo projects, boards, columns, and
cards. It is idempotent, so it can be run repeatedly without duplicating data.

To start with an empty database:

```sh
make build
./bin/kan migrate
./bin/kan
```

## Install A Release

Download the archive for Linux, macOS, or Windows from
[GitHub Releases](https://github.com/epoxsizer/kan/releases), verify it with
`checksums.txt`, and place the `kan` binary in a directory from your `PATH`.

You can also install with Go:

```sh
go install github.com/epoxsizer/kan/cmd/kan@latest
```

Released binaries can check for and install the latest stable public release:

```sh
kan upgrade --check
kan upgrade check
kan upgrade
```

When the TUI starts, `kan` checks for a newer version in the background at most
once every 24 hours. If an update exists, the status line suggests running
`kan upgrade`; startup is never delayed and network failures are only written to
the log. Successful downloads are verified against the release
`checksums.txt` before the current executable is replaced. Development builds
cannot self-upgrade, and protected installation paths must be updated manually
or by the account that owns the executable.

For private GitHub repositories, export `KAN_GITHUB_TOKEN`, `GH_TOKEN`, or
`GITHUB_TOKEN` with repository Contents read access before checking or upgrading:

```sh
export KAN_GITHUB_TOKEN="github_pat_..."
kan upgrade check
```

## Key Bindings

| Key | Action |
|---|---|
| Left click | Select an item; click the selected item again to open it |
| Mouse wheel | Navigate lists/cards or scroll the active detail/control |
| Right click | Back, close, or cancel like `Esc` |
| `h j k l`, arrows | Navigate |
| `Enter`, `d` | Open the selected object in a compact detail window |
| `e` | Edit the selected object or card |
| `Shift-E` | Configure the selected column; toggle compact/full-screen when a detail window is open |
| `a` | Add a card or object on the current screen |
| `D` | Delete with confirmation |
| `H`, `L` | Move the selected card to the previous/next column |
| `Shift-Tab`, `Tab` | Move the selected card between columns |
| `Shift-Left`, `Shift-Right` | Move the selected column left or right |
| `:column move-left`, `:column move-right` | Reorder the selected column from the command menu |
| `J`, `K` | Reorder cards |
| `M` | Choose a destination column |
| `u` | Undo the last successful move or reorder |
| `/` | Search within the current board |
| `:` | Open hierarchical action menus for cards, columns, boards, projects, settings, and views |
| `:card archive` | Archive the selected card with confirmation |
| `:column archive` | Archive every active card in the selected column |
| `:view archived` | Show archived cards for the current board |
| `:column settings` | Configure the selected column |
| `:view today`, `:view overdue`, `:view blocked`, `:view stale`, `:view untriaged` | Show planning views for the current board |
| `:card new-template`, `:card save-template`, `:card from-template`, `:card templates` | Create and use board-only card templates |
| `:view table`, `:view cards` | Choose the project and board list layout |
| `?` | Full help |
| `Esc` | Back or cancel |
| `q`, `Ctrl-C` | Quit |

Mouse support is enabled automatically. Hold `Shift` while dragging when you
need to select terminal text. Card drag-and-drop is not enabled.

New cards have no due date by default. Open the due-date field to choose a date,
or press `x` in the calendar to remove the due date.

Columns can enable automatic archiving and set how many days cards remain in
that column before archival; new columns default to 14 days with automation
disabled. The retention timer resets whenever a card enters a different column.

In forms, use `Tab` to move between fields and `Ctrl-S` to save. Text inputs
support cursor editing with arrows and `Home`/`End`; `Ctrl-W`, `Ctrl-U`, and
`Ctrl-K` delete the previous word, text before the cursor, and text after the
cursor.

Card descriptions support Markdown with rendered headings, emphasis, lists,
task lists, links, tables, and code blocks. In the description editor,
`Ctrl-P` switches between editing and preview; terminals at least 100 columns
wide show both panes. `Ctrl-F` searches, `Ctrl-Z`/`Ctrl-Y` undo and redo,
`Ctrl-T` imports Markdown task-list items into the card checklist while keeping
the original description text, and `Tab`/`Shift-Tab` indent or outdent list
items. Enter continues Markdown lists.

Board-only card templates help create repeated work. Use `:card new-template`
to define one manually, `:card save-template` to copy the selected card into a
template, `:card from-template` to create a card from a template, and
`:card templates` to inspect available templates on the current board.

`Ctrl-G` opens the command configured by `$VISUAL` or `$EDITOR` (for example,
`EDITOR="code --wait"`). External edits return to the description buffer and
still require `Ctrl-S` to apply and save.

## CLI Commands

The app can be used without the TUI from shell scripts, CI jobs, and automation
agents. Successful data-management commands write JSON to standard output.
Names and titles containing spaces must be quoted in the shell.

```sh
kan project list
kan project create --name "New project" --comment "Project notes"
kan board list --project PROJECT_ID
kan board create --project PROJECT_ID --name "Development"
kan column create --board BOARD_ID --name "In Progress"
kan card create --board BOARD_ID --column COLUMN_ID --title "Prepare release"
kan card search --board BOARD_ID --query "release"
kan card update CARD_ID --priority high
kan card archive CARD_ID
kan card archived --board BOARD_ID --column COLUMN_ID
kan card restore CARD_ID
kan card delete CARD_ID --yes
```

For the full command reference:

```sh
kan --help
kan card --help
kan card create --help
```

Do not run separate CLI write commands at the same time as the TUI. Use the
built-in MCP server described below for supported model-driven changes while
the TUI is running.

## Pair With A Model Over MCP

Kan includes an optional local
[Model Context Protocol](https://modelcontextprotocol.io/) server so a model can
inspect and update the same task board while you work in the TUI. It uses
Streamable HTTP at `http://127.0.0.1:7337/mcp`, requires bearer authentication,
and only accepts a literal loopback bind address.

Generate a token, export it, and enable MCP in the `config.toml` beside the Kan
executable:

```sh
export KAN_MCP_TOKEN="$(openssl rand -hex 32)"
```

```toml
[mcp]
enabled = true
address = "127.0.0.1:7337"
token = "" # KAN_MCP_TOKEN takes precedence
```

Start `kan` before connecting the model. Invalid MCP configuration or a bind
failure stops startup instead of silently running without the requested server.

For Codex:

```sh
codex mcp add kan \
  --url http://127.0.0.1:7337/mcp \
  --bearer-token-env-var KAN_MCP_TOKEN
```

For Claude Code, use a project `.mcp.json` so the token remains in the
environment:

```json
{
  "mcpServers": {
    "kan": {
      "type": "http",
      "url": "http://127.0.0.1:7337/mcp",
      "headers": {
        "Authorization": "Bearer ${KAN_MCP_TOKEN}"
      }
    }
  }
}
```

The server exposes discovery, listing, search, create, patch, move, archive,
and restore tools for cards. It deliberately does not expose permanent deletion
or project, board, column, and custom-field mutation. MCP writes are serialized
with TUI writes, and the active TUI reloads immediately after a successful
model change. If both sides edit the same card, stale updates are rejected
instead of overwriting newer data.

Configuration references:

- [Codex MCP documentation](https://developers.openai.com/codex/mcp)
- [Claude Code MCP documentation](https://docs.anthropic.com/en/docs/claude-code/mcp)
- [Kan configuration example](docs/config.example.toml)

## Import, Export, And Backups

```sh
kan backup
kan backup before-upgrade
kan export --out kan-export.json
kan import kan-export.json
```

Manual and automatic backups are stored in `backup/` relative to the current
working directory. While the TUI is running, an automatic backup is created about
every six hours. Timestamped backups older than 14 days are removed from this
directory. Backups are always local and their 14-day retention is fixed.

## S3 JSON Sync

Kan can synchronize its complete portable JSON export with one S3 or
S3-compatible object. Synchronization runs only while the TUI is open: Kan
reconciles at startup, every configured interval, and once during clean
shutdown. Configure it beside the executable in `config.toml`:

```toml
[sync]
enabled = true
interval = "10m"
object_key = "kan/sync.json"

[sync.s3]
bucket = "kan-sync"
region = "us-east-1"
endpoint = "https://s3.example.com" # optional for AWS S3
access_key_id = "replace-me"
secret_access_key = "replace-me"
force_path_style = false
```

The configured object is a `kan export` document and can be restored after
downloading it with `kan import`. Keep credentials in a private `config.toml`
(Kan creates it with mode `0600`) and never commit them.

Two Kan installations can safely share the object. Kan records the S3 ETag and
a hash of the last synchronized export in `<database>.sync-state.json`, then
uses conditional uploads to prevent silent overwrites. If only one side changed,
Kan uploads or imports that change; before a non-empty local database is
replaced it writes a local `kan-pre-sync-*` backup. If both sides changed,
the remote object was deleted, or the remote export is invalid, synchronization
pauses and the TUI continues locally. Temporary S3 outages are retried on the
next interval.

Close the TUI before using the manual recovery commands because Kan protects
the database with an exclusive process lock:

```sh
kan sync status
kan sync pull --yes
kan sync push
kan sync push --force --yes
```

`sync status`, `sync pull`, and `sync push` write JSON to standard output.
`sync pull --yes` explicitly replaces local data; `sync push --force --yes` is
the only command that intentionally overwrites a changed remote object.

## Data Paths

By default, `kan` creates and loads `config.toml` from the directory containing
the executable. The database and log paths remain relative to the current
working directory unless explicitly configured.

- config: `<executable-directory>/config.toml`
- database: `./kan.db`
- log file: `./kan.log`

Paths can be overridden with `--config`, `--db`, `--log`, or with `KAN_CONFIG`,
`KAN_DB`, and `KAN_LOG`. `KAN_MCP_TOKEN` overrides `mcp.token`.

An example configuration file is available at
[`docs/config.example.toml`](docs/config.example.toml).

The theme section supports detailed color overrides for text, panels, selected
cards, status bars, help popups, command text, and columns. New configurations
use the prototype's light-blue `#4C8DFF` for focused borders and selected
columns, cards, rows, and controls. Existing explicit theme overrides remain
unchanged. `show_selected_card_details = false` keeps selected board cards at
their normal one-line height; set it to `true` for the optional metadata line.

The planning section controls board health and planning views. By default,
cards are stale after 7 days without updates, blocked cards are detected by the
`blocked` or `blocker` tag, and untriaged cards are active cards without both a
priority and due date.

## Development

```sh
make fmt         # format code
make test        # run tests
make check       # format check, go vet, tests, and build
make build       # build bin/kan
make cross-build # build Linux, macOS, and Windows binaries for amd64/arm64
```

Builds use `CGO_ENABLED=0` and a pure-Go SQLite driver.

The project is distributed under the [MIT License](LICENSE). Contribution rules
are in [CONTRIBUTING.md](CONTRIBUTING.md), and vulnerability reporting guidance
is in [SECURITY.md](SECURITY.md).
