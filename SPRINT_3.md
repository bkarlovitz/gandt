# Sprint 3: Single-Account Read-Only - Browse And Read Gmail

## Objective
With one authenticated account, `gandt` can backfill policy-selected Gmail metadata and bodies, browse labels and threads from cache, and read plaintext or HTML-converted messages offline.

## Source Context
Grounded in `prd.md` sections 7, 9, 10, 12, 13.2, 13.3, 14, 17, 18, 20, and milestone M1.

## Tasks
- [x] **Task 3.1**: Expand Gmail wrappers for message and thread reads
  - Implement `users.messages.list`, `users.messages.get` for `METADATA` and `FULL`, and `users.threads.get`.
  - Normalize Gmail errors into service-layer errors the UI can present.
  - Keep wrappers interface-based for unit tests and later sync workers.
  - Validation: `go test ./internal/gmail/...` covers request parameters, pagination, metadata parsing, full body parsing, and error mapping with HTTP fixtures.

- [x] **Task 3.2**: Implement message, thread, attachment metadata, and label mapping repositories
  - Add upsert/list/get functions for `threads`, `messages`, `message_labels`, and `attachments`.
  - Preserve Gmail IDs and account IDs in every primary key.
  - Keep JSON fields encoded through typed helpers instead of ad hoc string assembly.
  - Validation: `go test ./internal/cache/...` covers inserts, updates, cascades, label mappings, JSON fields, and FTS trigger updates.

- [ ] **Task 3.3**: Implement policy evaluation for fetch depth
  - Resolve effective policy using explicit DB row, config policy, account default, and global default precedence.
  - Compute maximum depth and longest retention for messages with multiple labels.
  - Respect `cache_exclusions` by returning a never-persist decision.
  - Validation: `go test ./internal/sync/...` covers policy precedence, multi-label maximum depth, retention selection, and exclusion matches.

- [ ] **Task 3.4**: Build first-time backfill orchestration
  - For included labels, page through Gmail message IDs within retention and backfill limits.
  - Batch metadata fetches in groups of 100 where supported by the Gmail client abstraction.
  - Enqueue body fetches for policies with `body` or `full` depth.
  - Validation: `go test ./internal/sync/...` runs fixture backfills for included, excluded, metadata-only, body, and full policies.

- [ ] **Task 3.5**: Persist parsed Gmail metadata
  - Map Gmail headers into from, to, cc, bcc, subject, date, snippet, size, internal date, labels, and thread rows.
  - Avoid writing excluded messages to the persistent message tables.
  - Record `cached_at` only when a body is persisted.
  - Validation: `go test ./internal/sync/...` verifies database rows after fixture sync and confirms excluded messages are absent.

- [ ] **Task 3.6**: Implement MIME body extraction
  - Prefer `text/plain`, fall back to `text/html`, and preserve attachment metadata.
  - Decode Gmail base64url payloads and nested MIME parts.
  - Keep raw HTML nullable and persist only when policy depth allows it.
  - Validation: `go test ./internal/gmail ./internal/render` covers plain-only, html-only, multipart alternative, nested multipart, and attachment cases.

- [ ] **Task 3.7**: Implement terminal body rendering
  - Add `internal/render/html.go`, `quote.go`, and `attachments.go`.
  - Convert HTML to text with link footnotes, image placeholders, and basic table handling.
  - Dim or collapse quoted sections over the configured threshold.
  - Validation: `go test ./internal/render/...` uses golden files for plaintext, HTML newsletters, links, images, tables, and quoted replies.

- [ ] **Task 3.8**: Wire mailbox view to cached data
  - Replace dummy message rows with cached thread/message summaries for the active label.
  - Keep 5,000-message cached list rendering under the PRD target by loading view models efficiently.
  - Show unread, selected row, thread count, cache-state badge, and attachment indicator.
  - Validation: `go test ./internal/ui/...` covers mailbox snapshots and `go test ./internal/cache/...` benchmarks list query performance.

- [ ] **Task 3.9**: Wire reader view to cached and on-demand data
  - Open selected threads with cached body content when available.
  - On cache miss, fetch the full thread through a `tea.Cmd`; cache it only if policy permits.
  - Render headers, body, quote state, and attachment list.
  - Validation: `go test ./internal/ui/...` simulates cached read, cache miss loading, cache miss success, and cache miss error.

- [ ] **Task 3.10**: Add offline read behavior
  - Detect network read failures and surface an offline status.
  - Continue browsing cached labels, messages, and bodies.
  - Mark streamed-but-not-cached messages clearly when policy depth is metadata.
  - Validation: `go test ./internal/ui ./internal/sync` covers offline messages and manual QA reads a previously cached thread with networking disabled.

- [ ] **Task 3.11**: Implement read-only navigation polish
  - Add thread navigation with `J`/`K`, next/previous thread with `N`/`P`, render-mode toggle with `V`, browser open placeholder with `B`, and quote toggle with `z`.
  - Keep unavailable actions disabled or clearly reported until later sprints implement them.
  - Ensure reader/list focus behaves correctly below 80 columns.
  - Validation: `go test ./internal/ui/...` covers key handling in wide and narrow layouts.

- [ ] **Task 3.12**: Verify the M1 acceptance path
  - Add one Gmail test account, backfill default labels, browse Inbox, open a thread, and read message text.
  - Run `sqlite3 cache.db` to verify expected tables, FTS rows, and reserved annotations table.
  - Start the app offline and verify cached messages still render.
  - Validation: manual QA confirms M1 acceptance from `prd.md` and records any slow operations over target.
