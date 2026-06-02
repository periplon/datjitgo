# Bump GitHub Actions to Node 24 — plan & ledger

Date: 2026-06-02
Branch: `chore/actions-node24`

## Why

The v0.3.0 release run emitted Node 20 deprecation warnings. GitHub forces
Node 24 for JavaScript actions on 2026-06-16 and removes Node 20 from runners
on 2026-09-16. Bump the pinned actions to their current Node 24 majors.

## Current pins → target (latest Node 24 majors, verified via GitHub API)

| Action | Now | Target | Runtime |
|---|---|---|---|
| actions/checkout | v4 (node20) | **v6** | node24 |
| actions/setup-go | v5 (node20) | **v6** | node24 |
| actions/upload-artifact | v4 (node20) | **v7** | node24 |
| actions/download-artifact | v4 (node20) | **v8** | node24 |
| golangci/golangci-lint-action | v9 (node24) | v9 (unchanged) | node24 |

Major-only pins kept (matches existing style; picks up patch releases).

## Breaking-change review (only our usage matters)

- **upload-artifact v4→v7**: inputs `name`, `path`, `if-no-files-found`
  unchanged. v7 adds an optional `archive` input (default keeps zip + honours
  `name`); we don't set it. Safe.
- **download-artifact v4→v8**: `merge-multiple` input persists v5–v8 with the
  same flatten-into-one-dir behavior; `path` unchanged. v8 adds optional
  `skip-decompress`/`digest-mismatch`; we don't use them. Safe.
- upload v7 ↔ download v8 interoperate: all v4+ use the same immutable-artifact
  backend (the hard break was v3→v4). Mixed majors fine.
- checkout v6 / setup-go v6: drop-in for our usage (`go-version-file`, `cache`).

## Files

- `.github/workflows/ci.yml`: checkout v4→v6, setup-go v5→v6, upload-artifact
  v4→v7. golangci-lint-action@v9 untouched.
- `.github/workflows/release.yml`: checkout v4→v6, setup-go v5→v6,
  upload-artifact v4→v7, download-artifact v4→v8.

## Ledger

- [x] Worktree `../datjitgo-actions` + branch.
- [x] Verify latest Node 24 majors + breaking changes (GitHub API + release notes).
- [x] Write plan/ledger.
- [x] Edit ci.yml + release.yml.
- [x] actionlint clean.
- [x] Self-review (cavecrew-reviewer): CLEAN.
- [ ] Commit, push, open PR.

## Review notes

### Round 1 (cavecrew-reviewer) — CLEAN
- All `uses:` now node24; no node20 action left in `.github/`.
- upload-artifact@v7 still accepts `name`/`path`/`if-no-files-found: error`.
- download-artifact@v8 still accepts `path`/`merge-multiple: true`.
- upload@v7 ↔ download@v8 compatible (shared v4+ backend).
- golangci-lint-action@v9 confirmed node24, correctly left unchanged.
- All target majors exist (checkout v6, setup-go v6, upload v7, download v8).
