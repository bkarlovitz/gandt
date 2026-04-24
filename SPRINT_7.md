# Sprint 7: Search - Gmail Query Passthrough And Offline FTS

## Objective
Users can run online Gmail searches, toggle to offline FTS search over cached mail, view results in the mailbox pane, and reuse recent searches.

## Source Context
Grounded in `prd.md` sections 7, 9, 13.4, 15, 18, 20, and milestone M4.

## Tasks
- [x] **Task 7.1**: Add search state and mode handling
  - Implement Search mode entered by `/`.
  - Track query text, online/offline mode, active account, loading state, result set, and errors.
  - Exit search cleanly with `Esc` and restore the previous mailbox context.
  - Validation: `go test ./internal/ui/...` covers entering, editing, submitting, toggling, canceling, and exiting search.

- [x] **Task 7.2**: Implement online Gmail search
  - Pass the user query verbatim to `users.messages.list?q=...` for the active account.
  - Fetch enough metadata to render search results consistently with normal mailbox rows.
  - Respect pagination and cancellation when a user starts a new query.
  - Validation: `go test ./internal/gmail ./internal/sync ./internal/ui` covers query parameters, pagination, cancellation, and result rendering.

- [x] **Task 7.3**: Implement offline FTS search parser
  - Support overlapping Gmail-style fields where practical: `from:`, `to:`, `subject:`, and full-text terms.
  - Compile supported terms into safe FTS5 queries using parameters.
  - Report unsupported operators clearly without claiming full Gmail syntax offline.
  - Validation: `go test ./internal/cache/...` covers parser output, parameterization, unsupported operators, and injection-like input.

- [x] **Task 7.4**: Implement FTS result queries
  - Query `messages_fts` and join back to message/thread metadata for display.
  - Scope every query by active account.
  - Target under 100ms for typical cached searches.
  - Validation: `go test ./internal/cache/...` includes fixture searches and a benchmark for representative result sets.

- [ ] **Task 7.5**: Render search results in the message list pane
  - Show `search: <query> [online|offline]` header.
  - Reuse selection, open-thread, cache-state, unread, and attachment indicators from mailbox rows.
  - Preserve keyboard navigation and reader behavior from search results.
  - Validation: `go test ./internal/ui/...` snapshots online results, offline results, empty state, loading, and error states.

- [ ] **Task 7.6**: Add `Ctrl+/` mode toggle
  - Toggle the current search between online and offline mode.
  - Re-run the query in the new mode when a query exists.
  - Default to offline mode automatically when the active account is offline and show that mode clearly.
  - Validation: `go test ./internal/ui/...` covers toggle behavior, rerun behavior, and offline default.

- [ ] **Task 7.7**: Persist recent searches
  - Add a small `recent_searches` schema migration or include it in schema v1 before release, with matching documentation updates.
  - Store recent searches locally per account with query text, mode, and last-used timestamp.
  - Limit stored recents to a small configurable count.
  - Avoid storing message bodies or result snapshots in recents.
  - Validation: `go test ./internal/cache ./internal/config` covers migration, insert, dedupe, ordering, limit, and per-account scoping.

- [ ] **Task 7.8**: Build recent search UI
  - Open recents with `Ctrl+R` from Search mode.
  - Allow selecting, deleting, and re-running a recent query.
  - Preserve mode and active account semantics.
  - Validation: `go test ./internal/ui/...` simulates recent search selection, deletion, and re-run.

- [ ] **Task 7.9**: Integrate search with sync and cache policy
  - For online results, fetch and persist metadata/body only according to effective policy.
  - For excluded messages, allow transient display without writing body content.
  - Ensure offline search only returns data that is already cached.
  - Validation: `go test ./internal/sync ./internal/cache` covers online result persistence decisions and excluded result handling.

- [ ] **Task 7.10**: Verify the M4 acceptance path
  - Run online Gmail queries using `from:`, `to:`, `subject:`, `has:attachment`, `newer_than:`, and `label:`.
  - Run offline cached searches and confirm results return under the PRD target.
  - Confirm recent searches survive restart and remain account-scoped.
  - Validation: manual QA confirms M4 acceptance criteria from `prd.md`.
