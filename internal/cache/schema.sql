CREATE TABLE schema_version (
  version     INTEGER NOT NULL,
  applied_at  DATETIME NOT NULL
);

CREATE TABLE accounts (
  id           TEXT PRIMARY KEY,
  email        TEXT NOT NULL UNIQUE,
  display_name TEXT,
  added_at     DATETIME NOT NULL,
  last_sync_at DATETIME,
  history_id   TEXT,
  color        TEXT
);

CREATE TABLE labels (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  id         TEXT NOT NULL,
  name       TEXT NOT NULL,
  type       TEXT NOT NULL,
  unread     INTEGER DEFAULT 0,
  total      INTEGER DEFAULT 0,
  color_bg   TEXT,
  color_fg   TEXT,
  PRIMARY KEY (account_id, id)
);

CREATE TABLE threads (
  account_id        TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  id                TEXT NOT NULL,
  snippet           TEXT,
  history_id        TEXT,
  last_message_date DATETIME,
  PRIMARY KEY (account_id, id)
);

CREATE TABLE messages (
  account_id    TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  id            TEXT NOT NULL,
  thread_id     TEXT NOT NULL,
  from_addr     TEXT,
  to_addrs      TEXT,
  cc_addrs      TEXT,
  bcc_addrs     TEXT,
  subject       TEXT,
  date          DATETIME,
  snippet       TEXT,
  size_bytes    INTEGER,
  body_plain    TEXT,
  body_html     TEXT,
  raw_headers   TEXT,
  internal_date DATETIME,
  fetched_full  INTEGER DEFAULT 0,
  cached_at     DATETIME,
  PRIMARY KEY (account_id, id),
  FOREIGN KEY (account_id, thread_id) REFERENCES threads(account_id, id) ON DELETE CASCADE
);

CREATE TABLE message_labels (
  account_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  label_id   TEXT NOT NULL,
  PRIMARY KEY (account_id, message_id, label_id),
  FOREIGN KEY (account_id, message_id) REFERENCES messages(account_id, id) ON DELETE CASCADE,
  FOREIGN KEY (account_id, label_id) REFERENCES labels(account_id, id) ON DELETE CASCADE
);

CREATE TABLE attachments (
  account_id    TEXT NOT NULL,
  message_id    TEXT NOT NULL,
  part_id       TEXT NOT NULL,
  filename      TEXT,
  mime_type     TEXT,
  size_bytes    INTEGER,
  attachment_id TEXT,
  local_path    TEXT,
  PRIMARY KEY (account_id, message_id, part_id),
  FOREIGN KEY (account_id, message_id) REFERENCES messages(account_id, id) ON DELETE CASCADE
);

CREATE TABLE outbox (
  id         TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  raw_rfc822 BLOB NOT NULL,
  queued_at  DATETIME NOT NULL,
  attempts   INTEGER DEFAULT 0,
  last_error TEXT,
  status     TEXT NOT NULL
);

CREATE TABLE sync_policies (
  account_id        TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  label_id          TEXT NOT NULL,
  include           INTEGER NOT NULL,
  depth             TEXT NOT NULL,
  retention_days    INTEGER,
  attachment_rule   TEXT NOT NULL,
  attachment_max_mb INTEGER,
  updated_at        DATETIME NOT NULL,
  PRIMARY KEY (account_id, label_id)
);

CREATE TABLE cache_exclusions (
  account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  match_type  TEXT NOT NULL,
  match_value TEXT NOT NULL,
  created_at  DATETIME NOT NULL,
  PRIMARY KEY (account_id, match_type, match_value)
);

CREATE TABLE message_annotations (
  account_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  namespace  TEXT NOT NULL,
  key        TEXT NOT NULL,
  value      TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (account_id, message_id, namespace, key),
  FOREIGN KEY (account_id, message_id) REFERENCES messages(account_id, id) ON DELETE CASCADE
);

CREATE TABLE recent_searches (
  account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  query      TEXT NOT NULL,
  mode       TEXT NOT NULL,
  last_used  DATETIME NOT NULL,
  PRIMARY KEY (account_id, query, mode)
);

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

CREATE INDEX idx_accounts_email ON accounts(email);
CREATE INDEX idx_labels_type ON labels(account_id, type);
CREATE INDEX idx_threads_date ON threads(account_id, last_message_date DESC);
CREATE INDEX idx_messages_thread ON messages(account_id, thread_id);
CREATE INDEX idx_messages_thread_date ON messages(account_id, thread_id, internal_date DESC, date DESC);
CREATE INDEX idx_messages_date ON messages(account_id, internal_date DESC);
CREATE INDEX idx_messages_cached ON messages(account_id, cached_at);
CREATE INDEX idx_msglabels_label ON message_labels(account_id, label_id);
CREATE INDEX idx_attachments_message ON attachments(account_id, message_id);
CREATE INDEX idx_outbox_account ON outbox(account_id, status);
CREATE INDEX idx_sync_policies_label ON sync_policies(account_id, label_id);
CREATE INDEX idx_exclusions_match ON cache_exclusions(account_id, match_type, match_value);
CREATE INDEX idx_annot_namespace ON message_annotations(namespace);
CREATE INDEX idx_recent_searches_used ON recent_searches(account_id, last_used DESC);
