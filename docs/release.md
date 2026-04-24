# Release Packaging

G&T uses GoReleaser for v1 snapshot and tagged builds.

Snapshot verification:

```sh
goreleaser release --snapshot --clean
```

The GoReleaser config builds static `gandt` binaries for Linux, macOS, and Windows on amd64 and arm64 with version injection through `main.version`.

Homebrew tap metadata is present in `.goreleaser.yaml` as a placeholder for `bkarlovitz/homebrew-gandt`. Before the first public tag, confirm the tap repository exists, the license field matches the final project license, and the generated formula installs from the release artifacts.
