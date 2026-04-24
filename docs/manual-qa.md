# Manual QA Runbook

Use this runbook before tagging a v1 release candidate. Follow it in order and record every failure, crash, confusing state, or slow operation before release.

These steps require real disposable Gmail accounts and must not run in automated CI. Use at least three Gmail test accounts:

- Account A: primary test account
- Account B: secondary account for multi-account routing
- Account C: third account for switcher and per-account policy coverage

Do not use personal or production mailboxes for destructive actions such as spam, trash, purge, wipe, or account removal.

## 1. Prepare The Local QA Environment

- [X] Confirm local prerequisites are available:

  ```sh
  go version
  command -v rg
  command -v sqlite3
  command -v goreleaser
  ```

  Expected: Go 1.25 or newer, `rg`, `sqlite3`, and `goreleaser` are available. If `goreleaser` is missing, complete the app QA first and mark release packaging blocked until it is installed.

- [X] Use isolated config, data, and download directories for this QA pass:

  ```sh
  cd /home/karlo/uwchlan/gandt
  export XDG_CONFIG_HOME="$PWD/.qa/config"
  export XDG_DATA_HOME="$PWD/.qa/data"
  export XDG_DOWNLOAD_DIR="$PWD/.qa/downloads"
  mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_DOWNLOAD_DIR"
  ```

  Expected: all cache, logs, attachments, and downloads for this run stay under `.qa/`.

- [X] Build and run automated checks:

  ```sh
  make test
  make vet
  make build
  ./bin/gandt --version
  ```

  Expected: tests and vet pass. `--version` prints `dev` for a local build unless you built with `make build VERSION=<release>`.

- [X] Prepare Gmail test data before launching the TUI:
  - Send at least one plaintext message from B to A.
  - Send at least one HTML or newsletter-like message to A.
  - Send at least one message with a small attachment to A.
  - Create or identify one disposable message that can be archived, trashed, marked spam, labeled, muted, and unread.
  - Send at least one message between A, B, and C so reply-all has multiple recipients.
  - Create or identify one user label in Account A, or be ready to create one during the label test.

## 2. Launch Without Accounts

- [X] Start the no-color smoke test:

  ```sh
  NO_COLOR=1 ./bin/gandt
  ```

  Expected: the no-account or fake-inbox state renders without ANSI color. Press `?` to open help, `Esc` to close help, then `q` to quit.

- [X] Start the normal local build:

  ```sh
  ./bin/gandt
  ```

  Expected: the TUI opens cleanly with no account configured and no crash.

## 3. Add Three Gmail Accounts

- [ ] Prepare one Google Desktop OAuth client for this QA pass:
  - Create or select one Google Cloud project.
  - Enable the Gmail API.
  - Configure the Google Auth platform consent screen.
  - Create one OAuth client with application type `Desktop app`.
  - If the OAuth app is in testing mode, add Accounts A, B, and C as test users.
  - Keep the generated client ID and client secret ready for G&T.

  Expected: one Desktop OAuth client ID and secret are available. Reuse this same client for all three Gmail accounts; do not create one client per Gmail account.

- [ ] Add Account A:
  - Press `:`.
  - Type `add-account`.
  - Press `Enter`.
  - If prompted, enter the Google Desktop OAuth client ID and secret created above.
  - Complete the browser OAuth flow for Account A.

  Expected: Account A appears in the mailbox, labels appear after bootstrap, and the status bar reports success or sync progress.

- [ ] Add Account B with the same flow:

  ```text
  :add-account
  ```

  Expected: Account B appears without disrupting Account A.

- [ ] Add Account C with the same flow:

  ```text
  :add-account
  ```

  Expected: Account C appears without disrupting Account A or B.

- [ ] Verify account switching:
  - Press `Ctrl+A` to open the account switcher.
  - Use `j`/`k` or arrow keys to move.
  - Press `Enter` to switch.
  - Press `1`, `2`, and `3` from the mailbox view to jump directly to Accounts A, B, and C.

  Expected: switching is visually instant, the active account header changes, and message lists do not leak data between accounts.

- [ ] Verify duplicate-account protection:

  ```text
  :add-account
  ```

  Complete OAuth using an already-added account.

  Expected: G&T rejects the duplicate account without damaging the existing account state.

- [ ] Verify safe account removal:
  - Switch to Account C.
  - Run `:remove-account`.
  - At the prompt, press `n`.
  - Run `:remove-account` again.
  - At the prompt, press `y`.
  - Re-add Account C with `:add-account`.

  Expected: cancel leaves Account C intact; confirm removes only Account C; re-add works; Accounts A and B remain available.

## 4. Inspect SQLite State

- [ ] Quit G&T with `q`, then inspect the cache:

  ```sh
  sqlite3 "$XDG_DATA_HOME/gandt/cache.db" '
  .tables
  SELECT COUNT(*) AS accounts_count FROM accounts;
  SELECT COUNT(*) AS labels_count FROM labels;
  SELECT COUNT(*) AS threads_count FROM threads;
  SELECT COUNT(*) AS messages_count FROM messages;
  SELECT COUNT(*) AS message_labels_count FROM message_labels;
  SELECT COUNT(*) AS messages_fts_count FROM messages_fts;
  SELECT name FROM sqlite_master
  WHERE name IN ("messages_fts", "message_annotations")
  ORDER BY name;
  '
  ```

  Expected: `accounts_count` is 3 after re-adding Account C, labels are nonzero, message counts are nonzero after backfill, and both `message_annotations` and `messages_fts` are listed.

- [ ] Confirm secrets are not in config, SQLite schema, or logs:

  ```sh
  rg -n "access_token|refresh_token|client_secret|Authorization|Bearer" \
    "$XDG_CONFIG_HOME/gandt" "$XDG_DATA_HOME/gandt/logs" || true

  sqlite3 "$XDG_DATA_HOME/gandt/cache.db" '
  SELECT COUNT(*) AS sqlite_secret_hits
  FROM (
    SELECT sql FROM sqlite_master WHERE sql IS NOT NULL
  )
  WHERE sql LIKE "%client_secret%"
     OR sql LIKE "%access_token%"
     OR sql LIKE "%refresh_token%";
  '
  ```

  Expected: no ripgrep matches and `sqlite_secret_hits = 0`. OAuth client credentials and tokens should live only in the OS keychain.

- [ ] Relaunch G&T:

  ```sh
  ./bin/gandt
  ```

  Expected: the three configured accounts load from local state.

## 5. Sync And Cache Controls

- [ ] Confirm initial sync:
  - Switch through Accounts A, B, and C.
  - Confirm labels and messages appear for each account after backfill.

  Expected: each account has independent labels, messages, unread counts, and sync status.

- [ ] Run active-account delta refresh:
  - Switch to Account A.
  - Press `Ctrl+R`.

  Expected: the status bar reports refresh or sync completion, and navigation remains responsive while sync runs.

- [ ] Run all-account refresh:

  ```text
  :sync-all
  ```

  Expected: all configured accounts are refreshed and the account switcher shows per-account sync status.

- [ ] Run selected-label refresh:
  - Select the Inbox label or another label with messages.
  - Run `:sync-label`.

  Expected: the selected label is relisted without blocking the UI.

- [ ] Open the cache dashboard:

  ```text
  :cache
  ```

  Expected: totals for accounts, labels, age buckets, rows, SQLite bytes, FTS rows, and attachments are plausible. Press `Esc` to return.

- [ ] Edit cache policy:
  - Run `:cache-policy`.
  - Use `j`/`k` to select Account A's Inbox or a test label.
  - Press `d` until depth becomes `body` or `full`.
  - Press `t` to cycle retention to `30d`, `90d`, or `365d`.
  - Press `a` to cycle attachment policy.
  - If attachment policy is `under_size`, press `+` or `-` to adjust the MB limit.
  - Press `s` to save.

  Expected: the row is marked explicit with `*`, a refresh runs after save, and the next sync honors the changed policy.

- [ ] Reset a cache policy:
  - Run `:cache-policy`.
  - Select the row edited above.
  - Press `x`.
  - Press `Esc`.

  Expected: the row returns to inherited/default policy values and no unrelated account changes.

- [ ] Preview and cancel a cache exclusion:

  ```text
  :cache-exclude sender <test-sender@example.com>
  ```

  At the preview prompt, press `n`.

  Expected: G&T shows affected message/body/attachment counts and then cancels without purging.

- [ ] Confirm a cache exclusion on disposable data:

  ```text
  :cache-exclude sender <test-sender@example.com>
  ```

  At the preview prompt, press `y`.

  Expected: matching cached rows are purged for the active account only.

- [ ] Run a dry-run purge:

  ```text
  :cache-purge --label INBOX --older-than 365d --dry-run
  ```

  Expected: G&T reports planned message, body, attachment, and byte counts without deleting anything.

- [ ] Run a confirmed purge only on disposable data:

  ```text
  :cache-purge --label INBOX --older-than 365d
  ```

  At the preview prompt, press `y`.

  Expected: selected cache rows and cached attachment files are deleted; OAuth credentials remain available.

- [ ] Compact the cache:

  ```text
  :cache-compact
  ```

  Expected: compact completes without breaking the next launch.

## 6. Reading And Rendering

- [ ] Open a plaintext thread:
  - Select a cached plaintext message.
  - Press `Enter`.

  Expected: headers and readable plaintext body render in the reader pane.

- [ ] Open an HTML-only or newsletter-like thread:
  - Select the HTML message.
  - Press `Enter`.
  - Press `V` repeatedly to cycle `plaintext`, `html2text`, `raw_html`, and `glamour`.

  Expected: each mode changes the reader output without crashing.

- [ ] Verify links, images, and quotes:
  - In an HTML message with links, confirm URL footnotes appear when enabled.
  - In a message with images, confirm image placeholders appear.
  - Press `z` to show and collapse quoted text.

  Expected: quoted text toggles and the status bar reports the quote state.

- [ ] Open Gmail web UI for the selected message:
  - Press `B`.

  Expected: the selected message opens in the browser for the active account.

- [ ] Verify reader navigation:
  - Press `J` and `K` to move between messages in a thread.
  - Press `N` and `P` to move to next and previous threads.

  Expected: selection and reader content stay in sync.

## 7. Triage Actions

Perform these on disposable messages and verify each result in Gmail web UI.

- [ ] Archive:
  - Select an Inbox message.
  - Press `e`.

  Expected: the message is optimistically removed from Inbox.

- [ ] Undo:
  - Within 30 seconds of the archive, press `U`.

  Expected: the message returns.

- [ ] Trash:
  - Select a disposable message.
  - Press `#`.

  Expected: Gmail web UI shows the message in Trash.

- [ ] Spam:
  - Select a disposable message.
  - Press `!`.

  Expected: Gmail web UI shows the message in Spam.

- [ ] Star and unstar:
  - Select a message.
  - Press `s`.
  - Press `s` again.

  Expected: star state toggles in G&T and Gmail web UI.

- [ ] Mark unread/read:
  - Select a message.
  - Press `u`.
  - Press `u` again if needed.

  Expected: unread/read state changes in G&T and Gmail web UI.

- [ ] Add a label:
  - Select a message.
  - Press `+`.
  - Type an existing label name or a new disposable label name.
  - Press `Enter`.

  Expected: label is applied to the selected message only.

- [ ] Remove a label:
  - Select the labeled message.
  - Press `-`.
  - Type the label name or press `Enter` to remove the first removable label.

  Expected: label is removed from the selected message only.

- [ ] Mute:
  - Select a disposable thread.
  - Press `m`.

  Expected: thread is muted in G&T and Gmail web UI.

- [ ] Verify account routing:
  - Switch to Account B.
  - Perform one harmless action such as star/unstar.
  - Switch back to Account A.

  Expected: Account B action does not affect Account A, even if Gmail message IDs overlap internally.

## 8. Search

- [ ] Online search:
  - Press `/`.
  - Type `from:<known-sender@example.com>`.
  - Press `Enter`.

  Expected: Gmail results appear for the active account.

- [ ] Run additional online Gmail queries:
  - `to:<account-a@example.com>`
  - `subject:<known subject word>`
  - `has:attachment`
  - `newer_than:30d`
  - `label:inbox`

  Expected: supported Gmail query syntax is passed through online and results are account-scoped.

- [ ] Offline search:
  - Press `/`.
  - Type a cached sender, recipient, subject, or body term.
  - Press `Ctrl+/` to switch to offline mode.
  - Press `Enter`.

  Expected: cached results return quickly by observation, under the PRD target of 100 ms for typical local searches.

- [ ] Open a search result:
  - Use `j`/`k` to select a result.
  - Press `Enter`.

  Expected: the reader opens the selected result's thread.

- [ ] Recent searches:
  - Press `/`.
  - Press `Ctrl+R`.
  - Use `j`/`k` to select a recent search.
  - Press `Enter` to rerun it.
  - Press `Ctrl+R` again, select a recent search, and press `x` to delete it.

  Expected: recents survive restart, rerun correctly, delete correctly, and remain scoped to the active account.

## 9. Compose, Drafts, And Attachments

- [ ] Compose and send a new message:
  - Press `c`.
  - Confirm G&T presents editable header fields for To, Cc, Bcc, Subject, and From.
  - Fill To with Account B, fill Subject with `G&T QA new message`, and enter a short body.
  - Press `Ctrl+S`.

  Expected: status reports `send complete`, and Account B receives the message. From cold start, this should be possible in under 30 seconds once accounts are already configured. If compose mode does not provide a way to edit recipients, subject, and body, mark this as a release blocker.

- [ ] Reply:
  - Open a message sent to Account A.
  - Press `r`.
  - Confirm the To and Subject fields are prefilled.
  - Add body text.
  - Press `Ctrl+S`.

  Expected: reply is sent in the original thread with quoted context. If body entry is unavailable, mark this as a release blocker.

- [ ] Reply-all:
  - Open a thread involving Accounts A, B, and C.
  - Press `R`.
  - Confirm original recipients are included except the active account's own address.
  - Press `Ctrl+S`.

  Expected: reply-all reaches the other test accounts and stays in the thread. If recipient review or body entry is unavailable, mark this as a release blocker.

- [ ] Forward:
  - Open a message.
  - Press `f`.
  - Address it to another test account.
  - Press `Ctrl+S`.

  Expected: forwarded message is received with forwarded context. If recipient entry is unavailable, mark this as a release blocker.

- [ ] Save and reopen a draft:
  - Press `c`.
  - Fill To and Subject.
  - Enter body text.
  - Press `Ctrl+D`.
  - Open Gmail web UI and confirm the draft exists.
  - In G&T, open the Drafts label and open the draft.

  Expected: the draft can be reopened and sent. If draft reopen does not enter an editable compose state, mark this as a release blocker.

- [ ] Attach a small file:
  - Create a local attachment:

    ```sh
    printf 'G&T QA attachment\n' > "$XDG_DOWNLOAD_DIR/gandt-qa-attachment.txt"
    ```

  - In compose mode, press `Ctrl+T`.
  - Type the full path printed by:

    ```sh
    printf '%s\n' "$XDG_DOWNLOAD_DIR/gandt-qa-attachment.txt"
    ```

  - Press `Enter`.
  - Press `Ctrl+S`.

  Expected: compose view lists the attachment and the recipient receives it.

- [ ] Download and open a received attachment:
  - Open the message with the received attachment.
  - Press `?` and look for the documented attachment download/open key or command.
  - Use that key or command for the selected attachment.

  Expected: the file is saved under `$XDG_DOWNLOAD_DIR` and cached under `$XDG_DATA_HOME/gandt/attachments/`; opening uses the platform opener. If no attachment download/open key or command is exposed in the UI, mark this as a release blocker.

## 10. Offline And Recovery

- [ ] Cached browsing offline:
  - Disable network access at the OS level.
  - Keep G&T running or relaunch it.
  - Browse cached labels and open cached messages.

  Expected: cached mail remains browsable and cached bodies render.

- [ ] Offline search:
  - Press `/`.
  - Enter a cached query.
  - Press `Ctrl+/` to use offline mode.
  - Press `Enter`.

  Expected: offline search still returns cached matches.

- [ ] Online failure state:
  - While still offline, run an online search or open an uncached thread.

  Expected: G&T shows a distinct offline/rate-limit style message and does not crash.

- [ ] Outbox retry:
  - While offline, compose a message to another test account.
  - Press `Ctrl+S`.
  - Re-enable network.
  - Run `:sync-all` or wait for retry.

  Expected: send is queued while offline and sent after reconnect.

- [ ] OAuth revoked:
  - In the Google Account security settings for one disposable account, revoke this OAuth client's access.
  - Return to G&T and run `:sync-all` or perform an online action for that account.

  Expected: G&T prompts for re-authentication of that account only. Re-authorize with `:add-account`.

- [ ] Keychain inaccessible:
  - In a disposable desktop session or VM, lock or disable the OS keychain provider.
  - Launch G&T or run an auth-backed action.

  Expected: G&T shows a keychain-inaccessible message. Do not put OAuth tokens or client secrets into `config.toml` as a workaround.

- [ ] Cache wipe on disposable data only:

  ```text
  :cache-wipe
  ```

  Press `y` for confirmation 1, then `y` for confirmation 2.

  Expected: SQLite cache files and cached attachments are removed, OAuth tokens remain in the keychain, and the app recreates schema cleanly on next startup.

## 11. Platform Smoke Checks

- [ ] Linux terminal smoke test:
  - Complete Sections 1 through 10 on Linux.

  Expected: no crash, unreadable screen, or broken key path.

- [ ] macOS terminal smoke test:
  - Build or install the same release candidate.
  - Run `gandt --version`.
  - Launch G&T.
  - Add one disposable account or reuse already-authorized local QA credentials.
  - Open a cached message, run `:cache`, compose a draft, and quit.

  Expected: Keychain, browser OAuth, terminal rendering, and platform opener work.

- [ ] Windows best-effort smoke test:
  - Build or install the Windows artifact.
  - Run `gandt --version`.
  - Launch G&T.
  - Confirm the no-account screen, help overlay, and clean quit.
  - If credentials are available, add one disposable account and open a cached message.

  Expected: no startup crash; Credential Manager and browser OAuth behavior are recorded if tested.

## 12. Release Verification

- [ ] Run final automated checks:

  ```sh
  make test
  make vet
  goreleaser release --snapshot --clean
  ```

  Expected: all pass. If GoReleaser fails, record the failure before tagging.

- [ ] Review logs for secrets and unexpected message body leakage:

  ```sh
  rg -n "access_token|refresh_token|client_secret|Authorization|Bearer" \
    "$XDG_CONFIG_HOME/gandt" "$XDG_DATA_HOME/gandt/logs" || true
  ```

  Expected: no OAuth token or client secret appears. Logs should also avoid message bodies and attachment content.

- [ ] Confirm release blockers:
  - No crash or unrecoverable screen during normal triage.
  - OAuth tokens and client secrets are only in the OS keychain.
  - Cache purge and cache wipe require confirmation.
  - Account removal affects only the selected account.
  - Cached message bodies and attachments are understood to be unencrypted local data.
  - Known deviations are documented in `docs/performance.md` or release notes.

- [ ] Record final result:
  - Date:
  - Commit:
  - OS and terminal:
  - Gmail test accounts used:
  - Passed sections:
  - Failed sections:
  - Release blockers:
  - Non-blocking follow-ups:
