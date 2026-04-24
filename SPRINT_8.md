# Sprint 8: Compose And Attachments - Draft, Reply, Send, Download

## Objective
Users can compose new mail, reply, reply-all, forward, autosave Gmail drafts, send mail with retry, and fetch or send attachments across configured accounts.

## Source Context
Grounded in `prd.md` sections 8, 9, 13.3, 13.5, 13.7, 13.8, 14, 15, 17, 19, 20, and milestone M5.

## Tasks
- [ ] **Task 8.1**: Implement compose data model
  - Add typed structs for compose headers, recipients, body source, attachments, draft IDs, reply context, and send state.
  - Validate email address fields enough to catch obvious local errors before Gmail API calls.
  - Keep active account and `sendAs` alias explicit.
  - Validation: `go test ./internal/compose/...` covers validation, reply context, forward context, and attachment metadata.

- [ ] **Task 8.2**: Fetch send-as identities
  - Add Gmail wrapper for `users.settings.sendAs.list`.
  - Cache or memoize aliases per account for compose header dropdowns.
  - Fall back to the account email if alias fetch fails.
  - Validation: `go test ./internal/gmail ./internal/compose` covers aliases, fallback, and account scoping.

- [ ] **Task 8.3**: Build compose header forms
  - Use Huh for To, Cc, Bcc, Subject, and From fields.
  - Support new, reply, reply-all, and forward prefill rules.
  - Keep forms usable in narrow terminal layouts.
  - Validation: `go test ./internal/ui/...` covers form initialization, validation errors, cancel, and submit states.

- [ ] **Task 8.4**: Implement external editor integration
  - Open `$EDITOR` on a temporary file with prefilled quoted content where relevant.
  - Fall back to inline textarea when `$EDITOR` is unset or config forces inline mode.
  - Clean up temp files without losing text on editor failure.
  - Validation: `go test ./internal/compose/...` uses fake editor commands for success, failure, unset editor, and tempfile cleanup.

- [ ] **Task 8.5**: Implement inline compose body editor
  - Use Bubbles textarea for inline mode.
  - Support `Ctrl+E` to open the current body in `$EDITOR`.
  - Preserve body contents through resize, validation errors, and draft save.
  - Validation: `go test ./internal/ui/...` covers inline editing, external handoff, resize, and cancel confirmation.

- [ ] **Task 8.6**: Implement reply, reply-all, and forward quoting
  - Add attribution line `On $date, $from wrote:`.
  - Prefix quoted lines with `> `.
  - Reply-all includes original To and Cc minus the user's own address.
  - Validation: `go test ./internal/compose/...` includes golden tests for reply, reply-all, forward, and self-address removal.

- [ ] **Task 8.7**: Implement MIME assembly
  - Assemble RFC 822 messages for plain text bodies, HTML fallback where needed, and attachments.
  - Encode headers, recipients, body, and attachment parts correctly.
  - Keep raw message generation deterministic for tests.
  - Validation: `go test ./internal/compose/...` parses generated MIME and verifies headers, body, and attachment parts.

- [ ] **Task 8.8**: Implement Gmail drafts
  - Add wrappers for drafts list, get, create, update, delete, and send.
  - Autosave every 30 seconds while compose is dirty.
  - Reopen Gmail drafts from the Drafts label into compose view.
  - Validation: `go test ./internal/gmail ./internal/compose ./internal/ui` covers create/update/delete/send, autosave timing, and reopen behavior.

- [ ] **Task 8.9**: Implement send and outbox retry
  - Add wrappers for `users.messages.send` and draft send.
  - On network failure, write raw RFC 822 bytes into `outbox` with pending status.
  - Retry with exponential backoff when connectivity returns and mark sent or failed.
  - Validation: `go test ./internal/compose ./internal/sync ./internal/cache` covers successful send, queued send, retry success, retry failure, and outbox persistence.

- [ ] **Task 8.10**: Build compose mode UI actions
  - Wire `c`, `r`, `R`, and `f` from mailbox and reader contexts.
  - Add `Ctrl+D` save draft and close, `Ctrl+S` send, `Ctrl+C` discard with confirmation, and `Ctrl+T` attach.
  - Show clear send, autosave, queued, and error states.
  - Validation: `go test ./internal/ui/...` simulates each compose key path and checks resulting state.

- [ ] **Task 8.11**: Implement attachment download
  - Add wrapper for `users.messages.attachments.get`.
  - Save selected attachment to `$XDG_DOWNLOAD_DIR` or configured downloads path.
  - Cache fetched attachment bytes under `$XDG_DATA_HOME/gandt/attachments/<account_id>/<message_id>/<filename>`.
  - Validation: `go test ./internal/gmail ./internal/cache ./internal/ui` covers attachment fetch, safe filenames, configured path, and cached local path.

- [ ] **Task 8.12**: Implement attachment open
  - Open downloaded attachments with `open`, `xdg-open`, or `start` depending on platform.
  - Fetch lazily first if the bytes are not already cached.
  - Show an error when the platform opener is unavailable.
  - Validation: `go test ./internal/compose ./internal/ui` uses fake platform openers for success, missing opener, and lazy fetch behavior.

- [ ] **Task 8.13**: Implement attachment send picker
  - Add `Ctrl+T` attachment selection using a file picker component or path input.
  - Validate file existence, size, and readability before adding it to the message.
  - Include selected attachment size/type in compose UI.
  - Validation: `go test ./internal/ui ./internal/compose` covers add, remove, missing file, unreadable file, and MIME inclusion.

- [ ] **Task 8.14**: Update Gmail state after compose operations
  - After send or draft update, refresh affected labels and threads.
  - Ensure sent/draft messages route to the correct account.
  - Keep the UI consistent if a send succeeds after being queued.
  - Validation: `go test ./internal/sync ./internal/ui` covers post-send refresh, draft label update, and outbox sent transition.

- [ ] **Task 8.15**: Verify the M5 acceptance path
  - From cold start, compose and send a new message in under 30 seconds with a test Gmail account.
  - Reply, reply-all, forward, attach a file, save a draft, reopen it on another Gmail surface, and send.
  - Disable networking, send to outbox, restore networking, and confirm retry sends the queued mail.
  - Validation: manual QA confirms M5 acceptance criteria from `prd.md` across at least two accounts.
