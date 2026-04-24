# G&T

G&T is a terminal-native Gmail client for keyboard-driven inbox triage. The binary is named `gandt`; the ampersand is display-only and is not used in paths, commands, or identifiers.

This repository has completed the Sprint 2 implementation for the M1
single-account foundation: the app can configure bring-your-own Google OAuth
client credentials, run a loopback OAuth account-add flow, initialize the
SQLite cache, and render cached account and label state. The real Gmail
single-account bootstrap demo still requires manual QA with a test Gmail
account and Google Desktop OAuth client credentials.

## Platforms

- macOS
- Linux
- Windows best-effort

G&T builds as a single Go binary with `CGO_ENABLED=0`. Go 1.25 or newer is required.

## Gmail Access

Gmail integration will use the Gmail API directly, not IMAP. Users must bring their own Google OAuth client credentials; shared hosted credentials are out of scope for v1. Tokens and client credentials will be stored in the OS keychain, not in `config.toml`.

No OAuth credentials or Gmail account are required to launch the TUI and inspect
the no-account/fake-inbox states. Adding a Gmail account with `:add-account`
requires Google Desktop OAuth client credentials and browser authorization. Use
`:replace-credentials` to replace the stored OAuth client credentials.

## Local Development

```sh
go mod download
make fmt
make test
make vet
make build
make run
```

Useful direct commands:

```sh
go run ./cmd/gandt --version
CGO_ENABLED=0 go test ./...
```

## Local Files

G&T honors XDG overrides when present:

- Config: `$XDG_CONFIG_HOME/gandt/config.toml`
- Data: `$XDG_DATA_HOME/gandt/`
- Attachments: `$XDG_DATA_HOME/gandt/attachments/`
- Logs: `$XDG_DATA_HOME/gandt/logs/gandt.log`

Without XDG overrides, Unix-like systems use:

- Config: `~/.config/gandt/config.toml`
- Data: `~/.local/share/gandt/`

On Windows, G&T uses `%APPDATA%\gandt` for config and `%LOCALAPPDATA%\gandt` for data when XDG overrides are not set.

## Configuration

If `config.toml` is missing, G&T uses PRD defaults. Supported top-level sections are `ui`, `sync`, `cache`, `accounts`, `keys`, and `paths`.

Example:

```toml
[ui]
theme = "dark"
compose_editor = "external"
render_mode_default = "plaintext"
render_url_footnotes = true

[sync]
poll_active_seconds = 60
poll_idle_seconds = 300
backfill_limit_per_label = 5000

[cache.defaults]
depth = "full"
retention_days = 90
attachment_rule = "under_size"
attachment_max_mb = 10
total_budget_mb = 2000

[paths]
downloads = "~/Downloads"
```
