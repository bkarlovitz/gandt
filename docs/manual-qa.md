# Manual QA Checklist

Use this checklist before tagging a v1 release candidate. These steps require real test Gmail accounts and must not run in automated CI.

## Test Matrix

- [ ] macOS terminal smoke test
- [ ] Linux terminal smoke test
- [ ] Windows best-effort smoke test
- [ ] At least two Gmail test accounts with Google Desktop OAuth client credentials
- [ ] Network available, then intentionally disabled for offline checks

## Install And Launch

- [ ] Install from a snapshot artifact or local `make build`.
- [ ] Run `gandt --version` and confirm the release version is injected.
- [ ] Launch `gandt` with no accounts and confirm the no-account/fake-inbox state renders.
- [ ] Confirm `NO_COLOR=1 gandt` renders without ANSI color.

## Account Setup And Removal

- [ ] Run `:add-account` for account A.
- [ ] Complete browser OAuth and confirm account A appears in the mailbox.
- [ ] Run `:add-account` for account B.
- [ ] Confirm `Ctrl+A` opens the account switcher.
- [ ] Confirm `1` and `2` switch accounts.
- [ ] Run `:remove-account` on a test account and confirm prompts prevent accidental removal.
- [ ] Re-add the removed test account.

## Sync And Cache

- [ ] Confirm initial labels and messages appear after backfill.
- [ ] Run `Ctrl+R` and confirm delta sync status.
- [ ] Run `:sync-all` and confirm all accounts are refreshed.
- [ ] Run `:sync-label` from a selected label.
- [ ] Open `:cache` and verify totals are plausible.
- [ ] Open `:cache-policy`, edit a policy, save, and confirm the next sync honors it.
- [ ] Run `:cache-exclude sender <test-sender>` with preview, then cancel.
- [ ] Run `:cache-purge --label INBOX --older-than 365d --dry-run`.
- [ ] Run `:cache-compact`.

## Reading And Rendering

- [ ] Open cached plaintext thread.
- [ ] Open HTML-only thread.
- [ ] Cycle `V` through plaintext, html2text, raw HTML, and Glamour.
- [ ] Confirm links show footnotes when enabled.
- [ ] Confirm images show placeholders.
- [ ] Confirm `B` opens the message in Gmail web UI.
- [ ] Confirm `z` toggles quoted text.

## Triage

- [ ] Archive a message and confirm optimistic removal.
- [ ] Undo with `U`.
- [ ] Trash a message.
- [ ] Mark spam on a disposable test message.
- [ ] Star and unstar a message.
- [ ] Mark read/unread.
- [ ] Add and remove a label.
- [ ] Mute a thread.
- [ ] Confirm all operations affect the active account only.

## Search

- [ ] Run online search with a Gmail query such as `from:test@example.com`.
- [ ] Toggle offline search with `Ctrl+/`.
- [ ] Confirm offline search returns cached matches in under 100 ms by observation.
- [ ] Open a result and confirm the reader loads the selected thread.
- [ ] Open recent searches with `Ctrl+R`, rerun one, and delete one.

## Compose, Drafts, And Attachments

- [ ] Compose a new message with `c`.
- [ ] Send to a test recipient.
- [ ] Reply with `r`.
- [ ] Reply-all with `R`.
- [ ] Forward with `f`.
- [ ] Save a draft and confirm it appears in Drafts.
- [ ] Reopen a draft.
- [ ] Attach a small file.
- [ ] Download a received attachment.
- [ ] Open a cached attachment.
- [ ] Force a send failure while offline and confirm outbox retry behavior after reconnect.

## Offline And Error Recovery

- [ ] Disable network and confirm cached browsing still works.
- [ ] Confirm offline search still works for cached messages.
- [ ] Attempt online search and confirm a distinct offline/rate-limit style message.
- [ ] Re-enable network and run sync.
- [ ] Revoke OAuth for a test account and confirm G&T prompts re-authentication.
- [ ] Lock or disable the keychain provider in a disposable environment and confirm a keychain-inaccessible message.
- [ ] Exercise `:cache-wipe` only on disposable data and confirm two prompts are required.

## Release Blockers

- [ ] No crash or unrecoverable screen during normal triage.
- [ ] No OAuth token, client secret, message body, or attachment content appears in logs.
- [ ] `goreleaser release --snapshot --clean` succeeds.
- [ ] `make test` succeeds.
- [ ] Known deviations are documented in `docs/performance.md` or release notes.
