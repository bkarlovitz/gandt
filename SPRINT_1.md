# Sprint 1: Foundations - Runnable TUI With Fake Mail

## Objective
`gandt` builds as a Go 1.25+ CLI, opens a Bubble Tea three-pane fake inbox, loads local config defaults, logs to a file, and quits cleanly.

## Source Context
Grounded in `prd.md` sections 5, 6, 12, 15, 16, 20, and milestone M0.

## Tasks
- [x] **Task 1.1**: Scaffold the Go module and command entrypoint
  - Create `go.mod` with module path `github.com/<owner>/gandt` until the final owner is known.
  - Add `cmd/gandt/main.go` that constructs and runs the root Bubble Tea program.
  - Keep root package exports empty for v1 unless a concrete need appears.
  - Validation: `go test ./...` passes and `go run ./cmd/gandt --version` prints an injected or development version.

- [x] **Task 1.2**: Pin the foundation dependencies
  - Add Bubble Tea, Bubbles, Lip Gloss, Glamour, Huh, Charm log, TOML/config dependency, SQLite, sqlx, OAuth2, Gmail API, keyring, html2text, and browser dependencies.
  - Verify the dependency graph supports `CGO_ENABLED=0`.
  - Document any dependency deferred until its sprint if adding all dependencies upfront creates unused-code churn.
  - Validation: `CGO_ENABLED=0 go test ./...` passes.

- [x] **Task 1.3**: Add development build and quality commands
  - Add `Makefile` targets for `fmt`, `test`, `lint` or `vet`, `run`, and `build`.
  - Add a `scripts/` helper only where a Make target would become hard to read.
  - Ensure all commands work on macOS and Linux shells without project-specific absolute paths.
  - Validation: `make fmt test build` passes.

- [x] **Task 1.4**: Configure CI for supported platforms
  - Add GitHub Actions for Linux, macOS, and Windows.
  - Test at least Go 1.25 and the current stable Go version supported by the project.
  - Run formatting, tests, `go vet`, and a static binary build check.
  - Validation: a pull request shows all CI jobs green.

- [x] **Task 1.5**: Implement XDG path resolution
  - Add `internal/config` helpers for `$XDG_CONFIG_HOME/gandt` and `$XDG_DATA_HOME/gandt`.
  - Resolve defaults on macOS, Linux, and best-effort Windows.
  - Create config, data, attachment, and log directories on startup with conservative permissions.
  - Validation: `go test ./internal/config/...` covers XDG env overrides and default paths.

- [x] **Task 1.6**: Implement config loading with PRD defaults
  - Add `config.toml` parsing for `ui`, `sync`, `cache`, `accounts`, `keys`, and `paths`.
  - Apply sane defaults when no config file exists.
  - Validate enum values for render mode, compose editor, cache depth, and attachment rules.
  - Validation: `go test ./internal/config/...` covers missing file, valid config, invalid enum, and override precedence inside the config file.

- [ ] **Task 1.7**: Add file-only logging
  - Initialize Charm log to `$XDG_DATA_HOME/gandt/logs/gandt.log`.
  - Keep stdout/stderr available to the TUI except for explicit CLI commands like `--version`.
  - Include startup metadata without logging secrets or message body content.
  - Validation: `go test ./internal/config/...` covers log path creation, and manual `go run ./cmd/gandt` creates `gandt.log`.

- [ ] **Task 1.8**: Create the root Bubble Tea model
  - Add `internal/ui/app.go`, `msg.go`, `keys.go`, and `styles.go`.
  - Model Normal, Search, Compose, Command, and Help modes even if only Normal and Help are active in this sprint.
  - Handle terminal resize and clean quit via `q`, `Esc`, and `Ctrl+C` where appropriate.
  - Validation: `go test ./internal/ui/...` covers Init, Update quit behavior, and resize handling.

- [ ] **Task 1.9**: Build the fake three-pane mailbox view
  - Add label, message list, and reader panes with dummy account, labels, messages, attachments, and status bar text.
  - Preserve the responsive behavior from the PRD: three panes at wide widths, collapsed labels below 120 columns, list-or-reader below 80 columns.
  - Respect `NO_COLOR` by disabling color styles.
  - Validation: `go test ./internal/ui/...` includes golden snapshots for wide, medium, and narrow layouts.

- [ ] **Task 1.10**: Add keyboard navigation for dummy data
  - Implement `j`/`k`, arrow keys, `g`/`G`, `Enter`, `Tab`, `?`, and `q` against the fake mailbox.
  - Keep keybindings centralized and ready for config overrides.
  - Render the contextual help footer from the keybinding map.
  - Validation: `go test ./internal/ui/...` simulates navigation and verifies selected label/message/reader state.

- [ ] **Task 1.11**: Add a basic README for local development
  - Document the display name `G&T`, binary name `gandt`, supported platforms, and bring-your-own OAuth constraint at a high level.
  - Include local setup commands, test commands, and where config/data/log files are written.
  - Keep user-facing claims aligned with `prd.md`.
  - Validation: manual review confirms commands in README match `Makefile` targets and current code.

- [ ] **Task 1.12**: Verify the M0 demo path
  - Start `gandt`, show the fake inbox, navigate messages, open help, resize the terminal, and quit cleanly.
  - Confirm no network or Gmail credentials are required for the demo.
  - Capture any terminal rendering defects as follow-up issues before moving to Gmail integration.
  - Validation: manual QA checklist passes on at least one macOS or Linux terminal.
