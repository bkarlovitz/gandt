# G&T v1 Performance Notes

Measured on 2026-04-24 on Linux amd64, AMD Ryzen 5 7530U, Go 1.25, with `CGO_ENABLED=0`.

## Automated Benchmarks

Command:

```sh
go test ./internal/ui ./internal/cache -run '^$' -bench 'Benchmark(ColdStartNoAccountView|MailboxRender5000|CachedThreadOpen5000|SearchResultsRender100|TriageOptimisticAction5000|MailboxMemory10000|MessageRepositoryListSummariesByLabel5000|MessageRepositorySearchSummaries5000)' -benchmem
```

| Metric | Result | PRD target | Status |
|--------|--------|------------|--------|
| Cold start no-account view construction | 0.053 ms | <300 ms | Meets target for UI construction; full terminal startup still needs manual QA |
| Cached inbox render, 5k messages | 25.3 ms | <100 ms | Meets target |
| Cached thread open, 5k-message mailbox | 0.004 ms | <50 ms | Meets target |
| Offline search over 5k cached messages | 32.2 ms | <100 ms | Meets target |
| Search results render, 100 rows | 0.72 ms | <100 ms render budget | Meets target |
| Triage optimistic action, 5k-message mailbox | 4.9 ms | <50 ms perceived latency | Meets target |
| 10k cached message UI model allocation | 3.5 MB allocated/op | <150 MB resident memory | Allocation benchmark is comfortably below target; resident memory still needs release QA |
| Binary size, static build | 29,528,248 bytes | <30 MB | Meets target narrowly |

## Manual Or Environment-Dependent Checks

- Online Gmail search round-trip was not executed in this automated pass because it requires real OAuth credentials and a test Gmail account. It remains part of the Sprint 9 manual QA checklist and must meet the PRD's <1s typical-query target or be documented as a release deviation.
- Cache-miss thread open over broadband was not executed in this automated pass for the same reason. The cached path is well below target; live fetch timing remains part of release QA.
- A direct `gandt --version` process startup measurement was about 0.47s with `/usr/bin/time`, which includes process startup and module initialization but does not represent the interactive TUI path.

## Index And Render Review

The existing SQLite schema already has the indexes needed by the measured hot paths: label summaries use `idx_msglabels_label`, thread opens use `idx_messages_thread_date`, recent searches use `idx_recent_searches_used`, and offline search uses `messages_fts`. No additional index was added during this pass because the measured cached and offline paths meet the v1 targets.
