# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `internal/gate` package that centralizes merge-request orchestration logic, leaving `main` as a thin wiring layer.
- Structured logging via `log/slog` for all application output.
- `context.Context` propagation through every GitLab client method down to the HTTP requests.
- 30-second default timeout on the GitLab HTTP client to avoid indefinite hangs.
- Pagination for GitLab list endpoints (award emojis), with a safety limit to guard against runaway loops.
- Nix flake (`flake.nix`, `flake.lock`) for a reproducible development environment.
- `Taskfile.yml` for local automation (codegen, lint, security, test, build).
- golangci-lint configuration (`.golangci.yml`) with `revive` and `misspell`.
- README project logo and refreshed badges.
- This changelog.

### Changed

- Bumped Go to 1.25.11.
- Generate mocks with `go.uber.org/mock` via the `go tool` directive instead of hand-written mocks; generated mocks are no longer committed.
- `GetLatestCommitTimestamp` now fetches only the most recent commit instead of paginating the entire commit history.
- Migrated build and test automation from `Makefile` to `Taskfile.yml`.
- CI now pins all GitHub Actions to commit SHAs, runs golangci-lint, and drives build/test through Taskfile.
- Upgraded GoReleaser configuration to v2 with a grouped, conventional-commit changelog.
- Replaced `panic` on configuration errors with `slog.Error` followed by `os.Exit(1)`.
- Go idiom cleanups: unexported GitLab client fields, `URL` initialism naming, `any` over `interface{}`, and early-return config parsing.

### Fixed

- Only the first page of paginated GitLab responses was read, so approvals on merge requests with many emoji reactions could be silently missed.
- File paths are now URL-encoded in `GetFileContent`, so CODEOWNERS files in subdirectories (e.g. `.github/CODEOWNERS`) resolve correctly.
- Leading slashes are stripped from CODEOWNERS patterns so absolute-style patterns (e.g. `/terraform`) match the slash-less Atlantis `REPO_REL_DIR`.

### Removed

- `Makefile` (replaced by `Taskfile.yml`).
- `GetMrCommits` from the public GitLab client API.

### Security

- Wired gosec and govulncheck into local automation; gosec runs in CI.

[Unreleased]: https://github.com/shini4i/atlantis-emoji-gate/compare/v0.4.0...HEAD
