---
id: ref-cross-compiled-binary
c3-seal: 2662b1c0e323a1e900944285d079796139f34257a61d4b63ab9a3d1764df7c8a
title: Cross-Compiled Binary Distribution
type: ref
goal: Ship c3x as named per-platform release binaries, with standard runtime-manager binaries, full-fat semantic skill binaries, and Linux portable binaries where distro/sandbox compatibility matters more than local ONNX semantic search.
---

# Cross-Compiled Binary Distribution

## Goal

Ship c3x as named per-platform release binaries, with standard runtime-manager binaries, full-fat semantic skill binaries, and Linux portable binaries where distro/sandbox compatibility matters more than local ONNX semantic search.

## Choice

Build the standard Go CLI release binary for linux/amd64, linux/arm64, and darwin/arm64; build full-fat embedmodel skill binaries for that same matrix; and build additional pure-Go Linux portable binaries for linux/amd64 and linux/arm64 named `c3x-{VERSION}-linux-{arch}-portable`.

## Why

Standard and full-fat builds preserve the existing feature-complete runtime path, including embedded semantic assets for self-contained skill installs. Pure-Go Linux portable builds give musl, Alpine, distroless-like, and tightly sandboxed environments a bundled core runtime without forcing a heavier native ONNX/musl build.

## How

Merging the release PR makes release-please create the `v{VERSION}` tag and the GitHub Release, which chains the release workflow's release build variant: thin plus full-fat binaries for each platform in the matrix, and a `CGO_ENABLED=0` portable binary for each Linux arch. Release assembly publishes the standard binaries for the npm manager and packages full-fat and Linux portable binaries into their matching skill ZIPs, uploading them to the release that already exists.
