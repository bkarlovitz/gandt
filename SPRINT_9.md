# Sprint 9: Polish And Release - Production-Quality V1

## Objective
`gandt` meets the PRD's usability, error handling, performance, documentation, and release requirements for a v1 build.

## Source Context
Grounded in `prd.md` sections 5, 12, 13.10, 14, 15, 16, 17, 18, 19, 20, 21 M6, 22, and 23.

## Tasks
- [x] **Task 9.1**: Improve HTML rendering quality
  - Refine html2text settings for poor plaintext and HTML-only messages.
  - Add render-mode cycling for plaintext, html2text, raw HTML, and Glamour where applicable.
  - Add `B` open-in-browser behavior for messages that need the Gmail web UI.
  - Validation: `go test ./internal/render ./internal/ui` golden tests cover newsletters, tables, links, images, raw HTML, Glamour, and browser-open fallback.

- [x] **Task 9.2**: Refine themes and visual states
  - Finalize dark, light, and auto themes with semantic roles.
  - Apply unread bolding, read dimming, selected row accent, account color, warning, error, success, and muted states consistently.
  - Respect `NO_COLOR` in every view.
  - Validation: `go test ./internal/ui/...` snapshots key views for dark, light, account accent, and no-color output.

- [x] **Task 9.3**: Complete keybinding configuration
  - Load key overrides from `[keys]`.
  - Detect conflicts and invalid key names with actionable errors.
  - Keep help overlay generated from the active keymap.
  - Validation: `go test ./internal/config ./internal/ui` covers override success, conflicts, invalid keys, and help overlay output.

- [ ] **Task 9.4**: Harden error screens and toasts
  - Review every service error path for clear user-facing copy and local log detail.
  - Keep non-fatal errors as auto-dismissed toasts and fatal errors as recoverable screens where possible.
  - Ensure DB corruption, keychain inaccessible, OAuth revoked, rate-limited, and offline states are distinct.
  - Validation: `go test ./internal/ui ./internal/auth ./internal/cache ./internal/gmail` covers representative fatal and non-fatal errors.

- [ ] **Task 9.5**: Complete performance pass
  - Benchmark cold start, cached inbox render, cached thread open, online search, offline search, triage action latency, memory for 10k cached messages, and binary size.
  - Optimize query indexes, view model construction, and render paths that miss targets.
  - Record results in release notes or a performance doc.
  - Validation: benchmark output meets or explicitly documents deviations from PRD section 18 targets.

- [ ] **Task 9.6**: Add release packaging
  - Configure GoReleaser for static binaries with version injection.
  - Add Homebrew tap metadata placeholders.
  - Verify artifacts for Linux, macOS, and Windows best-effort builds.
  - Validation: `goreleaser release --snapshot --clean` succeeds locally or in CI.

- [ ] **Task 9.7**: Finish user documentation
  - Expand README with installation, first-run OAuth setup, config, cache privacy, keybindings, compose, search, multi-account, and troubleshooting.
  - Document known v1 limitations and out-of-scope items from the PRD.
  - Keep naming conventions clear: display `G&T`, binary/config paths `gandt`.
  - Validation: manual review confirms every documented command exists and out-of-scope claims match `prd.md`.

- [ ] **Task 9.8**: Add operational troubleshooting docs
  - Document log file location, cache inspection with `sqlite3`, keychain troubleshooting, OAuth re-authentication, and cache purge/wipe recovery.
  - Include privacy notes for unencrypted message cache and local-only logs.
  - Avoid suggesting unsupported daemon, plugin, telemetry, or non-Gmail workflows.
  - Validation: manual review against `prd.md` sections 19 and 23.

- [ ] **Task 9.9**: Add manual QA checklist
  - Create a release checklist covering account add/remove, multi-account switching, sync, triage, cache controls, search, compose, drafts, attachments, offline behavior, and error recovery.
  - Include macOS and Linux terminal coverage and Windows best-effort smoke checks.
  - Keep real Gmail account steps isolated from automated CI.
  - Validation: a maintainer can complete the checklist without reading source code.

- [ ] **Task 9.10**: Run end-to-end release QA
  - Execute the manual QA checklist against real test Gmail accounts.
  - Run all automated tests and CI.
  - Fix or file every blocker found before tagging v1.
  - Validation: completed checklist, green CI, and no known release blockers.

- [ ] **Task 9.11**: Review security posture before release
  - Audit logs, config, SQLite, and temp files for secrets or unexpected message body leakage.
  - Confirm OAuth tokens and client credentials are only in keychain.
  - Confirm destructive cache commands require confirmation and do not affect keychain tokens unless account removal is explicit.
  - Validation: manual security review notes confirm PRD section 19 requirements.

- [ ] **Task 9.12**: Verify final M6 and v1 acceptance
  - Confirm all previous sprint demo paths still work after polish.
  - Confirm docs, release artifacts, and performance results are ready for users.
  - Tag unresolved PRD open questions as deferred decisions rather than hidden requirements.
  - Validation: v1 release candidate can be installed, launched, configured, and used for day-to-day Gmail triage on macOS or Linux.
