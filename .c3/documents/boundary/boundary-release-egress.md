---
id: boundary-release-egress
c3-seal: 746effc43d70a658c97610e94b681659d4902f5fefcc4c3e3318cbeb8118c3cf
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
| network | HTTPS to GitHub Releases and the npm registry only; assets verified against published checksums; versions pinned by VERSION | packages/cli/src/**; cli/tools/semantic-assets/**; skills/c3/bin/VERSION |

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
