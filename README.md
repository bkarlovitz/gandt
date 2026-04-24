# G&T

G&T is a terminal-native Gmail client for keyboard-driven inbox triage. The binary, config directory, and cache paths are named `gandt`; `G&T` is display text only.

G&T uses the Gmail API directly, not IMAP. Users bring their own Google Desktop OAuth client credentials. OAuth client credentials and account tokens are stored in the OS keychain, not in `config.toml`.

## Status

The v1 implementation covers Gmail account setup, policy-aware SQLite caching, multi-account switching, triage actions, online and offline search, compose/reply/forward/send, drafts, attachments, cache controls, render-mode cycling, and release packaging. Real Gmail end-to-end release QA is still required before tagging v1.

## Platforms

- macOS
- Linux
- Windows best-effort

G&T builds as a single static Go binary with `CGO_ENABLED=0`. Go 1.25 or newer is required for local development.

## Installation

From source:

```sh
go install github.com/bkarlovitz/gandt/cmd/gandt@latest
```

Local checkout:

```sh
go mod download
make build
./bin/gandt --version
./bin/gandt
```

Snapshot release artifacts can be built with:

```sh
goreleaser release --snapshot --clean
```

## First Run And OAuth

Launch without accounts to inspect the TUI:

```sh
gandt
```

Add a Gmail account from command mode:

```text
:add-account
```

The account flow asks for Google Desktop OAuth client credentials, opens a browser for consent, stores credentials and tokens in the OS keychain, then backfills Gmail metadata and policy-selected bodies into the local cache.

Use `:replace-credentials` to replace the stored Google OAuth client credentials. If Gmail access is revoked, re-authenticate the account with `:add-account`.

## Local Files

G&T honors XDG overrides when present:

- Config: `$XDG_CONFIG_HOME/gandt/config.toml`
- Data: `$XDG_DATA_HOME/gandt/`
- Cache: `$XDG_DATA_HOME/gandt/cache.db`
- Attachments: `$XDG_DATA_HOME/gandt/attachments/`
- Logs: `$XDG_DATA_HOME/gandt/logs/gandt.log`

Without XDG overrides, Unix-like systems use `~/.config/gandt/` and `~/.local/share/gandt/`. On Windows, G&T uses `%APPDATA%\gandt` for config and `%LOCALAPPDATA%\gandt` for data.

## Configuration

If `config.toml` is missing, G&T uses PRD defaults. Supported top-level sections are `ui`, `sync`, `cache`, `accounts`, `keys`, and `paths`.

```toml
[ui]
theme = "dark"                    # dark | light | auto
compose_editor = "external"       # external | inline
render_mode_default = "plaintext" # plaintext | html2text | raw_html | glamour
render_url_footnotes = true
recent_search_limit = 20

[sync]
poll_active_seconds = 60
poll_idle_seconds = 300
backfill_limit_per_label = 5000

[cache.defaults]
depth = "full"                    # none | metadata | body | full
retention_days = 90
attachment_rule = "under_size"    # none | under_size | all
attachment_max_mb = 10
total_budget_mb = 2000

[[cache.policies]]
account = "me@example.com"
label = "Receipts"
depth = "body"
retention_days = 365
attachment_rule = "none"

[[cache.exclusions]]
account = "me@example.com"
match_type = "domain"             # sender | domain | label
match_value = "private.example"

[accounts.work]
email = "me@example.com"
color = "#4287f5"

[keys]
archive = "e"
trash = "#"

[paths]
downloads = "~/Downloads"
```

Cache policy precedence is explicit DB row, config policy, account default, then global default. Editing a label in `:cache-policy` writes an explicit DB row; resetting it returns to the next configured source.

## Keybindings

Press `?` for the active help overlay. `[keys]` overrides are validated at config load and the help overlay reflects the active keymap.

Common defaults:

- `j/k`, arrows: move
- `g/G`: top/bottom
- `Enter`: open thread
- `Tab`: switch pane in narrow layouts
- `/`: search
- `Ctrl+/`: toggle online/offline search while searching
- `Ctrl+R`: recent searches in search mode; refresh in normal mode
- `c`, `r`, `R`, `f`: compose, reply, reply-all, forward
- `e`, `#`, `!`, `s`, `u`, `m`: archive, trash, spam, star, unread, mute
- `+/-`: add/remove label
- `U`: undo last triage action
- `Ctrl+A`, `1`..`9`: account switcher and direct account jump
- `V`: cycle plaintext, html2text, raw HTML, and Glamour render modes
- `B`: open the selected message in Gmail web UI
- `z`: show or collapse quoted text
- `:`: command mode
- `q`: quit

## Compose

Use `c` for a new message, `r` for reply, `R` for reply-all, and `f` for forward. Compose can use an external `$EDITOR` or inline mode via `ui.compose_editor`. Draft save, send, outbox retry, aliases from Gmail send-as settings, and attachments are implemented for v1.

## Search

`/` starts search. Online mode passes Gmail query syntax to Gmail. Offline mode searches cached messages through SQLite FTS5 and supports the implemented local fields such as `from:`, `to:`, and `subject:`. Recent searches are stored locally per account.

## Cache Controls And Privacy

The SQLite cache is intentionally readable with stock SQLite tooling and is documented in `docs/schema.md`. Message bodies and downloaded attachments are unencrypted local files. Users who need at-rest protection should use OS or disk encryption such as FileVault, LUKS, or BitLocker.

Commands:

- `:cache`
- `:cache-policy`
- `:cache-exclude <sender|domain|label> <value>`
- `:cache-purge --account <email> --label <id> --older-than <days|Nd> --from <sender> --dry-run`
- `:cache-compact`
- `:cache-wipe`

Purge and wipe commands require confirmation and do not remove OAuth tokens. Account removal is the explicit path that can revoke and remove account tokens.

## Multi-Account

Each account has independent cache policies, colors, labels, sync state, and Gmail operations. Use `Ctrl+A` for the account switcher or `1` through `9` to jump to an account. Triage, search, compose, drafts, and cache views route through the active account.

## Troubleshooting

- OAuth revoked: run `:add-account` for the account again.
- Keychain inaccessible: unlock or repair the OS keychain or Secret Service provider, then retry.
- Offline/rate limited: cached mail remains browsable; retry sync/search later.
- Cache corruption: quit G&T and inspect or remove `cache.db`; use `:cache-wipe` when the UI is still recoverable.
- Logs: `$XDG_DATA_HOME/gandt/logs/gandt.log`.

## Known V1 Limitations

- HTML rendering is best effort; newsletters and complex layouts may need `B` to open Gmail web UI.
- Online search and cache-miss thread-open latency require real Gmail QA before tagging v1.
- Inline terminal image rendering is not implemented.
- The cache is not encrypted by G&T.
- Windows support is best-effort.

Explicitly out of scope for v1: filter rules management, vacation responder, signatures editor UI, PGP/S/MIME, calendar/contacts, non-Gmail accounts, daemon mode, SSH hosting, mobile UI, web UI, telemetry/analytics, a formal plugin system, scripting/RPC API, and in-process extension points.

## Local Development

```sh
go mod download
make fmt
make test
make vet
make build
make run
```
