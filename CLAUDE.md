# Working in this repo

This is the C3 **product source** — the Go CLI and the Claude skill that ships it. This file is the repo-dev contract: how to invoke C3 here, how we work, and how we release. It teaches no product behavior.

**What C3 is, the operation set, and the change-unit / freeze / canvas model are owned by `skills/c3/SKILL.md`.** Read it there; do not duplicate it here.

---

## Local C3 Source Rule

This repository is the C3 project source. Work here is intentionally outside the installed/global C3 skill scope — never let the global skill answer for this checkout.

Hard rules:
- Do not use bare `c3x`; it may resolve to the global installed skill.
- Do not load C3 from `~/.agents/skills/c3`, `~/.claude/skills/c3`, `~/.codex/skills/c3`, or marketplace installs for this repo.
- Load C3 skill instructions from `skills/c3/SKILL.md` in this checkout.
- Run C3 through the local built wrapper: `C3X_MODE=agent bash skills/c3/bin/c3x.sh <command>`.
- If `skills/c3/bin/c3x.sh` or the matching local binary is missing, tell the user — CI builds binaries, not you. Only run `bash scripts/build.sh` when explicitly debugging the build.
- The wrapper rebuilds only when the platform binary is **absent**; after editing CLI source, delete the stale gitignored binary in `skills/c3/bin/` so you don't dogfood old code.
- At session start, create a local alias/function and use it for every C3 command:

```bash
alias c3local='C3X_MODE=agent bash skills/c3/bin/c3x.sh'
c3local check
```

If C3 output looks wrong, commands fail unexpectedly, or behavior differs from source changes, suspect the wrong C3 version is being used. Prove the path/version before continuing.

File lookup: `c3local lookup <file-or-glob>` maps files/directories to components + refs.
CLI: `c3local <command>` after the alias above; see `skills/c3/SKILL.md` for the operation set.

---

## Workflow

- Load `/superpowers:brainstorming` and `/superpowers-developing-for-claude-code:developing-claude-code-plugins` up front.
- Use `AskUserQuestionTool` where possible — it yields a better-grounded answer.
- Start with brainstorming to pin the intention; align the concept in prose before offering implementation menus.
- Once understood, write the plan, then implement in parallel using subagents.
- Before claiming work is done: run `/noslop` to strip AI slop, then run the local C3 flow (`c3local check` for doc integrity + `c3local eval` for fact↔code conformance) to verify docs match code.
- Commit with conventional commits — that is what drives the release. Do not bump versions or write changelog entries by hand; see **Release Process**. `/release` reports what would be cut and how to steer it.

---

## Plugin Structure

### plugin.json

Auto-discovery only. Do NOT add explicit component paths.

```json
{
  "name": "c3-skill",
  "version": "x.x.x"
}
```

### Repository Layout

```
c3-design/
├── .claude-plugin/           # Plugin metadata
│   ├── plugin.json
│   └── marketplace.json
├── cli/                      # Go CLI source
│   ├── main.go
│   ├── cmd/                  # Command implementations
│   └── internal/             # Core libraries (content, store, schema, changeset,
│                             #   walker, codemap (glob match), eval, frontmatter, …)
├── packages/cli/             # npm @cuongtran001/c3x-cli thin client (downloads the binary)
├── skills/c3/                # Unified skill (auto-discovered)
│   ├── SKILL.md              # Skill definition + intent router
│   ├── bin/                           # CLI wrapper + version (binaries built in CI)
│   │   ├── c3x.sh                    # Platform-detecting wrapper (committed)
│   │   ├── VERSION                   # Current version, read by c3x.sh (committed)
│   │   ├── AST_GREP_VERSION          # Pinned ast-grep version for outline gathers (committed)
│   │   └── c3x-{version}-{os}-{arch} # Cross-compiled binaries (gitignored; local
│   │                                 #   builds accumulate here, only the matching
│   │                                 #   platform/version is used)
│   └── references/           # Operation-specific guidance (10 files)
│       ├── onboard.md
│       ├── query.md
│       ├── audit.md
│       ├── change.md
│       ├── canvas.md
│       ├── ref.md
│       ├── rule.md
│       ├── sweep.md
│       └── eval.md           # conformance: a fact's claim vs the external it governs
├── scripts/
│   ├── build.sh                          # Cross-compile Go CLI (debug-only; CI owns the build)
│   └── test_release_version_surfaces.py  # Guards every release-please-managed version surface
├── release-please-config.json            # Which files release-please bumps, and how
└── .release-please-manifest.json         # The version — single source of truth
```

### Build System

**Do NOT run `bash scripts/build.sh` during normal releases.** CI owns the build. `release.yml` validates version surfaces, runs tests, builds supported platform assets, assembles skill archives, uploads them to the release release-please already created, and publishes `@cuongtran001/c3x-cli` with the `NPM_TOKEN` repository secret when that exact version is not already on npm. Only run `build.sh` locally when debugging a build issue.

```bash
cd cli && go test ./...       # Run Go tests locally
```

### CI/CD

- **Pull request** -> `ci.yml`: the version-surface gate first, then Go, npm, and skill-packaging tests. This is the only place a release-please misconfiguration is caught *before* a tag exists.
- **Push to `main`** -> `release-please.yml`: opens or updates a Release PR that bumps every version surface and writes `CHANGELOG.md` from the conventional commits since the last release. Merging that PR makes release-please create tag `v{VERSION}` and the GitHub Release.
- **`release-please.yml` runs under the `RELEASE_PLEASE_TOKEN` PAT**, not `GITHUB_TOKEN`. GitHub raises no event for anything `GITHUB_TOKEN` creates, so under the default token the Release PR arrives with `ci.yml` never having run on it — the one PR carrying release-please's own output would be the only ungated one. The PAT needs **Contents: read+write** and **Pull requests: read+write** on this repo; rotating it means re-running `gh secret set RELEASE_PLEASE_TOKEN`.
- **`release-please.yml` then calls `release.yml`** through `workflow_call`, gated on `release_created`. `release.yml` deliberately has **no `release:` trigger**: under the PAT that event does fire, and a listener on it would duplicate this call — two builds racing `gh release upload --clobber` over identical filenames.
- **`release.yml`** re-asserts all seven version surfaces against the tag, runs tests, cross-compiles `linux/amd64`, `linux/arm64`, and `darwin/arm64`, assembles assets, uploads them to the existing release, and publishes `@cuongtran001/c3x-cli` when that version is not already on npm. It never creates a tag or a release. Dispatch it manually with a `tag` input to rebuild assets; re-runs are idempotent.
- **`distribute.yml`** and **`npm-publish.yml`** are `workflow_dispatch`-only holdovers. Neither is part of the release path.

### Release Process

Releases are automated. You do not bump versions or write changelog entries by hand.

1. Land work on `main` with conventional commits — `feat:` bumps minor, `fix:`/`docs:`/`refactor:`/`perf:` bump patch, `feat!:` or a `BREAKING CHANGE:` footer bumps major.
2. `release-please.yml` opens a Release PR. Review the version bump and the generated `CHANGELOG.md` entry.
3. Merge it. The tag, the GitHub Release, the assets, and the npm publish all follow.
4. Verify with `gh run watch`, `gh release view v{VERSION}`, and `npm view @cuongtran001/c3x-cli version`.

To force a specific version, add a `Release-As: X.Y.Z` footer to a commit on `main`. See `/release` for the steering details.

### Versioning

`.release-please-manifest.json` is the single source of truth. Every file below is **derived** from it by release-please and must never be edited by hand:

| File | Purpose |
|------|---------|
| `.release-please-manifest.json` | Source of truth — the version release-please fans out |
| `skills/c3/bin/VERSION` | Version c3x.sh and build.sh resolve binaries against |
| `.claude-plugin/plugin.json` | Plugin metadata |
| `.claude-plugin/marketplace.json` | Marketplace listing |
| `packages/cli/package.json` | npm `@cuongtran001/c3x-cli` thin-client name + version |
| `packages/cli/package-lock.json` | npm lockfile (two `version` fields) |
| `packages/cli/src/version.ts` | `C3X_VERSION` the npm wrapper pins + downloads; also `NPM_PACKAGE` and the `RELEASE_REPO_SLUG` release assets resolve from |

`release-please-config.json` declares the updater for each. Adding a new surface means adding an
`extra-files` entry **and** a case in `scripts/test_release_version_surfaces.py` — release-please's
JSON updater silently does nothing when a jsonpath matches nothing, so that test is the only thing
standing between a typo and a half-bumped release.

**Version files carry the version as their first whitespace-separated token**; the rest of the line
is free for the `x-release-please-version` marker. Read them with `awk 'NF {print $1; exit}'` in
shell and `.split()[0]` in Python — never the whole file.

`skills/c3/bin/AST_GREP_VERSION` is **not** release-please managed. It pins the ast-grep runtime and
must stay equal to `AST_GREP_VERSION` in `packages/cli/src/version.ts`; bump both together by hand.
