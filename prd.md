# G&T — Product Requirements Document

> Gin & Tonic · Gmail + TUI · a stiff drink for your inbox

**Status:** Draft v0.2
**Project name:** G&T
**Binary / module slug:** `gandt`
**Target platforms:** macOS, Linux (primary); Windows (best-effort)
**Language:** Go 1.25+

### Naming conventions

| Context | Form |
|---------|------|
| Display / docs / README / marketing | **G&T** |
| Binary, CLI invocation | `gandt` |
| Go module path | `github.com/<owner>/gandt` |
| Go package root | `package gandt` |
| Config directory | `$XDG_CONFIG_HOME/gandt/` |
| Data directory | `$XDG_DATA_HOME/gandt/` |
| Keychain service | `com.<owner>.gandt` |
| Log file | `$XDG_DATA_HOME/gandt/logs/gandt.log` |

The ampersand in "G&T" is display-only — it never appears in filesystem paths, identifiers, URLs, or CLI arguments, where it would break shell parsing and URL encoding.

---

## 1. Overview

A terminal-native Gmail client built on the Charm ecosystem, designed for power users who live in the terminal and want a fast, keyboard-driven alternative to the Gmail web UI for day-to-day mail triage, reading, and composition. Supports multiple Gmail accounts as a first-class concept, uses the Gmail API directly (not IMAP), and maintains a local SQLite cache for fast startup and offline reading.

---

## 2. Goals & Non-Goals

### Goals

- **Fast keyboard workflow** — inbox triage in under 10 seconds from launch to first action.
- **Multi-account parity** — every operation (read, compose, search, send) works identically across any configured account, with a fast switcher.
- **Fidelity to Gmail semantics** — threads, labels, search operators, and drafts behave as they do in Gmail (not as IMAP folders).
- **Offline-friendly reading** — cached messages readable without network.
- **Composable** — compose body opens in `$EDITOR` by default for users who want vim/emacs.
- **Production quality** — proper error states, loading indicators, no blocked UI, crash recovery.

### Non-Goals (v1)

- Non-Gmail IMAP accounts (Fastmail, iCloud, corporate Exchange). A future version could abstract over mail providers; v1 is Gmail-only.
- Push notifications via Pub/Sub. Polling is adequate for v1.
- Rich HTML rendering with inline images via Kitty/iTerm graphics protocols. Plaintext-first.
- Calendar, Contacts, or other Google Workspace integrations.
- End-to-end encryption (PGP/S/MIME).
- Running as a daemon or serving the UI over SSH.
- Mobile or web UI.

---

## 3. Users & Use Cases

### Primary user

A developer or operator who manages 2–4 Gmail accounts (personal + work + side projects), spends most of their day in a terminal, and prefers keyboard workflows. Comfortable with mutt/aerc-style keybindings, `$EDITOR`, and config files over GUIs.

### Core use cases

1. **Triage** — open the app, scan the inbox across accounts, archive/delete/label in rapid succession.
2. **Read a thread** — open a conversation, read prior messages in order, jump between thread and inbox.
3. **Reply** — respond to a thread from the correct account with proper quoting and signature.
4. **Compose cold** — send a new message from a chosen account with attachments.
5. **Search** — run a Gmail search query (`from:x has:attachment newer_than:7d`) and act on results.
6. **Account switch** — jump between accounts without restarting.

---

## 4. Scope (v1)

| Area | In Scope | Out of Scope |
|------|----------|--------------|
| Accounts | Multi-account, add/remove, switch, per-account identity for sending | Account impersonation, delegates, aliases beyond `sendAs` |
| Reading | Inbox + all labels, thread view, HTML→text rendering, attachments list and download | Inline image rendering, calendar invites, encrypted mail |
| Actions | Archive, trash, mark read/unread, label add/remove, star, mute thread | Filters management, vacation responder, settings sync |
| Compose | New, reply, reply-all, forward, attachments, drafts synced to Gmail | Templates (future), scheduled send, confidential mode |
| Search | Full Gmail search syntax passthrough | Saved searches as pinned sidebar items (future) |
| Sync | Initial backfill + incremental via `history.list`, policy-driven | Real-time push, CardDAV |
| Local data | Per-account + per-label cache policy, retention windows, size budgets, privacy controls, cache inspection & purge | Encrypted cache at rest, cross-device cache sync |
| Offline | Read cached messages, queue sends | Offline compose with local-only drafts (drafts must round-trip to Gmail) |

---

## 5. Technical Stack

### Core libraries

| Concern | Library |
|---------|---------|
| TUI framework | `github.com/charmbracelet/bubbletea` |
| Components (list, viewport, textarea, textinput, spinner, help, table, progress) | `github.com/charmbracelet/bubbles` |
| Styling / layout | `github.com/charmbracelet/lipgloss` |
| Markdown rendering | `github.com/charmbracelet/glamour` |
| Forms (compose headers, account setup) | `github.com/charmbracelet/huh` |
| Logging (file sink only — TUI owns stderr) | `github.com/charmbracelet/log` |
| Gmail API | `google.golang.org/api/gmail/v1` |
| OAuth2 | `golang.org/x/oauth2` + `golang.org/x/oauth2/google` |
| Token storage | `github.com/zalando/go-keyring` |
| SQLite (pure Go, no CGO) | `modernc.org/sqlite` |
| SQL helpers | `github.com/jmoiron/sqlx` |
| HTML → text | `jaytaylor.com/html2text` |
| Open browser (OAuth) | `github.com/pkg/browser` |
| Config | `github.com/spf13/viper` or hand-rolled TOML via `github.com/BurntSushi/toml` |

### Build & distribution

- Single static binary. `CGO_ENABLED=0` (hence pure-Go SQLite).
- Minimum Go version: 1.25, matching current foundation dependency floors.
- Release via `goreleaser` with Homebrew tap.
- Version injection via `-ldflags`.

---

## 6. System Architecture

### Process model

Single process, single binary. Three logical layers:

1. **UI layer** (Bubble Tea) — all rendering and input handling. Pure, synchronous, single-threaded per Bubble Tea's model.
2. **Service layer** — Gmail client wrapper, cache, sync coordinator. Exposes async operations returning `tea.Cmd`.
3. **Storage layer** — SQLite cache + OS keychain for tokens.

The UI layer never makes a network call on the `Update` thread. Every Gmail/cache interaction is dispatched as a `tea.Cmd` that returns a `tea.Msg` back to `Update`.

### Module layout

```
gandt/
├── cmd/gandt/
│   └── main.go
├── internal/
│   ├── auth/              # OAuth2 flows, token storage, multi-account registry
│   │   ├── oauth.go
│   │   ├── keyring.go
│   │   └── accounts.go
│   ├── gmail/             # Gmail API client wrapper (one per account)
│   │   ├── client.go
│   │   ├── messages.go
│   │   ├── threads.go
│   │   ├── labels.go
│   │   ├── drafts.go
│   │   └── send.go
│   ├── cache/             # SQLite schema + queries
│   │   ├── schema.sql
│   │   ├── migrate.go
│   │   ├── messages.go
│   │   ├── threads.go
│   │   └── labels.go
│   ├── sync/              # Sync coordinator
│   │   ├── backfill.go
│   │   └── delta.go
│   ├── render/            # HTML → terminal rendering
│   │   ├── html.go
│   │   ├── quote.go
│   │   └── attachments.go
│   ├── compose/           # Compose / reply / forward logic
│   │   ├── editor.go      # $EDITOR integration
│   │   ├── mime.go        # MIME assembly
│   │   └── quoting.go
│   ├── config/
│   │   └── config.go
│   └── ui/
│       ├── app.go         # root model
│       ├── msg.go         # tea.Msg types
│       ├── keys.go        # keybinding map
│       ├── styles.go      # lipgloss styles
│       └── views/
│           ├── accounts.go
│           ├── labels.go
│           ├── mailbox.go
│           ├── reader.go
│           ├── compose.go
│           ├── search.go
│           └── help.go
└── pkg/                   # nothing exported from v1
```

### Concurrency model

- Main goroutine: Bubble Tea event loop.
- Worker goroutines: spawned by `tea.Cmd` closures, send results back as `tea.Msg`.
- Background sync goroutine: long-lived, started in `Init()`, polls every N seconds (default 60s), posts `SyncUpdateMsg` on change.
- SQLite: single `*sql.DB` with connection pool; all access through repository functions.

---

## 7. Data Model

### Keychain (via go-keyring)

- Service: `com.<owner>.gandt`
- Key: `oauth-token:<account_id>` → JSON-serialized `oauth2.Token`
- Key: `client-credentials` → JSON `{client_id, client_secret}` (shared across accounts)

### SQLite schema

The database runs in **WAL mode** (`PRAGMA journal_mode=WAL`) so external processes can read the cache concurrently with the G&T process. Foreign keys are enforced (`PRAGMA foreign_keys=ON`). The schema is versioned via a `schema_version` table and migrated on startup.

The schema is considered a **public interface**. It is documented in `docs/schema.md`, changes are backward-compatible within a major version, and any external tool can open `cache.db` with a stock SQLite client. This is deliberate — the cache is intended to be a useful local substrate beyond the TUI itself.

```sql
CREATE TABLE schema_version (
  version     INTEGER NOT NULL,
  applied_at  DATETIME NOT NULL
);

CREATE TABLE accounts (
  id           TEXT PRIMARY KEY,          -- UUID
  email        TEXT NOT NULL UNIQUE,
  display_name TEXT,
  added_at     DATETIME NOT NULL,
  last_sync_at DATETIME,
  history_id   TEXT,                      -- Gmail historyId (opaque string)
  color        TEXT                       -- hex, for account badge
);

CREATE TABLE labels (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  id         TEXT NOT NULL,               -- Gmail label ID
  name       TEXT NOT NULL,
  type       TEXT NOT NULL,               -- 'system' | 'user'
  unread     INTEGER DEFAULT 0,
  total      INTEGER DEFAULT 0,
  color_bg   TEXT,
  color_fg   TEXT,
  PRIMARY KEY (account_id, id)
);

CREATE TABLE threads (
  account_id   TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  id           TEXT NOT NULL,
  snippet      TEXT,
  history_id   TEXT,
  last_message_date DATETIME,
  PRIMARY KEY (account_id, id)
);

CREATE TABLE messages (
  account_id   TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  id           TEXT NOT NULL,
  thread_id    TEXT NOT NULL,
  from_addr    TEXT,
  to_addrs     TEXT,                      -- JSON array
  cc_addrs     TEXT,
  bcc_addrs    TEXT,
  subject      TEXT,
  date         DATETIME,
  snippet      TEXT,
  size_bytes   INTEGER,
  body_plain   TEXT,                      -- nullable; NULL until full fetch
  body_html    TEXT,                      -- nullable
  raw_headers  TEXT,                      -- JSON map
  internal_date DATETIME,
  fetched_full INTEGER DEFAULT 0,         -- 1 = full body cached per policy
  cached_at    DATETIME,                  -- when body was last fetched (for retention)
  PRIMARY KEY (account_id, id)
);

CREATE TABLE message_labels (
  account_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  label_id   TEXT NOT NULL,
  PRIMARY KEY (account_id, message_id, label_id)
);

CREATE TABLE attachments (
  account_id    TEXT NOT NULL,
  message_id    TEXT NOT NULL,
  part_id       TEXT NOT NULL,            -- Gmail partId
  filename      TEXT,
  mime_type     TEXT,
  size_bytes    INTEGER,
  attachment_id TEXT,                     -- Gmail attachmentId for fetch
  local_path    TEXT,                     -- nullable; set when bytes cached on disk
  PRIMARY KEY (account_id, message_id, part_id)
);

CREATE TABLE outbox (
  id            TEXT PRIMARY KEY,
  account_id    TEXT NOT NULL,
  raw_rfc822    BLOB NOT NULL,
  queued_at     DATETIME NOT NULL,
  attempts      INTEGER DEFAULT 0,
  last_error    TEXT,
  status        TEXT NOT NULL             -- 'pending' | 'sent' | 'failed'
);

-- Sync policy: defines what G&T caches locally. See §11.
CREATE TABLE sync_policies (
  account_id         TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  label_id           TEXT NOT NULL,       -- or '*' for account default
  include            INTEGER NOT NULL,    -- 1 = include, 0 = exclude
  depth              TEXT NOT NULL,       -- 'none' | 'metadata' | 'body' | 'full'
  retention_days     INTEGER,             -- nullable = no limit
  attachment_rule    TEXT NOT NULL,       -- 'none' | 'under_size' | 'all'
  attachment_max_mb  INTEGER,             -- relevant when rule='under_size'
  updated_at         DATETIME NOT NULL,
  PRIMARY KEY (account_id, label_id)
);

-- Never-cache list: senders/domains/labels whose messages bypass the local cache entirely.
-- Matched messages are fetched on demand and never written to disk.
CREATE TABLE cache_exclusions (
  account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  match_type  TEXT NOT NULL,              -- 'sender' | 'domain' | 'label'
  match_value TEXT NOT NULL,
  created_at  DATETIME NOT NULL,
  PRIMARY KEY (account_id, match_type, match_value)
);

-- Reserved namespace for future local-only annotations written by integrations
-- (e.g. agent-generated summaries, tags, embeddings refs). NOT populated in v1.
-- Defined here so downstream additions don't require a breaking schema migration.
CREATE TABLE message_annotations (
  account_id   TEXT NOT NULL,
  message_id   TEXT NOT NULL,
  namespace    TEXT NOT NULL,             -- producer identifier, e.g. 'gandt.core', 'user'
  key          TEXT NOT NULL,
  value        TEXT,                      -- JSON-encoded; opaque to G&T core
  created_at   DATETIME NOT NULL,
  updated_at   DATETIME NOT NULL,
  PRIMARY KEY (account_id, message_id, namespace, key)
);

-- Full-text search over cached message content. Primary purpose: offline search
-- of the cache (Gmail search over the wire stays the default for online queries).
-- Secondary benefit: gives downstream tools a ready-made query surface.
CREATE VIRTUAL TABLE messages_fts USING fts5(
  account_id UNINDEXED,
  message_id UNINDEXED,
  subject,
  from_addr,
  to_addrs,
  snippet,
  body_plain,
  tokenize='porter unicode61'
);

-- Triggers keep FTS in sync with messages.
CREATE TRIGGER messages_fts_ai AFTER INSERT ON messages BEGIN
  INSERT INTO messages_fts(account_id, message_id, subject, from_addr, to_addrs, snippet, body_plain)
  VALUES (new.account_id, new.id, new.subject, new.from_addr, new.to_addrs, new.snippet, new.body_plain);
END;
CREATE TRIGGER messages_fts_au AFTER UPDATE ON messages BEGIN
  DELETE FROM messages_fts WHERE account_id = old.account_id AND message_id = old.id;
  INSERT INTO messages_fts(account_id, message_id, subject, from_addr, to_addrs, snippet, body_plain)
  VALUES (new.account_id, new.id, new.subject, new.from_addr, new.to_addrs, new.snippet, new.body_plain);
END;
CREATE TRIGGER messages_fts_ad AFTER DELETE ON messages BEGIN
  DELETE FROM messages_fts WHERE account_id = old.account_id AND message_id = old.id;
END;

CREATE INDEX idx_messages_thread   ON messages(account_id, thread_id);
CREATE INDEX idx_messages_date     ON messages(account_id, internal_date DESC);
CREATE INDEX idx_messages_cached   ON messages(account_id, cached_at);
CREATE INDEX idx_msglabels_label   ON message_labels(account_id, label_id);
CREATE INDEX idx_annot_namespace   ON message_annotations(namespace);
```

### On-disk layout

```
$XDG_CONFIG_HOME/gandt/
  config.toml
  accounts.json              # non-secret account registry (email, id, color)
$XDG_DATA_HOME/gandt/
  cache.db                   # SQLite
  attachments/<account_id>/<message_id>/<filename>
  logs/gandt.log
```

Secrets (OAuth tokens) never touch disk in plaintext — keychain only.

---

## 8. Authentication & Multi-Account

### OAuth client credentials

The user brings their own Google Cloud OAuth client. First run shows setup instructions:

1. Create a project at console.cloud.google.com
2. Enable Gmail API
3. Create an OAuth 2.0 Client ID, type **Desktop app**
4. Paste client ID + secret into the TUI setup screen (stored in keychain, not config)

This is necessary because distributing a shared client ID risks abuse, Google quota issues, and for `gmail.modify` scope, a restricted-scope app must go through verification for public distribution. For personal use, this is a one-time annoyance.

### OAuth flow (loopback redirect)

1. Generate random port on `127.0.0.1`.
2. Construct auth URL with scope `https://www.googleapis.com/auth/gmail.modify` and `https://www.googleapis.com/auth/gmail.send` (and `userinfo.email` for account identification).
3. Open browser via `pkg/browser.OpenURL`.
4. Spin up `http.Server` on the random port, wait for `/callback?code=...`.
5. Exchange code for token; fetch profile to get email address.
6. Generate account ID (UUID), store token in keychain under `oauth-token:<id>`, register account in `accounts` table and `accounts.json`.
7. Shut down callback server, return to TUI.

### Account switcher

- First-class UI element, top-left of screen.
- Keyboard shortcut: `Ctrl+A` opens switcher, digits `1`–`9` jump to account by ordinal.
- Each account has a color badge (auto-assigned, user-overridable) that tints the frame when active.
- Unified inbox is **not** v1. Per-account views only. Account switcher is instant because all accounts stay authenticated and their data is pre-cached.

### Scopes

| Scope | Purpose |
|-------|---------|
| `gmail.modify` | Read, label, archive, trash (the only non-obvious one: covers read + modify but not permanent delete of spam/trash) |
| `gmail.send` | Send messages |
| `userinfo.email` | Identify which account just authenticated |

Avoid `gmail.readonly` (can't archive) and full `https://mail.google.com/` (requires OAuth verification hell).

---

## 9. Gmail API Surface

Endpoints the client uses, grouped by feature:

### Messages
- `users.messages.list` — list IDs for a label/query
- `users.messages.get` (format=METADATA) — thread/label metadata, batch
- `users.messages.get` (format=FULL) — full body with parts
- `users.messages.batchModify` — bulk label add/remove, archive
- `users.messages.trash` / `users.messages.untrash`
- `users.messages.attachments.get` — attachment bytes

### Threads
- `users.threads.get` — full thread for reader pane
- `users.threads.modify` — label operations on entire thread

### Labels
- `users.labels.list` — populate sidebar
- `users.labels.create` / `users.labels.delete` — user label management

### Drafts
- `users.drafts.list` / `get` / `create` / `update` / `delete`
- `users.drafts.send` — convert draft to sent message

### Send
- `users.messages.send` — for composed messages not saved as drafts first

### Sync
- `users.history.list` — incremental changes since `historyId`

### Profile
- `users.getProfile` — fetch email + `historyId` on first sync

### Send-as identities
- `users.settings.sendAs.list` — discover aliases so Compose can pick the right From

---

## 10. Sync Strategy

Sync is **policy-driven**: every fetch decision — which labels, how deep (metadata vs body vs full), how far back, attachments or not, retention — is looked up in the `sync_policies` table (§7, §11). The sync engine itself is dumb: it reads policy, performs the necessary API calls, writes to the cache.

### First-time backfill per account

1. On account add, fetch `users.getProfile` → store current `historyId`.
2. Fetch labels via `users.labels.list`.
3. Seed `sync_policies` with the default policy (§11).
4. For each label with `include=1`, page through `users.messages.list` scoped to the policy's retention window. Cap IDs at the configured backfill limit per label (default 5,000).
5. Batch-fetch metadata in groups of 100 via `BatchGet` (format=METADATA). Store in `messages` + `message_labels`.
6. For labels whose policy depth is `body` or `full`, enqueue body fetches to a background worker (throttled to avoid quota spikes).
7. Mark account as synced, record `last_sync_at`.

Policy depth governs what's pre-fetched:
- `none` — label visible in sidebar, nothing cached; clicking into it streams from Gmail
- `metadata` — headers, subject, snippet, labels; body fetched on demand and **not** cached
- `body` — metadata + `text/plain` body cached; HTML and attachments on demand
- `full` — metadata + plaintext + HTML + attachments per the attachment rule

### Incremental delta

On each sync tick (default 60s active, 5min idle):

1. For each account, call `users.history.list?startHistoryId=<stored>`.
2. Process `messagesAdded`, `messagesDeleted`, `labelsAdded`, `labelsRemoved` in order.
3. For each added or label-changed message, consult policy for the label(s) it now belongs to and take the **maximum** depth across matching policies.
4. For messages matching any `cache_exclusions` rule (sender/domain/label), metadata is fetched transiently to populate the sidebar counts but the row is **not** written to `messages`; bodies are streamed on demand and never persisted.
5. For deleted messages, delete rows (cascade handles children).
6. Update `accounts.history_id` to the latest returned.

### History window expiration

Gmail only retains history for ~7 days. If `history.list` returns 404/`historyNotFound`:

1. Fall back to backfill strategy, respecting current policy.
2. Reuse existing cached bodies — don't refetch bodies already present.

### Retention enforcement

A retention sweep runs once per day (and on startup):

1. For each `(account_id, label_id)` policy with `retention_days` set, find messages whose latest label assignment is that label and whose `internal_date` is older than the cutoff.
2. If a message is older than the retention window across **all** its labels' policies, prune the row (cascade handles children, attachments on disk deleted by path).
3. Run `PRAGMA wal_checkpoint(TRUNCATE)` after a prune to reclaim space; run `VACUUM` on demand via `:cache-compact`.

### Manual refresh

- `r` — immediate delta sync for the active account
- `R` — full re-list of the current label
- `:sync-all` — delta sync across every account

---

## 11. Local Data Control

This is the user-facing layer that makes the cache explicit rather than magic. The user should always be able to answer three questions: **what's cached, why is it cached, and how do I change or remove it?**

### Design principles

1. **No implicit caching.** Every byte on disk traces back to a policy the user can inspect and edit.
2. **Sensible defaults, editable everywhere.** First-run defaults cover 90% of users without configuration; advanced users can go per-label.
3. **Readable cache.** The SQLite file is plain, WAL-mode, documented. Nothing bespoke; any tool that speaks SQLite can read it.
4. **Clean purge.** Any subset of the cache can be removed without corrupting the rest.
5. **Privacy controls are first-class**, not buried. Sensitive labels and senders can be explicitly excluded from local storage.

### Policy model

A policy is a tuple: **(account, label, depth, retention, attachment rule)**.

- Policies are keyed by `(account_id, label_id)`.
- A special `label_id = '*'` defines the account default, applied to any label without a specific policy.
- When a message has multiple labels with conflicting policies, the **maximum depth** wins. Retention uses the **longest** window.

### Default policy (seeded on account add)

| Label scope | Depth | Retention | Attachments |
|---|---|---|---|
| `INBOX`, `STARRED`, `IMPORTANT` | `full` | 90 days | under 10 MB |
| `SENT`, `DRAFT` | `body` | 90 days | none |
| `SPAM`, `TRASH` | `metadata` | 30 days | none |
| All other labels (`*`) | `metadata` | 365 days | none |

Yields roughly 100–500 MB of disk for an active user; tunable by anyone who cares.

### User controls

**Interactive:**
- `:cache` opens a dashboard: size by account, by label, by message age, attachment footprint, row counts, FTS index size.
- `:cache-policy` opens a table editor (account × label grid) for adjusting depth/retention/attachments.
- `:cache-exclude <sender|domain|label>` adds to the never-cache list.
- `:cache-purge` with flags: `--account`, `--label`, `--older-than`, `--from`, `--dry-run`. Always shows count and size preview before executing.
- `:cache-compact` runs `VACUUM`.
- `:cache-wipe` nukes the DB after a two-step confirmation.

**Config file** (declarative, overrides interactive edits):
```toml
[cache.defaults]
depth = "full"
retention_days = 90
attachment_rule = "under_size"
attachment_max_mb = 10

[[cache.policies]]
account = "work"
label = "receipts"
depth = "full"
retention_days = 1825            # 5 years
attachment_rule = "all"

[[cache.exclusions]]
account = "personal"
match_type = "label"
match_value = "private"
```

Precedence: explicit policy row in DB > config file policy > account default > global default.

### What the UI shows

- Label sidebar includes an indicator per label: `●` (full), `◐` (body), `○` (metadata), `·` (not cached). Hovering / focusing a label shows the depth + retention.
- Messages in the reader pane display a cache state badge: `cached` / `streamed` / `excluded`.
- Status bar during sync shows bytes written and labels affected.

### External tool access

The cache is readable by any process with filesystem access to `cache.db` while G&T is running, because the database operates in WAL mode. The schema is documented and semver'd. This is deliberate: users who want to script against their mail (scheduled exports, downstream analytics, agents, indexing) should be able to, without going through G&T.

**Reserved for future additions, not built in v1:**
- The `message_annotations` table (§7) exists as a namespace for downstream tools to write local-only metadata (summaries, tags, embeddings references). G&T core does not populate it. Schema stability is guaranteed so future additions don't break the cache.
- No IPC, plugin, or agent surface is specified here. That's a later conversation; this section only ensures the cache is a clean substrate when it happens.

### Performance

| Operation | Target |
|---|---|
| Dashboard render | <200ms for a 1 GB cache |
| Purge 10k messages | <2s including index update |
| Retention sweep, 100k messages | <5s |
| FTS query (cached) | <100ms for typical queries |

---

## 12. UX & Information Architecture

### Layout


Default three-pane horizontal split (like aerc/mutt-wizard):

```
┌────────────────────────────────────────────────────────────────────────┐
│ G&T · [work: me@work.com ▼]                            9:34 · syncing… │
├────────────┬─────────────────────────┬─────────────────────────────────┤
│ Inbox   42 │ ▸ Alice       9:21     │ From: Alice <alice@...>         │
│ Starred  3 │   Re: Q4 plan    ●      │ Subject: Re: Q4 plan            │
│ Sent       │                         │ Date: Thu Apr 23 09:21          │
│ Drafts   1 │   Bob         9:10     │                                 │
│ Spam       │   Invoice #4132         │ Hey — on Q4, I think we should │
│ Trash      │                         │ focus on the following:         │
│            │   Carol       8:55     │                                 │
│ ─ Labels ─ │   Lunch next week?      │   1. Migration prep             │
│ work    12 │                         │   2. Hiring pipeline            │
│ travel     │   (28 more…)            │                                 │
│ receipts 4 │                         │ ─── 2 attachments ─────────     │
│            │                         │ • plan.pdf       (142 KB)       │
│            │                         │ • roadmap.png    (88 KB)        │
├────────────┴─────────────────────────┴─────────────────────────────────┤
│ j/k: nav   ↵: open   r: reply   c: compose   /: search   ?: help       │
└────────────────────────────────────────────────────────────────────────┘
```

### Responsive collapse

- `< 120 cols` — collapse labels sidebar to a hotkey-driven overlay (press `g` to jump).
- `< 80 cols` — two-pane: list OR reader, toggle with `Tab`.

### Modes (loosely Vim-inspired)

| Mode | Entered by | Purpose |
|------|------------|---------|
| Normal | default | Navigate, trigger actions |
| Search | `/` | Type a Gmail query |
| Compose | `c`, `r`, `R`, `f` | Edit message (hands off to external editor optionally) |
| Command | `:` | Less-frequent actions (`:add-account`, `:remove-account`, `:quit`) |
| Help | `?` | Overlay showing keybindings |

### Visual design

- Lip Gloss theme with semantic roles: `primary`, `muted`, `accent`, `error`, `warn`, `success`, per-account `accent`.
- Unread messages bolded, read messages dimmed.
- Selected row has a full-width accent background.
- Thread count chip next to subject (`[4]` for 4 messages in thread).
- Account indicator always visible in header.
- Respect `NO_COLOR` env var.

---

## 13. Feature Specifications

### 13.1 Account management

- **Add**: `:add-account` command. Runs OAuth flow, stores token, kicks off backfill.
- **Remove**: `:remove-account`. Confirms, revokes token (best-effort via `oauth2.google.RevokeToken`), deletes DB rows, removes keychain entry.
- **Switch**: `Ctrl+A` or `1`..`9`.
- **Acceptance**: can add 3 accounts in under 2 min each; switcher round-trip is <50ms.

### 13.2 Mailbox navigation

- Label list in left pane, with unread counts.
- `g` + label prefix jumps to label.
- Arrow/jk navigation in message list.
- Virtualized rendering (Bubbles `list` handles this).
- Threads collapsed to a single row; expanding a thread shows all messages inline.
- **Acceptance**: 5,000-message inbox lists in <100ms from cache; scroll at 60fps.

### 13.3 Message reading

- Reader pane renders headers, body, attachment list.
- Body rendering order of preference: `text/plain` → `html2text(text/html)` → `[no body]`.
- Optional `V` keybind toggles raw HTML→markdown via Glamour for messages where plaintext is poor.
- Quoted text (`>` prefixed lines) dimmed and collapsible with `z`.
- Thread navigation: `J`/`K` move between messages within the thread; `N`/`P` move between threads.
- **Acceptance**: full body fetch + render in <500ms on broadband.

### 13.4 Search

- `/` opens search input. Default mode: **online** — query passed verbatim to `users.messages.list?q=...` with full Gmail syntax (`from:`, `to:`, `subject:`, `has:attachment`, `newer_than:`, `label:`, etc.).
- `Ctrl+/` toggles to **offline** mode — query runs against the local FTS5 index (§7). Same syntax where overlap makes sense (`from:`, `subject:`); full-text terms otherwise. Useful when offline, and faster for large result sets within the cached window.
- Results shown in the message list pane with a `search: <query> [online|offline]` header.
- `Esc` exits search view.
- Recent searches stored locally, accessible via `Ctrl+R`.
- **Acceptance**: online search renders within 1s on typical queries; offline search within 100ms.

### 13.5 Compose / Reply / Forward

- **Headers via Huh** — To, Cc, Bcc, Subject, From (dropdown of sendAs aliases for the active account).
- **Body via `$EDITOR`** by default (opens vim/nano/etc. on a tempfile with optional pre-filled quoted content). If `$EDITOR` is unset, falls back to inline Bubbles `textarea`.
- **Config switch** `compose.editor = "inline" | "external"` to force either mode.
- **Attachments**: `Ctrl+T` opens a file picker (Bubbles file browser component or path input).
- **Reply quoting**: `> ` prefix with attribution line `On $date, $from wrote:`.
- **Reply-all**: includes original To + Cc minus the user's own address.
- **Forward**: empty To, subject prefixed `Fwd: `, full original message quoted with headers.
- **Draft autosave** every 30s via `users.drafts.create/update`. Drafts sync to Gmail so they appear on mobile.
- **Send**: on confirmation, MIME-assemble, call `users.messages.send` or `users.drafts.send`. On network failure, enqueue in `outbox` table and retry with exponential backoff.
- **Acceptance**: end-to-end compose-and-send works from cold start in <30s.

### 13.6 Triage actions

| Key | Action | API call |
|-----|--------|----------|
| `e` | Archive | `batchModify` remove `INBOX` |
| `#` | Trash | `messages.trash` |
| `!` | Mark as spam | `batchModify` add `SPAM` remove `INBOX` |
| `s` | Toggle star | `batchModify` toggle `STARRED` |
| `u` | Toggle unread | `batchModify` toggle `UNREAD` |
| `+` | Add label (prompt) | `batchModify` add label |
| `-` | Remove label (prompt) | `batchModify` remove label |
| `m` | Mute thread | `threads.modify` add `MUTED` |

- All operations optimistic — UI updates immediately, API call dispatched async, revert on error.
- Undo (`U`) reverses the last single action within 30s.
- **Acceptance**: triaging 20 messages feels instantaneous.

### 13.7 Attachments

- Listed below body, with size and type.
- `Ctrl+S` on selected attachment saves to `$XDG_DOWNLOAD_DIR` (or config-specified path).
- `Ctrl+O` opens with `open`/`xdg-open`/`start`.
- Fetched lazily on first access, cached in `attachments/` subdir.

### 13.8 Drafts

- `Drafts` label shows current drafts.
- Selecting a draft re-opens the compose view with populated fields.
- Drafts always round-trip to Gmail — no local-only drafts. This simplifies multi-device consistency.

### 13.9 Labels

- System labels visible in sidebar: Inbox, Starred, Sent, Drafts, Spam, Trash, Important.
- User labels grouped below a divider, sorted alphabetically, optionally nested by `/` separator (Gmail convention).
- Create/delete via `:create-label` / `:delete-label` commands.
- **Non-goal**: editing filter rules (Gmail doesn't expose filters via API except in limited form — defer).

### 13.10 Notifications

- No OS-level notifications in v1.
- Status bar shows new-message count since last focus.
- Future: integrate with `notify-send` / `terminal-notifier` behind a config flag.

---

## 14. Rendering Strategy (HTML → terminal)

This is the hardest rendering problem and is explicitly scoped as "best-effort, iterate."

### Rules

1. If a `text/plain` MIME part exists, use it. Most legitimate senders include one.
2. If only `text/html` is available:
   a. Run through `html2text` with sane defaults (prefer anchor text over URLs, preserve line breaks, strip `<style>`/`<script>`).
   b. If the result is <10% of the HTML byte size, assume it's too aggressive and try alternative settings.
3. Provide a `V` toggle to switch between: plaintext → html2text → raw html → glamour(markdownified). Power user escape hatch.
4. URLs in links: render as `text [^1]` with a footnote section at bottom listing all links.
5. Images: show `[image: <alt or filename>]`. Optional Kitty/iTerm graphics is a future enhancement behind a feature flag.
6. Tables: `html2text` handles basic tables; complex layouts will be ugly — acceptable.
7. Marketing/newsletter HTML: will look bad. Acceptable. Users can fall back to "open in browser" via `B`.

### Quoted text

- Detect `> ` prefix and `------- Original Message -------` dividers.
- Collapse by default if quoted portion is >50% of the message.
- Expand with `z`.

---

## 15. Keybindings

### Normal mode

| Key | Action |
|-----|--------|
| `j`/`k` or ↓/↑ | Move down/up |
| `g`/`G` | Jump to top/bottom |
| `↵` | Open thread |
| `Esc` | Close pane / cancel |
| `/` | Search |
| `c` | Compose new |
| `r` | Reply |
| `R` | Reply-all |
| `f` | Forward |
| `e` | Archive |
| `#` | Trash |
| `s` | Star toggle |
| `u` | Unread toggle |
| `+`/`-` | Label add/remove |
| `m` | Mute thread |
| `U` | Undo last action |
| `Tab` | Toggle pane focus (narrow mode) |
| `g <label>` | Jump to label |
| `Ctrl+A` | Account switcher |
| `1`..`9` | Jump to Nth account |
| `r` (in reader) | Reply (context-dependent) |
| `V` | Toggle render mode |
| `B` | Open message in browser |
| `Ctrl+S` | Save attachment |
| `Ctrl+O` | Open attachment |
| `Ctrl+R` | Recent searches |
| `?` | Help overlay |
| `:` | Command mode |
| `q` | Quit |

### Compose mode

| Key | Action |
|-----|--------|
| `Ctrl+T` | Attach file |
| `Ctrl+D` | Save draft and close |
| `Ctrl+S` | Send |
| `Ctrl+C` | Discard (with confirm) |
| `Ctrl+E` | Open body in `$EDITOR` (when in inline mode) |

Rebindable via `config.toml`.

---

## 16. Configuration

`$XDG_CONFIG_HOME/gandt/config.toml`:

```toml
[ui]
theme = "dark"                    # dark | light | auto
compose_editor = "external"       # external | inline
render_mode_default = "plaintext" # plaintext | html2text | glamour
render_url_footnotes = true

[sync]
poll_active_seconds = 60
poll_idle_seconds = 300
backfill_limit_per_label = 5000

# Cache policy. See §11. These values seed the sync_policies table on
# account add and are reapplied whenever the config file is reloaded.
# Interactive edits via :cache-policy persist to the DB and take
# precedence over file defaults for the labels they touch.
[cache.defaults]
depth = "full"                    # none | metadata | body | full
retention_days = 90               # null = no limit
attachment_rule = "under_size"    # none | under_size | all
attachment_max_mb = 10
total_budget_mb = 2000            # soft cap; triggers retention sweep when exceeded

[[cache.policies]]
account = "work"                  # matches accounts.<name> key below
label = "receipts"
depth = "full"
retention_days = 1825
attachment_rule = "all"

[[cache.exclusions]]
account = "personal"
match_type = "label"              # sender | domain | label
match_value = "private"

[accounts.work]
color = "#4287f5"

[accounts.personal]
color = "#f542a7"

[keys]
archive = "e"
trash = "#"
# ... overrides

[paths]
downloads = "~/Downloads"
```

Account-specific OAuth tokens and client credentials are in keychain, not in this file.

---

## 17. Error Handling & Offline Behavior

### Error display

- Non-fatal errors: toast at bottom of screen, auto-dismiss after 5s, logged to file.
- Fatal errors (DB corruption, keychain inaccessible): clear error screen, no crash dump to terminal.
- Rate limit (429): exponential backoff, status bar indicator "Gmail rate-limited, retrying in Xs".

### Offline mode

- Detected by failed sync attempt with network error.
- Status bar shows "offline".
- Reading cached messages works.
- Composes can be written; on `Send`, message goes to `outbox` table.
- Outbox processed when connectivity returns.
- Triage actions queue in an `pending_actions` table (future enhancement; v1 just errors out with "action requires network").

### Token refresh

- Handled transparently by `oauth2.Token` + `TokenSource`.
- If refresh token is revoked/expired: surface a clear "re-authenticate <account>" prompt, invoke OAuth flow for that account only.

---

## 18. Performance Targets

| Metric | Target |
|--------|--------|
| Cold start to interactive | <300ms |
| Inbox list render (5k messages, cached) | <100ms |
| Thread open (cached) | <50ms |
| Thread open (cache miss, broadband) | <500ms |
| Search query round-trip | <1s |
| Triage action perceived latency | <50ms (optimistic UI) |
| Memory (10k cached messages) | <150MB |
| Binary size | <30MB |

---

## 19. Security Considerations

- OAuth tokens **only** in OS keychain (macOS Keychain, Secret Service on Linux, Credential Manager on Windows).
- Client ID/secret likewise in keychain. Never in config file or env vars by default.
- Message bodies cached unencrypted on disk in SQLite — **documented clearly**. Users with threat model requiring at-rest encryption should use FileVault / LUKS / BitLocker.
- TLS-only for all Gmail API traffic (enforced by the SDK).
- No telemetry, no crash reporting phoning home. All logs local.
- `gmail.modify` + `gmail.send` scopes require Google OAuth verification for public distribution — document that users must use their own client credentials.

---

## 20. Testing Strategy

| Layer | Approach |
|-------|----------|
| `cache/` | Standard Go tests with in-memory SQLite |
| `render/` | Golden-file tests: input HTML → expected rendered text |
| `gmail/` | Interface-based, mock Gmail service for unit tests; contract tests against recorded HTTP fixtures (`httptest`) |
| `sync/` | Scenario tests with fixture histories |
| `ui/` | `teatest` package from Bubble Tea for integration tests on the Model |
| End-to-end | Manual QA against a real test Gmail account before each release |

CI: GitHub Actions, matrix on macOS + Linux + Windows, Go 1.25 + the current stable Go release.

---

## 21. Milestones

### M0 — Foundations
- Repo scaffolded, deps pinned, CI green.
- Bubble Tea skeleton with three-pane layout and dummy data.
- Config loading.
- **Demo**: `gandt` opens, shows fake inbox, quits cleanly.

### M1 — Single-account read-only
- OAuth flow + keychain storage.
- Account add command.
- Gmail client wrapper with list/get/labels.
- SQLite cache with schema v1 (WAL mode, FTS5 triggers, annotations table reserved).
- Policy-aware backfill with default policy seeded on account add.
- Mailbox view + reader view with plaintext rendering.
- **Acceptance**: add one account, browse inbox, read threads, text is readable, `sqlite3 cache.db` shows expected tables including `messages_fts` and `message_annotations`.

### M2 — Triage + sync + cache controls
- `history.list` delta sync + background poller (policy-aware).
- Archive / trash / star / label / unread operations.
- Optimistic UI + undo.
- `:cache` dashboard (size, counts).
- `:cache-policy` editor.
- `:cache-purge` with dry-run and confirmation.
- Retention sweep job.
- `cache_exclusions` enforcement.
- **Acceptance**: triage a real inbox down to zero; adjust a policy and observe it take effect on next sync; purge + compact reclaims disk.

### M3 — Multi-account
- Account switcher UI.
- Per-account color badges.
- Per-account cache policies honored independently.
- All operations route to correct account.
- **Acceptance**: 3 accounts work identically, switch is instant, per-account cache settings differ.

### M4 — Search
- Online search input mode (Gmail query passthrough).
- Offline search via FTS5 over the cache (`Ctrl+/` toggle).
- Results pane.
- Recent searches.
- **Acceptance**: Gmail search syntax works online; offline search returns cached results in <100ms.

### M5 — Compose / Reply / Send
- Huh form for headers.
- `$EDITOR` integration.
- Reply/reply-all/forward with quoting.
- MIME assembly.
- Drafts sync.
- Send with outbox retry.
- Attachments (send + download).
- **Acceptance**: full compose-reply-send cycle works across accounts.

### M6 — Polish
- HTML rendering improvements.
- Theme refinement.
- Error states.
- Performance passes.
- Docs + README + release binaries.

---

## 22. Open Questions

1. **OAuth client distribution**. Bring-your-own is the safe default. Consider a shared low-quota client for "try it out" mode? Adds verification burden. Defer.
2. **Glamour vs custom renderer for HTML bodies**. Start with html2text; revisit if rendering quality is intolerable.
3. **Should the background sync continue when the TUI is in the "sleeping" state (e.g. terminal backgrounded)?** Default yes; provide a config flag.
4. **Encrypted cache at rest**. Could use SQLCipher (needs CGO) or app-level encryption with a keychain-stored key. Not in v1, but worth keeping the schema abstraction clean so we can slot it in. Note that encryption would constrain external-tool access (§11), so this is a real tradeoff rather than a pure win.
5. **Mobile-style "unified inbox" across accounts**. Power users often want this; complicates the data model (global ordering, cross-account actions). Defer to v2 unless trivially cheap via a VIEW.
6. **Vim vs emacs keybinding sets as presets**. Probably worth shipping both via `keys_preset = "vim" | "emacs" | "mutt"`.
7. **Display name / `From` picking**. Gmail's `sendAs` API gives us aliases. Should the account switcher also be a From switcher at compose time, or keep those separate? Probably separate — the active account dictates which OAuth token is used; within that account, a dropdown picks the alias.
8. **Visual identity**. The gin-and-tonic palette (pale blue-green, lime accent, ice-white highlights) is a natural fit for the default theme — worth prototyping during M6 polish but not blocking earlier milestones.
9. **External tool / agent access surface**. The cache (§11) is deliberately readable by any process — that's the v1 contract. An in-process plugin API, an MCP server over the cache, or a dedicated IPC channel are all plausible post-v1 directions. Not designing any of them now; just ensuring the cache shape, WAL mode, schema versioning, and reserved `message_annotations` table don't paint us into a corner. Decision deferred until real use cases surface.

---

## 23. Out of Scope (explicit)

Filter rules management, vacation responder, signatures editor UI (static signature via config is fine), PGP/S/MIME, calendar/contacts, non-Gmail accounts, daemon mode, SSH hosting via Wish, mobile UI, web UI, telemetry/analytics, formal plugin system, scripting/RPC API, in-process extension points.

These are deliberate exclusions. Revisit after v1 ships and real usage informs priorities.

Note the distinction between a **formal plugin/agent API** (out of scope) and **raw cache access** (explicitly supported per §11). The latter is a byproduct of using SQLite in WAL mode with a documented schema and costs us nothing; the former is a design problem we're not taking on in v1.

---

## Appendix A — Reference projects to study

- **aerc** (Go, IMAP/JMAP) — threading UX, compose flow, attachment handling
- **mutt/neomutt** (C) — keybindings, pager behavior, the archetype
- **himalaya** (Rust) — modern mail TUI, not Gmail-native but well-designed
- **soft-serve** (Go/Bubble Tea) — example of a non-trivial production Bubble Tea app
- **crush** (Charmbracelet) — another modern Bubble Tea app with complex state
