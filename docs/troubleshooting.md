# Operational Troubleshooting

This guide covers local recovery paths for `gandt`. G&T is a local TUI client; it does not run a daemon, plugin host, telemetry process, or non-Gmail workflow in v1.

## Logs

Logs are local-only and live at:

```sh
$XDG_DATA_HOME/gandt/logs/gandt.log
```

Without XDG overrides on Unix-like systems:

```sh
~/.local/share/gandt/logs/gandt.log
```

Logs should not contain OAuth tokens or message bodies. Treat logs as local diagnostic files and avoid sharing them publicly without review.

## Cache Inspection

The cache is a plain SQLite database in WAL mode:

```sh
sqlite3 "$XDG_DATA_HOME/gandt/cache.db"
```

Useful read-only checks:

```sql
.tables
SELECT email, last_sync_at FROM accounts;
SELECT account_id, COUNT(*) FROM messages GROUP BY account_id;
SELECT account_id, label_id, COUNT(*) FROM message_labels GROUP BY account_id, label_id;
SELECT COUNT(*) FROM messages_fts;
```

The public schema is documented in `docs/schema.md`.

## Cache Privacy

Message bodies in `cache.db` and downloaded files under `attachments/` are unencrypted local files. OAuth client credentials and tokens are stored in the OS keychain, not in SQLite. Users who need at-rest protection should use OS or disk encryption such as FileVault, LUKS, or BitLocker.

## Keychain Troubleshooting

G&T uses the OS keychain:

- macOS: Keychain
- Linux: Secret Service-compatible keychain
- Windows: Credential Manager, best-effort

If G&T reports `keychain inaccessible`, unlock or repair the OS keychain provider and retry the operation. On Linux, confirm a Secret Service provider is running in the desktop session. Do not put OAuth tokens or client secrets into `config.toml` as a workaround.

## OAuth Re-Authentication

If Gmail access is revoked or expired, run:

```text
:add-account
```

Use the same Gmail account to re-authorize. If the Google OAuth client credentials need replacement, run:

```text
:replace-credentials
```

OAuth tokens and client credentials remain in the keychain. Cache purge and wipe commands do not remove them.

## Cache Purge And Recovery

Preview cache removal before deleting data:

```text
:cache-purge --account me@example.com --label INBOX --older-than 90d --dry-run
```

Execute by omitting `--dry-run`; G&T prompts for confirmation:

```text
:cache-purge --account me@example.com --label INBOX --older-than 90d
```

Compact SQLite after large purges:

```text
:cache-compact
```

If the cache is badly corrupted but the TUI still opens, use:

```text
:cache-wipe
```

`cache-wipe` requires two confirmations and removes the SQLite cache plus cached attachments. It does not remove keychain credentials or account tokens. Explicit account removal is separate:

```text
:remove-account
```

## Offline And Rate-Limited States

When offline or rate-limited, cached messages remain browsable and offline search can continue over cached content. Retry sync, online search, or cache-miss thread opens after connectivity or Gmail quota recovers.

## Unsupported Recovery Paths

Do not rely on a background daemon, plugin API, telemetry service, non-Gmail account adapter, scripting/RPC API, or in-process extension point for v1 recovery. These are explicitly out of scope for v1.
