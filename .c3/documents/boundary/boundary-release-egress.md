---
id: boundary-release-egress
c3-seal: 5a85804acad4f42da40124242ff41581981f29f984d911a3e8f177cbde425087
title: release egress
type: boundary
parent: boundary-developer-workstation
goal: 'The only network traffic in the product is fetching released artifacts: the npm thin client and the wrapper download GitHub Release binaries, and the asset builder fetches the embedding model. Enclosing those paths keeps every download checksum-verified and version-pinned.'
---

## Goal

The only network traffic in the product is fetching released artifacts: the npm thin client and the wrapper download GitHub Release binaries, and the asset builder fetches the embedding model. Enclosing those paths keeps every download checksum-verified and version-pinned.

## Perimeter

| Kind | Mediation | Evidence |
| --- | --- | --- |
| network | HTTPS to GitHub Releases and the npm registry only; assets verified against published checksums; versions pinned by VERSION | this fact |

## Members

| Member | Role | Notes |
| --- | --- | --- |
| c3-3 | Resolves and downloads verified runtimes |  |
| c3-203 | npm exec fallback when no local binary exists |  |
| c3-402 | Fetches and packages the ONNX embedding-model assets |  |

## Crossings

| Path | Control | Evidence |
| --- | --- | --- |
| Binary download | SHA-256 verified against the release checksum file before use | packages/cli/src/** |
| Model asset fetch | Pinned model version, built offline into the fat variant | cli/tools embedding-asset builder |
