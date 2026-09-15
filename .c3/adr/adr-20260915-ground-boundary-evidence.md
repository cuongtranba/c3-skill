---
id: adr-20260915-ground-boundary-evidence
c3-seal: c129400554d422d05af5146ec95d4fcd25317177ef4857f9989adb2387450d42
title: ground-boundary-evidence
type: adr
goal: Ground the Evidence cells of the three boundary facts in files instead of the placeholder "this fact", so `check` reports no ungrounded evidence for the perimeters the visualizer renders as zones.
status: done
date: "2026-09-15"
---

## Goal

Ground the Evidence cells of the three boundary facts in files instead of the placeholder "this fact", so `check` reports no ungrounded evidence for the perimeters the visualizer renders as zones.

## Context

adr-20260915-c3v-zones-flows-dependencies created the boundaries with Perimeter rows whose Evidence column read "this fact", and one Crossings row citing another boundary id where a file was expected. `check` flags all four as ungrounded evidence; the facts are otherwise valid.

## Decision

Replace each cell with the source that mediates the perimeter: the wrapper and server binding for the workstation, the freeze guard and apply gates for the write path, the npm client and asset builder for release egress; the cross-boundary Crossings row points at the wrapper's npm fallback.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| boundary-developer-workstation | boundary | Perimeter and Crossings Evidence cells re-grounded | boundary-developer-workstation#n1346@v1:sha256:1febaf8298f285f843337b9f5d57225cde5268c22f0dc58448a08be25ae7fbf7 | No member change |
| boundary-c3-tree-write | boundary | Perimeter Evidence cell re-grounded | boundary-c3-tree-write#n1363@v1:sha256:5d2a54b27e4022f8aef416b2db842af988329b8da7157918db0aed400898bdb4 | No member change |
| boundary-release-egress | boundary | Perimeter Evidence cell re-grounded | boundary-release-egress#n1382@v1:sha256:8eb4704162d0157978e3a454df7d0063f31555faa1feab8f1fe9d9a5f397c2a0 | No member change |

## Verification

| Check | Result |
| --- | --- |
| `c3x check` | ok: true with no "ungrounded evidence" warnings |
