---
id: ref-fat-thin-distribution
c3-seal: 4d598f420be20d8bbde261060d4356a3af0719c0c9ebd132bd1852586894bae2
title: Fat Skill / Thin Client Distribution
type: ref
goal: Distribute C3 as full-fat skill ZIPs (semantic binaries bundled), Linux portable fat skill ZIPs (pure-Go core binaries bundled), a no-binary skill/plugin artifact (runtime delegated), and a thin npm client (binary downloaded on demand).
---

# Fat Skill / Thin Client Distribution

## Goal

Distribute C3 as full-fat skill ZIPs (semantic binaries bundled), Linux portable fat skill ZIPs (pure-Go core binaries bundled), a no-binary skill/plugin artifact (runtime delegated), and a thin npm client (binary downloaded on demand).

## Choice

The release ships per-platform full-fat skill ZIPs with semantic-capable binaries for sandboxed/offline use, Linux portable fat skill ZIPs with pure-Go core binaries for broader distro/sandbox compatibility, a platform-neutral no-binary skill/plugin ZIP with Claude metadata for installer flows, and the npm `@cuongtran001/c3x-cli` client as the runtime manager that downloads verified `cuongtranba/c3-skill` release assets on demand.

## Why

Sandboxed and offline skill installs need a self-contained binary, but not every sandbox can run the native semantic runtime path. Full-fat artifacts preserve embedded semantic search for supported environments, portable Linux artifacts favor distro compatibility with keyword/graph fallback, plugin and skills CLI installers need a platform-neutral package that does not carry binaries, and npm users want a small package that pulls only the runtime assets they need.

## How

CI assembles skill archives from one tree that includes `.gitattributes`, `.claude-plugin/`, and `skills/`: the platform-neutral `c3-skill-v{VERSION}.zip` removes all `skills/c3/bin/c3x-*` binaries, each full-fat `c3-skill-{platform}-v{VERSION}.zip` adds exactly the matching semantic-capable fat binary, and each Linux portable `c3-skill-linux-{arch}-portable-v{VERSION}.zip` adds exactly the matching pure-Go portable binary. The npm manager honors C3X_VERSION when externally set for development/tests, otherwise resolves project or latest runtime versions and downloads verified runtime assets from the GitHub Release.
