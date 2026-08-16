---
id: adr-20260811-fork-npm-client-identity
c3-seal: 1ffc55368cce2342971b3521ae5043253647f302abeded247cad4c010cb70f1a
title: fork-npm-client-identity
type: adr
goal: Re-point every frozen fact that names the project's npm client and its runtime-asset origin from the upstream identity (`@c3x/cli`, published from `lagz0ne/c3-skill`) to this fork's own identity (`@cuongtran001/c3x-cli`, published from and downloading assets out of `cuongtranba/c3-skill`), so the distribution facts describe the package this repository actually ships and the releases its runtime manager actually fetches.
status: accepted
date: "2026-08-11"
---

# fork-npm-client-identity

## Goal

Re-point every frozen fact that names the project's npm client and its runtime-asset origin from the upstream identity (`@c3x/cli`, published from `lagz0ne/c3-skill`) to this fork's own identity (`@cuongtran001/c3x-cli`, published from and downloading assets out of `cuongtranba/c3-skill`), so the distribution facts describe the package this repository actually ships and the releases its runtime manager actually fetches.

## Context

This repository is a fork of `lagz0ne/c3-skill`. The npm name `@c3x/cli` is owned solely by upstream, so the fork's release workflow could not publish: `npm publish` returned 404 for a package the fork's maintainer has no rights to, and the trusted-publisher binding for that name points at upstream's repository. The fork therefore had a release path that always failed at the npm step, and — because the wrapper's no-binary fallback resolves `@c3x/cli@${VERSION}` — any version the fork cut had no installable runtime at all.

The code on this branch already carries the fix: `packages/cli/package.json` is renamed to `@cuongtran001/c3x-cli` with the fork's repository URL; `packages/cli/src/version.ts` adds `NPM_PACKAGE` and `RELEASE_REPO_SLUG` constants; `packages/cli/src/manager.ts` derives `RELEASE_REPO` and `RELEASES_API` from `RELEASE_REPO_SLUG` so runtime assets resolve from this fork's GitHub Releases, and uses `NPM_PACKAGE` in its unsupported-platform and download-failure hints; `skills/c3/bin/c3x.sh` delegates to `npm exec --yes --package "@cuongtran001/c3x-cli@${VERSION}" -- c3x`; and `.github/workflows/release.yml` reads the package name from `packages/cli/package.json` and publishes with token auth instead of OIDC trusted publishing.

The frozen facts still describe the upstream identity, so the documented distribution story and the shipped one disagree. The file-context gate (`c3x lookup`) maps the changed runtime files to exactly two components — `packages/cli/src/**` to c3-301 and `skills/c3/bin/c3x.sh` to c3-203 — each governed by ref-cross-compiled-binary and ref-fat-thin-distribution, and returns no `rule-*` for any of them (the three repo rules govern Go CLI dispatcher, output, and error-wrapping surfaces, none of which this change touches). `packages/cli/package.json` and `.github/workflows/release.yml` map to no component at all.

## Decision

Rename the npm container and re-word every fact block that spells the package name or the asset origin, in one change-unit, without altering any behavior claim beyond the identity.

The container c3-3 is renamed to `npm @cuongtran001/c3x-cli` (frontmatter `title` plus its body H1, which `applyFrontmatter` deliberately leaves frozen) and its Responsibilities block names the new package and the fork's releases as the runtime source. The wrapper component c3-203 keeps its full/portable/source/npm fallback ladder verbatim and only re-spells the package spec in its Purpose, its ref-fat-thin-distribution governance row, its runtime-exec contract row, and its `skills/c3/bin/VERSION` derived-material row. The governing ref-fat-thin-distribution has its Choice block re-worded: the thin-client half of the split is now this fork's package pulling this fork's releases. The downloader component c3-301 has its Purpose extended to record that `version.ts` now carries the package name and the release-repository slug that `manager.ts` derives its download and release-index URLs from — the release origin is now a `version.ts` constant, and no fact said so.

The split of responsibility stays exactly as ref-fat-thin-distribution and ref-cross-compiled-binary describe it: the binary remains the single source of behavior, the npm package remains a runtime manager that downloads verified release assets on demand, and the platform matrix is untouched. Only the two names change.

The c3-0 Containers row for c3-3 also carries the old title, but membership rows are synthesized from the child's `parent:` link on every parentage path, so it is not patched here — `change apply` re-synthesizes it. Parent Delta: none (c3-0's Responsibilities framing and c3-3's Goal Contribution are unchanged; only the synthesized Name cell moves).

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| c3-3 | container | Its title and its Responsibilities block both name `@c3x/cli`, the package this fork cannot publish; the container now ships `@cuongtran001/c3x-cli` from this fork's releases. | c3-3#n939@v1:sha256:7c7fbfbe2110460588018af64ecb2592a3ff371d6530704c40ec6a02435c5e4e | Rename via frontmatter plus H1 block; re-word Responsibilities only. No behavior claim changes. |
| c3-203 | component | The wrapper's Purpose, fat-thin governance row, runtime-exec contract row, and VERSION derived-material row all pinned `@c3x/cli@${VERSION}`, but `c3x.sh` now execs `@cuongtran001/c3x-cli@${VERSION}`. | c3-203#n915@v1:sha256:99236b139c46285f217a68c734899f46e7949af109e2a1ef729436a85e106b41 "Carry bin/c3x.sh and bin/VERSION" | Re-spell the package spec in four blocks; the bundled/portable/source/npm ladder and platform gate stay verbatim under ref-cross-compiled-binary. |
| c3-301 | component | Its code binding is `packages/cli/src/**`, where `version.ts` gained `NPM_PACKAGE` and `RELEASE_REPO_SLUG` and `manager.ts` now derives the release-download and release-index URLs from that slug; the Purpose described `version.ts` as holding only the version pin and model revision. | c3-301#n953@v1:sha256:7c168cb2cb14e59fb60c160674153be39d2b05484c13d5b103464fb26126bd22 "Carry packages/cli/src" | Extend the Purpose to name the identity constants and the derived release origin; the manager's contract rows are unchanged. |
| ref-fat-thin-distribution | N.A - ref, not a topology node | Its Choice block named "the npm `@c3x/cli` client" as the runtime-manager half of the fat/thin split, which is the upstream package. | ref-fat-thin-distribution#n1054@v1:sha256:967bff5c47dcee5989eb98d4af5badd552740f8a834aa5d121d4f957a3ce8a18 "The release ships per-platform full-fat skill ZIPs" | update-ref: re-word Choice to the fork's package and release origin; Goal, Why, and How keep their generic wording and are not patched. |
| c3-2 | N.A - container unchanged | The skill container owns the wrapper whose npm fallback is re-pointed, but its Responsibilities describe that fallback generically as the pinned npm runtime manager and name no package, so nothing in c3-2 is stale. | c3-2#n843@v1:sha256:2d82a852ad3334c4cdb9a8e1da0589896efd762ebcb6d3089500157464c525b8 "Teach an agent to operate C3 through shared skill instructions" | No container patch. Parent Delta: none — c3-203's Goal Contribution row is unchanged by this unit. |
| c3-0 | N.A - system unchanged | No system-level fact is patched: c3-0's Goal, Abstract Constraints, and the framing of its Containers table all describe the npm client generically and stay true. Its Containers row for c3-3 carries the new title because the rename re-synthesized membership. | c3-0#n1150@v2:sha256:7f320c4aebdc65df28c1c4e4db9075d5369f37d7a687ce8260e92a6e44491f25 | No patch — membership rows are tool-synthesized. Parent Delta: none. |

## Compliance Refs

| Ref | Why required | Evidence | Action |
| --- | --- | --- | --- |
| ref-fat-thin-distribution | It owns the fat/thin split this change re-points: it is the ref that names which npm client is the runtime manager and where the verified release assets come from, and both c3-203 and c3-301 use it. | ref-fat-thin-distribution#n1054@v1:sha256:967bff5c47dcee5989eb98d4af5badd552740f8a834aa5d121d4f957a3ce8a18 "The release ships per-platform full-fat skill ZIPs" | update-ref — the Choice block is patched in this unit so the thin-client half names `@cuongtran001/c3x-cli` and this fork's releases. |
| ref-cross-compiled-binary | It fixes the per-platform asset names and the linux amd64/arm64 + darwin arm64 matrix that both the wrapper and the downloader resolve against; the fork change moves *where* those assets are fetched from without touching *what* they are called, so the ref must be shown to still hold. | ref-cross-compiled-binary#n1036@v1:sha256:f64903319c307116764ef288040ad92dea37db2c40430ab4974839b7b1ed12dc "Build the standard Go CLI release binary" | comply — no patch. Asset names and the platform matrix are unchanged; `assetNames` and the wrapper's `c3x-${VERSION}-${OS}-${ARCH}` computation are untouched by this branch. |

## Alternatives Considered

| Alternative | Rejected because |
| --- | --- |
| Repoint the npm trusted publisher for `@c3x/cli` at this fork and keep the upstream package name | Not available to this repository. `@c3x/cli` belongs to the upstream maintainer, and npm allows exactly one trusted publisher per package, so the fork cannot bind its own workflow to that name without upstream transferring or co-owning the package. Keeping the name would leave `release.yml` failing at `npm publish` with the same 404 on every push to `main`. |
| Skip npm publishing on the fork entirely and ship only the GitHub Release skill ZIPs | This is the state that produced the bug. `skills/c3/bin/c3x.sh` falls back to `npm exec --yes --package "<pkg>@${VERSION}"` whenever no bundled binary and no Go toolchain are present, so a no-binary skill install has no runtime at all unless a package exists at the pinned VERSION — which is exactly why `c3x` has no working runtime in this checkout today. Dropping npm would make the fat/portable ZIPs the only installable form and silently break the no-binary artifact that ref-fat-thin-distribution requires. |
| Leave the facts frozen as-is and treat the rename as an untracked implementation detail | The package name and the release origin are load-bearing distribution claims, not incidentals: c3-203's contract row is the documented evidence for the wrapper's exec line, and c3-301's binding covers the manager that resolves the release URLs. Letting them keep the upstream name would make `c3x eval` and any future reader disagree with the shipped code, which is the drift the freeze exists to prevent. |

## Verification

| Check | Result |
| --- | --- |
| `c3x change view adr-20260811-fork-npm-client-identity` and `c3x change status adr-20260811-fork-npm-client-identity` | Every patch reports `pending` with no drift before a human accepts. |
| `c3x change apply adr-20260811-fork-npm-client-identity` | All nine patches land in one transaction; the drift, canvas, morph, and retire gates pass. |
| `c3x check` | Stays `ok: true` at total 41 — no fact is created or retired by this unit. |
| `c3x read c3-3` and `c3x read c3-0 --section Containers` | c3-3's title, H1, and Responsibilities name `@cuongtran001/c3x-cli`; the synthesized c3-0 Containers row shows the new name without having been hand-edited. |
| `grep -rn "@c3x/cli" .c3/c3-3-/ .c3/c3-2-/ .c3/refs/ .c3/eval/` | No match — the live facts and the eval claim carry only the fork package name. |
| `c3x eval c3-3` and `c3x eval c3-301` | Both pass against the edited `.c3/eval/c3-3.yaml` claim and the unchanged `c3-301` version-pin pipeline. |
| `npm view @cuongtran001/c3x-cli version` after the next release run | Returns the version in `skills/c3/bin/VERSION`, proving the wrapper's no-binary fallback now resolves. |
