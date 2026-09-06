# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-09-07

### Added

- `internal/gate` package that centralizes merge-request orchestration logic, leaving `main` as a thin wiring layer.
- Structured logging via `log/slog` for all application output.
- `context.Context` propagation through every GitLab client method down to the HTTP requests.
- 30-second default timeout on the GitLab HTTP client to avoid indefinite hangs.
- A 2-minute deadline on the whole run, so a stalled GitLab API cannot hang the Atlantis apply step.
- Pagination for GitLab list endpoints, capped at 100 requests per call and rejecting malformed `X-Next-Page` headers, so a misbehaving API cannot loop forever.
- Nix flake (`flake.nix`, `flake.lock`) for a reproducible development environment.
- `Taskfile.yml` for local automation (codegen, lint, security, test, build).
- golangci-lint configuration (`.golangci.yml`) with `revive` and `misspell`.
- README project logo and refreshed badges.
- This changelog.

### Changed

- Bumped Go to 1.26.7 and updated all Go module dependencies.
- Generate mocks with `go.uber.org/mock` via the `go tool` directive instead of hand-written mocks; generated mocks are no longer committed.
- Migrated build and test automation from `Makefile` to `Taskfile.yml`.
- CI now pins all GitHub Actions to commit SHAs, runs golangci-lint, and drives build/test through Taskfile.
- The release workflow authenticates GoReleaser with the workflow-provided `GITHUB_TOKEN` instead of a 1Password-sourced PAT.
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

- `RESTRICTED` mode now measures approval freshness against the merge request's latest diff version, a push time recorded by GitLab itself. It previously used the newest commit's timestamp, which is the git committer date and could be backdated by the merge request author to make a stale approval pass.
- The GitLab client no longer follows HTTP redirects, so its `Private-Token` header cannot be forwarded to another host. Go strips only `Authorization` and `Cookie` when a redirect crosses hosts, leaving a custom auth header exposed.
- Wired gosec and govulncheck into local automation; both now also run in CI as independent jobs.

[Unreleased]: https://github.com/shini4i/atlantis-emoji-gate/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/shini4i/atlantis-emoji-gate/compare/v0.4.0...v0.5.0
