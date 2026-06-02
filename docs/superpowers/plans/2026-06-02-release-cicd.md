# Release CI/CD pipeline — plan & ledger

Date: 2026-06-02
Branch: `ci/release-pipeline`

## Problem

`.github/workflows/release.yml` triggers on `v*` tags but only cross-builds
`cmd/datjit` and uploads the raw binaries as **workflow artifacts**. Artifacts
are ephemeral (90-day retention, require a GitHub login to download) and are not
attached to a GitHub Release. There is no actual Release object created on tag
push, so `gh release`/the Releases page stays empty and `go install @vX.Y.Z`
users have no published binaries.

Secondary flaw: `cmd/datjit/main.go` hardcodes `const version = "0.1.0"`. A const
cannot be overridden at link time, so every released binary would report the
wrong version regardless of the tag it was built from.

## Goal

1. On every `v*` tag push, create a real GitHub Release with cross-compiled
   `datjit` binaries attached as release assets (linux/darwin × amd64/arm64,
   matching the documented matrix).
2. Attach a `SHA256SUMS` checksum file for integrity verification.
3. Populate the release body from the matching `CHANGELOG.md` section.
4. Stamp the real semver into each binary via `-ldflags -X main.version`.
5. Keep `make ci` green; keep CLAUDE.md "Releases" section accurate.

## Design

### Workflow `release.yml`

- Trigger: `push` tags `v*` (unchanged).
- Top-level `permissions: contents: write` (needed to create the Release).
- Job `build` (matrix linux/darwin × amd64/arm64):
  - checkout, setup-go from go.mod.
  - derive `VERSION=${GITHUB_REF_NAME#v}`.
  - `go build -trimpath -ldflags "-s -w -X main.version=$VERSION"`.
  - package `datjit` into `datjit_${VERSION}_${goos}_${goarch}.tar.gz`.
  - sha256 the archive; upload archive + per-target `.sha256` as artifacts.
- Job `release` (`needs: build`):
  - download all artifacts.
  - concatenate per-target sums into one `SHA256SUMS`.
  - extract the CHANGELOG section for the tag (awk between `## [x.y.z]` headers);
    fall back to a generic note if absent.
  - `gh release create "$TAG" --title --notes-file --verify-tag` with archives
    + `SHA256SUMS` as assets. `--prerelease` when the tag has a `-` suffix.

### Code change

- `cmd/datjit/main.go`: `const version` -> `var version = "dev"` so link-time
  injection works; default `dev` for plain `go build`/`go install`.
- `cmd/datjit/cli_test.go`: relax `TestVersion` regex to accept `dev` or any
  semver (was pinned to `0.1.0`).

### Docs

- CLAUDE.md "Releases": describe Release-object output instead of artifacts.
- CHANGELOG.md `[Unreleased]`: note the new release automation.

## Ledger

- [x] Investigate existing CI/CD, version wiring, changelog format.
- [x] Create worktree `../datjitgo-release-cicd` + branch `ci/release-pipeline`.
- [x] Write this plan/ledger.
- [x] Implement version ldflags hook (main.go + test).
- [x] Rewrite release.yml.
- [x] Update CLAUDE.md + CHANGELOG.
- [x] `make ci` green.
- [x] Lint workflow YAML (actionlint — CLEAN).
- [x] Self-review until no flaws (round 2 confirming).
- [ ] Commit, push, open PR.

## Review notes

### Round 1 (cavecrew-reviewer)
- 🔴 awk CHANGELOG extraction built a regex from the version string; a `+`
  build-metadata tag (e.g. `v1.0.0+build`) would corrupt the pattern and drop
  the notes. **Fixed**: switched to literal prefix matching via awk `index()`,
  so no version character is interpreted as a regex metacharacter. Re-verified:
  `0.2.1` extracts correctly; `1.0.0+build` returns empty (falls back to the
  generic note). No other flaws (artifact names, permissions, quoting, ldflags
  all clean).

### Verifications
- `go build -ldflags "-X main.version=9.9.9"` → `datjit v9.9.9` (injection works).
- `make ci` green (gofmt, lint, race tests, fixtures, build).
- `actionlint .github/workflows/release.yml` → CLEAN.
