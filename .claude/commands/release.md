---
description: Report and steer the automated release-please release
argument-hint: [status|force X.Y.Z]
allowed-tools: Bash(git:*), Bash(gh:*), Read, Edit, AskUserQuestion
---

# C3 Release

## Arguments

$ARGUMENTS

## Current State

- Manifest version: !`grep '"\."' "$CLAUDE_PROJECT_DIR/.release-please-manifest.json" | sed 's/.*: "\([^"]*\)".*/\1/'`
- Last git tag: !`cd "$CLAUDE_PROJECT_DIR" && git describe --tags --abbrev=0 2>/dev/null || echo "none"`
- Commits since: !`cd "$CLAUDE_PROJECT_DIR" && git log "$(git describe --tags --abbrev=0 2>/dev/null)"..HEAD --oneline --no-merges 2>/dev/null | head -20 || echo "(no tag)"`

## Releases are automated

`release-please` owns the version, the changelog, the tag, and the GitHub Release. **Do not bump
version files, write `CHANGELOG.md` entries, or create tags by hand** — a hand edit is either
overwritten by the next Release PR or fails `scripts/test_release_version_surfaces.py`.

Landing a conventional commit on `main` opens or updates a Release PR. Merging it releases.

## What this command is for

### Reporting status

From the state above, tell the user the version release-please would cut next and why, and whether
a Release PR is already open (`gh pr list --label 'autorelease: pending'`).

Derive the bump from the commits since the last tag:

| Commit | Bump |
|--------|------|
| `feat:` / `feat(scope):` | **minor** |
| `fix:` `docs:` `refactor:` `perf:` `build:` | **patch** |
| `feat!:` / any `!` suffix / `BREAKING CHANGE:` footer | **major** |
| `chore:` `ci:` `test:` | no release on their own |

`release-please-config.json` maps these onto the changelog sections `Added`, `Fixed`, `Changed`,
`Performance`, `Documentation`, `Build`; `ci`, `chore`, and `test` are hidden.

### Forcing a version

To release a version the commits would not produce, land a footer on `main`. Confirm the target
with the user via AskUserQuestion first.

```bash
git commit --allow-empty -m "chore: release 12.0.0" -m "Release-As: 12.0.0"
```

### Rewriting a release note

Edit `CHANGELOG.md` on the open Release PR's branch — release-please preserves manual edits there.

### Recovering a release whose assets never uploaded

`release.yml` re-runs are idempotent; the npm step checks whether that exact version already exists.

```bash
gh workflow run release.yml -f tag=v{VERSION}
```

## Adding a new version surface

If a release must start bumping a file that is not yet covered:

1. Add an `extra-files` entry to `release-please-config.json` — `json` with a jsonpath, or
   `generic` plus an inline `x-release-please-version` marker on the version line.
2. Add it to `GENERIC_SURFACES` or `JSON_SURFACES` in `scripts/test_release_version_surfaces.py`.

Step 2 is not optional. release-please's JSON updater silently updates nothing when a jsonpath
matches nothing, so that test is the only thing that catches a typo before a release ships
half-bumped.
