# Sprint 6: Multi-Account - Fast Switching With Per-Account Semantics

## Objective
`gandt` supports three Gmail accounts with instant switching, independent cache policies, per-account colors, and correct account routing for read, sync, and triage operations.

## Source Context
Grounded in `prd.md` sections 1, 3, 4, 7, 8, 10, 12, 13.1, 13.6, 16, 20, and milestone M3.

## Tasks
- [x] **Task 6.1**: Harden multi-account account registry behavior
  - Ensure every service call requires an explicit account ID.
  - Keep account IDs stable across restarts and account email changes reported by Gmail profile.
  - Store non-secret registry data in SQLite and the configured accounts file as required by the PRD.
  - Validation: `go test ./internal/auth ./internal/cache` covers multiple accounts, restart reload, and duplicate email handling.

- [x] **Task 6.2**: Extend account add for repeated accounts
  - Allow `:add-account` to add second and third accounts without disrupting existing sync state.
  - Prevent accidental duplicate authorization of the same email.
  - Start first-time backfill for the new account only.
  - Validation: `go test ./internal/auth ./internal/sync ./internal/ui` covers adding multiple distinct accounts and duplicate account rejection.

- [x] **Task 6.3**: Implement account removal
  - Add `:remove-account` with confirmation.
  - Best-effort revoke the token, delete keyring token, delete account rows, and remove attachments for that account.
  - Leave other accounts and shared client credentials intact.
  - Validation: `go test ./internal/auth ./internal/cache ./internal/ui` covers confirm, cancel, revoke failure, keyring deletion, and cascade cleanup.

- [x] **Task 6.4**: Build the account switcher UI
  - Add `Ctrl+A` switcher overlay and digit shortcuts `1` through `9`.
  - Display email, display name if known, color badge, sync status, and unread summary.
  - Make switching update the active account in under 50ms from cached state.
  - Validation: `go test ./internal/ui/...` simulates switcher navigation and measures cached switch path.

- [x] **Task 6.5**: Apply per-account color and theme accents
  - Tint the frame or selected accent with the active account color.
  - Allow account color override through config.
  - Preserve `NO_COLOR` behavior.
  - Validation: `go test ./internal/ui/...` snapshots default color, configured color, and no-color output.

- [x] **Task 6.6**: Route read operations by active account
  - Ensure label list, mailbox list, reader, body fetch, attachment metadata, and cache queries are scoped by active account.
  - Prevent cross-account leakage in all view models.
  - Show clear empty states when the active account has no cached data yet.
  - Validation: `go test ./internal/ui ./internal/cache` uses two accounts with overlapping Gmail IDs and verifies isolated views.

- [x] **Task 6.7**: Route triage operations by message account
  - Attach account IDs to selected message/thread actions.
  - Prevent actions from applying to the wrong account after a fast account switch.
  - Keep undo scoped to the account and message/thread that created it.
  - Validation: `go test ./internal/ui ./internal/gmail` covers action dispatch before and after account switching.

- [x] **Task 6.8**: Run independent sync loops per account
  - Sync each configured account with separate history ID, rate-limit state, and policy lookup.
  - Avoid one account's network error blocking another account.
  - Render per-account sync status in the switcher and active-account status bar.
  - Validation: `go test ./internal/sync/...` uses fake services to verify independent success, failure, and backoff.

- [x] **Task 6.9**: Honor per-account cache policies
  - Ensure config policies map to account aliases correctly.
  - Allow different retention, depth, attachment, and exclusion rules per account.
  - Show policy differences in the cache dashboard and policy editor.
  - Validation: `go test ./internal/sync ./internal/ui` covers two accounts with conflicting policies and expected fetch decisions.

- [ ] **Task 6.10**: Update commands for account-aware behavior
  - Ensure `:cache`, `:cache-policy`, `:cache-purge`, `:sync-all`, `:add-account`, and `:remove-account` all handle multiple accounts.
  - Add account filter or selector prompts where required.
  - Preserve sensible defaults for active-account commands.
  - Validation: `go test ./internal/ui/...` covers account selection, active-account default, and invalid account cases.

- [ ] **Task 6.11**: Verify multi-account performance and privacy
  - Seed fixture databases for three accounts and verify startup remains below the PRD target.
  - Audit logs and UI errors for unintended account data exposure.
  - Confirm switching does not trigger synchronous network calls.
  - Validation: benchmarks and manual QA notes confirm startup, switch latency, and no cross-account leakage.

- [ ] **Task 6.12**: Verify the M3 acceptance path
  - Add three real Gmail test accounts.
  - Confirm read, sync, triage, cache policies, and switcher behavior work identically for each account.
  - Confirm account switching is instant and per-account cache settings differ.
  - Validation: manual QA confirms all M3 acceptance criteria from `prd.md`.
