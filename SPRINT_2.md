# Sprint 2: Auth And Local Data - Add One Account And Inspect The Cache

## Objective
A user can configure Google OAuth client credentials, add one Gmail account through a loopback OAuth flow, initialize the SQLite cache schema, and verify account and label rows on disk.

## Source Context
Grounded in `prd.md` sections 7, 8, 9, 10, 11, 16, 19, 20, and milestone M1.

## Tasks
- [x] **Task 2.1**: Implement SQLite database opening and pragmas
  - Add `internal/cache` with a database opener under `$XDG_DATA_HOME/gandt/cache.db`.
  - Enable WAL mode, foreign keys, and connection settings appropriate for a TUI app.
  - Keep all database access behind repository functions.
  - Validation: `go test ./internal/cache/...` verifies WAL mode, foreign keys, and open/close behavior with a temp database.

- [x] **Task 2.2**: Add schema versioning and migrations
  - Create `internal/cache/schema.sql` with `schema_version` and all v1 public tables from the PRD.
  - Include FTS5 table and triggers, `message_annotations`, `sync_policies`, and `cache_exclusions`.
  - Add migration code that applies schema v1 exactly once.
  - Validation: `go test ./internal/cache/...` migrates an empty database and validates every expected table, trigger, and index exists.

- [x] **Task 2.3**: Document the public cache schema
  - Add `docs/schema.md` describing every table, key, index, WAL mode, FTS behavior, and compatibility promise.
  - Explicitly document that message bodies are unencrypted on disk and OAuth tokens are not in SQLite.
  - Add a note that `message_annotations` is reserved for future downstream tools and not populated by v1 core.
  - Validation: manual review confirms `docs/schema.md` matches `internal/cache/schema.sql`.

- [x] **Task 2.4**: Implement account repository operations
  - Add create, get, list, update sync metadata, and delete operations for `accounts`.
  - Enforce unique email behavior and cascade deletion expectations.
  - Keep account IDs opaque UUID strings.
  - Validation: `go test ./internal/cache/...` covers create/list/get/update/delete and duplicate email errors.

- [x] **Task 2.5**: Implement label and policy repository operations
  - Add upsert/list/delete methods for labels.
  - Add account default and per-label `sync_policies` operations.
  - Seed the default policy from PRD section 11 when an account is created.
  - Validation: `go test ./internal/cache/...` covers seeded defaults, policy lookup fallback, and label upserts.

- [ ] **Task 2.6**: Implement keyring-backed secret storage
  - Add `internal/auth/keyring.go` for client credentials and per-account OAuth tokens.
  - Use service name `com.<owner>.gandt` until the final owner is known.
  - Ensure logs and errors never include token values or client secrets.
  - Validation: `go test ./internal/auth/...` uses an injectable keyring interface and verifies storage keys and redacted errors.

- [ ] **Task 2.7**: Build OAuth client credential setup
  - Add a first-run setup flow for entering Google Desktop OAuth client ID and secret.
  - Store credentials in keyring, not config files or environment variables.
  - Allow replacing credentials through a command-mode action with confirmation.
  - Validation: `go test ./internal/auth/...` covers credential validation and keyring persistence using a fake keyring.

- [ ] **Task 2.8**: Implement loopback OAuth flow
  - Generate a random localhost port, state token, auth URL, and short-lived HTTP callback server.
  - Open the browser with the auth URL and exchange the returned code for an OAuth token.
  - Use `gmail.modify`, `gmail.send`, and `userinfo.email` scopes.
  - Validation: `go test ./internal/auth/...` uses `httptest` to verify state checking, callback handling, success, timeout, and cancellation.

- [ ] **Task 2.9**: Fetch Gmail profile and labels after auth
  - Add a minimal Gmail service wrapper for `users.getProfile` and `users.labels.list`.
  - Store the account email, current history ID, and labels in SQLite.
  - Assign a deterministic account color when the user has not configured one.
  - Validation: `go test ./internal/gmail/...` uses fake HTTP responses for profile and labels, and `go test ./internal/auth ./internal/cache` verifies persisted account bootstrap.

- [ ] **Task 2.10**: Add `:add-account` command UI
  - Wire command mode so `:add-account` runs credential setup if needed, then OAuth, then profile and label bootstrap as async `tea.Cmd` work.
  - Show loading, success, and failure states without blocking the Bubble Tea update loop.
  - Preserve the fake inbox fallback when no account is configured.
  - Validation: `go test ./internal/ui/...` simulates command submission and verifies loading/success/error messages.

- [ ] **Task 2.11**: Add account list rendering
  - Replace the fake header account with real account data after bootstrap.
  - Render labels from SQLite with unread counts and cache-depth indicators where available.
  - Show clear empty states for no accounts, no labels, and auth failure.
  - Validation: `go test ./internal/ui/...` snapshots no-account, bootstrapping, and one-account states.

- [ ] **Task 2.12**: Verify the single-account bootstrap demo
  - Add a real test Gmail account, complete OAuth, and inspect `cache.db` with stock `sqlite3`.
  - Confirm `accounts`, `labels`, `sync_policies`, `messages_fts`, and `message_annotations` exist.
  - Confirm client credentials and OAuth tokens are absent from config files and SQLite.
  - Validation: manual QA records the exact `sqlite3` inspection commands and expected row counts.
