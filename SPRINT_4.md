# Sprint 4: Sync And Triage - Keep Mail Current And Act Quickly

## Objective
`gandt` keeps one account current with policy-aware delta sync and supports optimistic archive, trash, spam, star, unread, label, and mute actions with undo.

## Source Context
Grounded in `prd.md` sections 9, 10, 13.6, 17, 18, 20, and milestone M2.

## Tasks
- [x] **Task 4.1**: Implement `history.list` delta sync
  - Fetch changes from the stored account `history_id`.
  - Process messages added, messages deleted, labels added, and labels removed in order.
  - Update account history ID only after successfully applying the batch.
  - Validation: `go test ./internal/sync/...` covers fixture histories and verifies final database state.

- [x] **Task 4.2**: Handle expired Gmail history windows
  - Detect `historyNotFound` or equivalent 404 behavior.
  - Fall back to policy-aware relist/backfill without refetching cached bodies that are still valid.
  - Surface a status message so the user knows a larger refresh is running.
  - Validation: `go test ./internal/sync/...` covers expired history fallback and preserved cached bodies.

- [x] **Task 4.3**: Add the background sync coordinator
  - Start a long-lived sync loop from `Init()` using active and idle poll intervals from config.
  - Post `SyncUpdateMsg` messages back into the Bubble Tea model.
  - Avoid network calls on the Bubble Tea update path.
  - Validation: `go test ./internal/sync ./internal/ui` uses fake clocks and fake Gmail services to verify active, idle, and stopped behavior.

- [x] **Task 4.4**: Implement manual refresh commands
  - Add `r` for active-account delta sync, `R` for full relist of the current label, and `:sync-all` as a no-op or one-account operation until multi-account support lands.
  - Show sync progress and errors in the status bar and toast area.
  - Debounce repeated refresh keys to avoid overlapping syncs for the same account.
  - Validation: `go test ./internal/ui/...` covers refresh key messages, command submission, progress rendering, and overlap prevention.

- [x] **Task 4.5**: Expand Gmail wrappers for triage actions
  - Implement message batch modify, trash, untrash, thread modify, and label create/delete wrappers needed by v1 actions.
  - Normalize API errors for rate limit, auth, not found, and permission cases.
  - Keep request construction testable without live Gmail.
  - Validation: `go test ./internal/gmail/...` covers action request bodies and error mapping with fixture HTTP responses.

- [x] **Task 4.6**: Implement optimistic action state
  - Apply archive, trash, spam, star, unread, label add/remove, and mute changes immediately in the UI and cache.
  - Dispatch the Gmail API action asynchronously.
  - Revert local state and notify the user if the API call fails.
  - Validation: `go test ./internal/ui ./internal/cache` covers optimistic success and revert-on-error for each action family.

- [ ] **Task 4.7**: Implement undo for the last single action
  - Track enough inverse operation metadata to undo one action within 30 seconds.
  - Support archive, trash, spam, star, unread, label, and mute undo where Gmail API semantics allow it.
  - Expire undo state cleanly after the time window or after a new action supersedes it.
  - Validation: `go test ./internal/ui ./internal/gmail` covers undo success, undo expiration, and unavailable undo messaging.

- [ ] **Task 4.8**: Add label add/remove prompts
  - Prompt for label selection or creation when the user presses `+`.
  - Prompt for removable labels when the user presses `-`.
  - Update cached label mappings and label counts after success.
  - Validation: `go test ./internal/ui/...` covers prompt flows, cancel behavior, and selected-label updates.

- [ ] **Task 4.9**: Implement rate-limit and auth error handling
  - Add exponential backoff for 429 and transient 5xx responses.
  - Surface "re-authenticate <account>" prompts for revoked or expired refresh tokens.
  - Keep non-fatal errors as toasts and fatal cache/keychain errors as clear error screens.
  - Validation: `go test ./internal/gmail ./internal/sync ./internal/ui` covers backoff, retry exhaustion, auth prompt, and fatal error rendering.

- [ ] **Task 4.10**: Add action and sync logging
  - Log sync cycles, action attempts, retries, failures, and durations without message bodies or secrets.
  - Include account ID or email only where useful for local debugging.
  - Ensure log writes do not block the UI.
  - Validation: `go test ./internal/sync/...` verifies structured log events using a test sink.

- [ ] **Task 4.11**: Verify triage performance targets
  - Load a 5,000-message cached inbox and measure list render below 100ms.
  - Confirm perceived latency for optimistic actions is below 50ms.
  - Confirm background sync does not block navigation during network delay.
  - Validation: benchmark output and manual QA notes show whether PRD section 18 targets are met.

- [ ] **Task 4.12**: Verify the first half of M2 acceptance
  - Use a real Gmail test account to archive, trash, star, unread, add label, remove label, mute, and undo actions.
  - Confirm Gmail web UI reflects the changes.
  - Confirm delta sync picks up changes made outside `gandt`.
  - Validation: manual QA confirms triaging a real inbox down to zero feels instantaneous.
