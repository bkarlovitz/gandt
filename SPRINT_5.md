# Sprint 5: Cache Controls - Inspect, Tune, Purge, And Retain Data

## Objective
Users can see what is cached, change cache policy by account and label, exclude sensitive senders/domains/labels, purge data safely, and run retention enforcement.

## Source Context
Grounded in `prd.md` sections 7, 10, 11, 16, 17, 18, 20, and milestone M2.

## Tasks
- [x] **Task 5.1**: Add cache size and count queries
  - Compute cache size by account, label, message age, attachment footprint, row counts, and FTS table footprint.
  - Keep dashboard queries fast enough for a 1 GB cache.
  - Expose query results through typed service structs.
  - Validation: `go test ./internal/cache/...` covers count/size calculations with fixture databases.

- [x] **Task 5.2**: Build the `:cache` dashboard
  - Render size, count, age, attachment, and policy summaries in a TUI table.
  - Show sync policy indicators consistently with the label sidebar.
  - Keep dashboard render target below 200ms for a large fixture database.
  - Validation: `go test ./internal/ui/...` includes dashboard snapshots and a benchmark for render time.

- [x] **Task 5.3**: Implement cache policy editing services
  - Add create/update/delete operations for explicit policy rows.
  - Validate depth, retention, attachment rule, and attachment max combinations.
  - Recompute effective policy for labels after updates.
  - Validation: `go test ./internal/cache ./internal/sync` covers policy edits, invalid combinations, and effective policy changes.

- [x] **Task 5.4**: Build the `:cache-policy` table editor
  - Show account-by-label policies with editable depth, retention, attachment rule, and attachment size.
  - Support cancel, save, and reset-to-default actions.
  - Trigger a sync decision refresh after saving policies.
  - Validation: `go test ./internal/ui/...` simulates edit/save/cancel/reset and verifies persisted rows.

- [x] **Task 5.5**: Apply config-file cache policies
  - Parse `[cache.defaults]`, `[[cache.policies]]`, and `[[cache.exclusions]]` from config.
  - Implement documented precedence: explicit DB row, config policy, account default, global default.
  - Reload policy safely when config is changed between launches.
  - Validation: `go test ./internal/config ./internal/sync` covers precedence and relaunch behavior.

- [x] **Task 5.6**: Implement cache exclusions
  - Add commands and services for sender, domain, and label exclusions.
  - Ensure sync fetches excluded message metadata transiently only where needed and never writes excluded bodies.
  - Purge already-cached rows that newly match an exclusion after user confirmation.
  - Validation: `go test ./internal/sync ./internal/cache` verifies sender/domain/label matches, no persisted excluded bodies, and confirmed purge.

- [x] **Task 5.7**: Add `:cache-exclude` command flow
  - Parse `:cache-exclude <sender|domain|label> <value>`.
  - Preview how many cached rows would be purged before committing.
  - Show a clear success or cancel state.
  - Validation: `go test ./internal/ui/...` covers valid command, invalid match type, preview, confirm, and cancel.

- [x] **Task 5.8**: Implement purge planning and dry run
  - Support `:cache-purge --account --label --older-than --from --dry-run`.
  - Return counts and estimated bytes before any destructive operation.
  - Do not remove account registry or keyring tokens in cache purge flows.
  - Validation: `go test ./internal/cache/...` covers purge plan filters and dry-run non-mutation.

- [x] **Task 5.9**: Implement confirmed purge, attachment cleanup, and compact
  - Delete selected message rows, child rows, and on-disk attachments.
  - Run WAL checkpoint after prune and expose `:cache-compact` for `VACUUM`.
  - Keep deletion failures recoverable and logged.
  - Validation: `go test ./internal/cache/...` covers purge mutation, attachment path deletion, WAL checkpoint invocation, and compact command.

- [x] **Task 5.10**: Implement retention sweep
  - Run retention once per day and on startup.
  - Prune rows only when a message is outside the retention window across all labels' effective policies.
  - Respect exclusions and attachment cleanup.
  - Validation: `go test ./internal/sync ./internal/cache` covers single-label, multi-label, no-limit, and startup sweep scenarios.

- [ ] **Task 5.11**: Add `:cache-wipe` with two-step confirmation
  - Delete the SQLite cache and attachment files after explicit confirmation.
  - Preserve OAuth tokens and client credentials unless the user removes accounts separately.
  - Recreate schema cleanly on next startup.
  - Validation: `go test ./internal/ui ./internal/cache` covers confirmation, cancel, wipe, and remigrate behavior.

- [ ] **Task 5.12**: Update schema and cache-control docs
  - Keep `docs/schema.md` current with any query or policy semantics added during this sprint.
  - Add README sections for cache location, privacy controls, purge commands, and at-rest encryption non-goal.
  - Include examples for cache policies and exclusions.
  - Validation: manual review confirms docs match command parser behavior and PRD security claims.

- [ ] **Task 5.13**: Verify the full M2 acceptance path
  - Adjust a policy and observe a changed fetch decision on next sync.
  - Add an exclusion and confirm matching cached rows are purged.
  - Run purge and compact and confirm disk usage is reclaimed.
  - Validation: manual QA confirms the M2 cache-control acceptance criteria from `prd.md`.
