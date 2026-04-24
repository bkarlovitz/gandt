# G&T Cache Schema

The G&T cache lives at `$XDG_DATA_HOME/gandt/cache.db`. It is a plain SQLite
database in WAL mode so the TUI and stock SQLite tools can read it while the app
is running.

The schema is a public interface for local tools. Version `1` is recorded in
`schema_version`; future changes within the same major version must be backward
compatible for existing columns and tables.

## Runtime Pragmas

- `PRAGMA journal_mode = WAL`: enables concurrent readers through SQLite's WAL
  files.
- `PRAGMA foreign_keys = ON`: enforces cascades between account-owned rows.
- `PRAGMA busy_timeout = 5000`: gives the TUI and inspection tools time to wait
  for transient locks.

## Privacy Boundary

Message bodies in `messages.body_plain` and `messages.body_html` are
unencrypted on disk. Users should treat `cache.db` as sensitive local data.

OAuth client credentials and per-account OAuth tokens are never stored in
SQLite. They belong in the OS keychain under the service
`com.<owner>.gandt`.

## Tables

### `schema_version`

Records applied cache schema versions.

Key columns:

- `version`: integer schema version.
- `applied_at`: UTC timestamp when the migration was applied.

### `accounts`

One row per Gmail account.

Keys:

- Primary key: `id`, an opaque UUID string.
- Unique key: `email`.

Columns:

- `display_name`: optional user-visible account name.
- `added_at`: account creation timestamp.
- `last_sync_at`: last completed sync timestamp.
- `history_id`: Gmail history ID used for incremental sync.
- `color`: optional hex color for account badges.

Indexes:

- `idx_accounts_email` on `email`.

### `labels`

Gmail labels for an account.

Keys:

- Primary key: `(account_id, id)`.
- `account_id` references `accounts(id)` with cascade delete.

Columns:

- `id`: Gmail label ID.
- `name`: display name.
- `type`: `system` or `user`.
- `unread` and `total`: Gmail count snapshots.
- `color_bg` and `color_fg`: optional Gmail label colors.

Indexes:

- `idx_labels_type` on `(account_id, type)`.

### `threads`

Cached Gmail thread metadata.

Keys:

- Primary key: `(account_id, id)`.
- `account_id` references `accounts(id)` with cascade delete.

Columns:

- `snippet`: Gmail thread snippet.
- `history_id`: latest known Gmail history ID for the thread.
- `last_message_date`: newest message timestamp in the thread.

Indexes:

- `idx_threads_date` on `(account_id, last_message_date DESC)`.

### `messages`

Cached Gmail message metadata and bodies.

Keys:

- Primary key: `(account_id, id)`.
- `(account_id, thread_id)` references `threads(account_id, id)` with cascade
  delete.

Columns:

- `from_addr`, `to_addrs`, `cc_addrs`, `bcc_addrs`: sender and JSON-encoded
  recipient arrays.
- `subject`, `date`, `snippet`, `size_bytes`: Gmail metadata.
- `body_plain`, `body_html`: nullable cached bodies. These are unencrypted on
  disk.
- `raw_headers`: JSON-encoded header map.
- `internal_date`: Gmail internal date.
- `fetched_full`: `1` when a full body was fetched under policy.
- `cached_at`: body cache timestamp for retention decisions.

Indexes:

- `idx_messages_thread` on `(account_id, thread_id)`.
- `idx_messages_thread_date` on `(account_id, thread_id, internal_date DESC,
  date DESC)`.
- `idx_messages_date` on `(account_id, internal_date DESC)`.
- `idx_messages_cached` on `(account_id, cached_at)`.

### `message_labels`

Many-to-many mapping between messages and labels.

Keys:

- Primary key: `(account_id, message_id, label_id)`.
- `(account_id, message_id)` references `messages(account_id, id)` with cascade
  delete.
- `(account_id, label_id)` references `labels(account_id, id)` with cascade
  delete.

Indexes:

- `idx_msglabels_label` on `(account_id, label_id)`.

### `attachments`

Attachment metadata and optional local file pointers.

Keys:

- Primary key: `(account_id, message_id, part_id)`.
- `(account_id, message_id)` references `messages(account_id, id)` with cascade
  delete.

Columns:

- `part_id`: Gmail MIME part ID.
- `filename`, `mime_type`, `size_bytes`: attachment metadata.
- `attachment_id`: Gmail attachment ID for later fetches.
- `local_path`: nullable path once bytes are cached on disk.

Indexes:

- `idx_attachments_message` on `(account_id, message_id)`.

### `outbox`

Queued outbound messages for retry.

Keys:

- Primary key: `id`.
- `account_id` references `accounts(id)` with cascade delete.

Columns:

- `raw_rfc822`: complete RFC 822 message bytes.
- `queued_at`: queue timestamp.
- `attempts`: retry count.
- `last_error`: last send error.
- `status`: `pending`, `sent`, or `failed`.

Indexes:

- `idx_outbox_account` on `(account_id, status)`.

### `sync_policies`

Policy rows controlling what is cached locally.

Keys:

- Primary key: `(account_id, label_id)`.
- `account_id` references `accounts(id)` with cascade delete.
- `label_id = '*'` is the account default.

Columns:

- `include`: `1` to include, `0` to exclude.
- `depth`: `none`, `metadata`, `body`, or `full`.
- `retention_days`: nullable retention window.
- `attachment_rule`: `none`, `under_size`, or `all`.
- `attachment_max_mb`: threshold for `under_size`.
- `updated_at`: last policy update timestamp.

Indexes:

- `idx_sync_policies_label` on `(account_id, label_id)`.

### `cache_exclusions`

Never-cache rules for sensitive senders, domains, or labels.

Keys:

- Primary key: `(account_id, match_type, match_value)`.
- `account_id` references `accounts(id)` with cascade delete.

Columns:

- `match_type`: `sender`, `domain`, or `label`.
- `match_value`: value matched by the sync layer.
- `created_at`: rule creation timestamp.

Indexes:

- `idx_exclusions_match` on `(account_id, match_type, match_value)`.

## Cache Control Semantics

Policy precedence is:

1. Explicit `sync_policies` row written by `:cache-policy`.
2. Matching `[[cache.policies]]` entry from `config.toml`.
3. Account default row where `label_id = '*'`.
4. Global `[cache.defaults]` values from config defaults.

Config-file policies are not copied into SQLite unless the user edits and saves
that label in `:cache-policy`. Resetting a label policy deletes the explicit row
and returns the effective policy to the next configured source.

Exclusions are account-scoped rows in `cache_exclusions`. `sender` values are
normalized to the email address, `domain` values are lowercase without a leading
`@`, and `label` values match Gmail label IDs. Sync may fetch excluded message
metadata transiently to decide whether a message matches, but excluded message
bodies are not written to `messages`. Confirmed exclusions purge already cached
matching message rows and referenced local attachment files.

Purge planning is non-destructive. `:cache-purge` supports `--account`,
`--label`, `--older-than`, `--from`, and `--dry-run`; it reports candidate
message, body, attachment, and byte counts before any deletion. Confirmed purge
deletes matching rows from `messages`; foreign-key cascades remove child rows in
`message_labels`, `attachments`, `messages_fts`, and annotations. Local
attachment files referenced by `attachments.local_path` are removed before the
rows are deleted. After a purge G&T runs `PRAGMA wal_checkpoint(TRUNCATE)`.
`:cache-compact` runs `VACUUM`.

Retention sweep uses each message's effective policies across all current
labels. A message is pruned only when its timestamp is older than every finite
retention window on its labels. Any label with no retention limit keeps the
message. Messages matching exclusions are pruned during the sweep, with the same
attachment cleanup as purge.

`:cache-wipe` is deliberately broader than purge: after two confirmations it
removes `cache.db`, `cache.db-wal`, `cache.db-shm`, and cached attachment files.
It does not remove account OAuth tokens or client credentials from the OS
keychain.

### `message_annotations`

Reserved namespace for future downstream tools. G&T v1 core creates this table
but does not populate it.

Keys:

- Primary key: `(account_id, message_id, namespace, key)`.
- `(account_id, message_id)` references `messages(account_id, id)` with cascade
  delete.

Columns:

- `namespace`: producer identifier such as `gandt.core` or `user`.
- `key`: annotation key.
- `value`: JSON-encoded value, opaque to G&T core.
- `created_at` and `updated_at`: annotation timestamps.

Indexes:

- `idx_annot_namespace` on `namespace`.

### `recent_searches`

Small account-scoped history of user-entered search queries. Rows store only the
query text, mode, and last-used timestamp; they do not store message bodies or
result snapshots.

Keys:

- Primary key: `(account_id, query, mode)`.
- `account_id` references `accounts(id)` with cascade delete.

Columns:

- `query`: user-entered search text.
- `mode`: search mode, currently `online` or `offline`.
- `last_used`: timestamp used for recency ordering and limit trimming.

Indexes:

- `idx_recent_searches_used` on `(account_id, last_used DESC)`.

## Full-Text Search

`messages_fts` is an FTS5 virtual table populated by triggers on `messages`.
It indexes `subject`, `from_addr`, `to_addrs`, `snippet`, and `body_plain`.
`account_id` and `message_id` are stored as unindexed lookup columns.

Triggers:

- `messages_fts_ai`: inserts an FTS row after a message insert.
- `messages_fts_au`: replaces the FTS row after a message update.
- `messages_fts_ad`: removes the FTS row after a message delete.

The FTS index is for offline cache search. Online search still goes through
Gmail query syntax.
