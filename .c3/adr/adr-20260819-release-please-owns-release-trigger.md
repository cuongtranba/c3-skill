---
id: adr-20260819-release-please-owns-release-trigger
c3-seal: e53541c17e529f9b4b42e8287e21ca8ab833a11727bc1aeeeae497f6aed41ace
title: release-please-owns-release-trigger
type: adr
goal: 'Move release creation from a hand-pushed `v*` tag into release-please, and record the consequence in the two frozen facts that still describe the old trigger: `ref-cross-compiled-binary`, whose How names the tag push as what starts the release build, and `c3-203`, whose derived-material row for `skills/c3/bin/VERSION` assumes that file''s whole content is the version string.'
status: done
date: "2026-08-19"
---

# release-please owns the release trigger

## Goal

Move release creation from a hand-pushed `v*` tag into release-please, and record the consequence in the two frozen facts that still describe the old trigger: `ref-cross-compiled-binary`, whose How names the tag push as what starts the release build, and `c3-203`, whose derived-material row for `skills/c3/bin/VERSION` assumes that file's whole content is the version string.

## Context

Releases were driven by hand: a human bumped seven version surfaces, wrote a `CHANGELOG.md` entry, and pushed to `main`, where `release.yml` re-validated the surfaces and created the tag and GitHub Release. `distribute.yml` also fired on any pushed `v*` tag. The seven manual edits were the defect — each release was an opportunity to forget one, and nothing ran on pull requests to catch it.

release-please now owns the version. `.release-please-manifest.json` is the single source of truth and it fans out to every surface through `release-please-config.json`. Two mechanics of that tool reach into the frozen facts:

First, release-please creates the tag and the GitHub Release itself, when its Release PR merges; the release workflow is chained from it and only uploads assets. The `v*` tag push is no longer what starts a build — `distribute.yml` is now `workflow_dispatch`-only precisely so a stray tag cannot race the real path.

Second, release-please can only rewrite a version in a non-JSON file on a line carrying an `x-release-please-version` marker; there is no updater for a bare whole-file version. So `skills/c3/bin/VERSION` now reads `11.9.0 # x-release-please-version`, and every reader takes the first whitespace-separated token instead of the whole file.

## Decision

Patch exactly the two blocks that became false, and nothing else.

`ref-cross-compiled-binary`'s How is re-worded so the release build is started by the release release-please creates, not by a tag push into the distribute workflow. The build matrix, the `CGO_ENABLED=0` portable variant, and the assembly split across npm assets and skill ZIPs are unchanged and stay verbatim — this decision moves *what triggers* the build, not *what it produces*.

`c3-203`'s derived-material row for `skills/c3/bin/VERSION` records the marker: the allowed variance now admits trailing marker text on the version line, and the evidence names the first-whitespace-token rule that makes the file's version recoverable. The wrapper's Purpose still says "read VERSION" and stays frozen as written — the wrapper still reads that file, and how it parses a line is exactly the kind of mechanic the Contract row already lets vary.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| ref-cross-compiled-binary | N.A - governing ref update | Its How named a `v*` tag push into the distribute workflow as the release trigger; release-please now creates the tag and release, and distribute.yml is dispatch-only. | ref-cross-compiled-binary#n1095@v1:sha256:27b3090905b9b43db148df20628edafdce6427a1ad7170058818a33c9524040a | Confirm the asset names and platform matrix are untouched; only the trigger sentence moves. |
| c3-203 | component | Its `skills/c3/bin/VERSION` derived-material row assumed the file holds the version and nothing else; the file now carries a release-automation marker on the same line. | c3-203#n985@v1:sha256:ecf94fb7200ca8fbfaf9284c007073712b44391550ea8cd7c66085ddae56f39b | Confirm the wrapper's fallback ladder, platform gate, and Contract row stay frozen; only the VERSION material row changes. |

## Compliance Refs

| Ref | Why required | Evidence | Action |
| --- | --- | --- | --- |
| ref-cross-compiled-binary | It fixes the per-platform asset names and the linux amd64/arm64 + darwin arm64 matrix that both the wrapper and the downloader resolve against. This unit changes what starts the build without touching what it emits, so the ref needs its trigger sentence corrected and the rest shown to still hold. | ref-cross-compiled-binary#n1095@v1:sha256:27b3090905b9b43db148df20628edafdce6427a1ad7170058818a33c9524040a | update-ref — patch the How trigger sentence only. |
| ref-fat-thin-distribution | It requires the no-binary artifact to delegate to the npm manager pinned to the same VERSION, which is the surface the marker change could have broken. The wrapper still derives that pin from the same file, now by first token. | ref-fat-thin-distribution#n1113@v1:sha256:adbf4f83982dc0e8cc7e0dce5468f4a3cd1db8b23dcdf99c6f65d31f7f9605a1 | comply — no patch. The pinned package spec and the version it resolves are unchanged. |

## Enforcement Surfaces

| Surface | Behavior | Evidence |
| --- | --- | --- |
| scripts/test_release_version_surfaces.py | Fails when any surface diverges from the manifest, when a declared jsonpath resolves to nothing, when a `generic` surface loses its marker, or when a reader parses the whole VERSION file again. | `python3 scripts/test_release_version_surfaces.py` |
| .github/workflows/ci.yml | Runs that gate on every pull request, so a release-please misconfiguration is caught before a tag exists. | `.github/workflows/ci.yml` step "Release version surfaces" |
| .github/workflows/release.yml | The plan job re-asserts all seven surfaces against the release tag and refuses to proceed on a mismatch; it can no longer create a tag or a release. | `.github/workflows/release.yml` plan job |

## Risks

| Risk | Mitigation | Verification |
| --- | --- | --- |
| A jsonpath typo leaves a surface un-bumped, because release-please's JSON updater no-ops silently instead of erroring. | The surface test resolves every declared jsonpath against the real file and fails on anything but exactly one semver string; the release plan job re-asserts the same set against the tag. | `python3 scripts/test_release_version_surfaces.py` |
| A VERSION reader is missed and resolves a binary named after the marker text. | The surface test forbids the whole-file read patterns in every known reader, and a `dev` build proves the name end-to-end. | `bash scripts/build.sh --version dev --out-dir /tmp/c3x-probe` emits `c3x-11.9.0-<os>-<arch>-fat` |

## Verification

| Check | Result |
| --- | --- |
| `python3 scripts/test_release_version_surfaces.py` | Passes: every surface equals the manifest, every jsonpath resolves, the marker is present, no reader parses the whole file. |
| `bash scripts/build.sh --version dev --out-dir /tmp/c3x-probe` | Emits `c3x-11.9.0-<os>-<arch>-fat`, proving the first-token read. |
| `C3X_MODE=agent bash skills/c3/bin/c3x.sh check` | Clean: the patched facts stay canvas-valid and every citation resolves. |
| `C3X_MODE=agent bash skills/c3/bin/c3x.sh eval` | No drift for c3-203 or ref-cross-compiled-binary. |
