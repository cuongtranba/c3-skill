---
id: adr-20260710-add-visual-layer-explore
c3-seal: e20846c876cd52ae8b4260eb581210e783c7f99d2f1e48cf517a60fb8ba17d76
title: add-visual-layer-explore
type: adr
goal: 'Give C3 a visual layer: a new `c3 explore` command that projects the live architecture model — systems, containers, components, their dependency edges, and each fact''s lifecycle status — into a single self-contained, interactive 3D C4 explorer, so the model can be seen and navigated, not only queried.'
status: proposed
date: "2026-07-10"
---

## Goal

Give C3 a visual layer: a new `c3 explore` command that projects the live architecture model — systems, containers, components, their dependency edges, and each fact's lifecycle status — into a single self-contained, interactive 3D C4 explorer, so the model can be seen and navigated, not only queried.

## Context

C3's model was reachable only through text/JSON/mermaid queries. Reviewers could lose sight of components, could not see dependency wiring at a glance, had no interactive way to explore the topology, and — critically — had no observable view of lifecycle status (frozen facts, ADR states, change-unit staging). The design reference (an imported "Architecture Explorer" mockup) proved the 3D radial C4 shape but was hardcoded and status-blind. The constraint: the visual layer must source data live from the store so it can never silently drift from or lose the model, and must ship as one offline file.

## Decision

Add component `explore-cmd` (c3-114) under the Go CLI container. It walks the store into a node/edge/status payload and assembles a single HTML file with the 3D engine (three.js + OrbitControls), renderer JS/CSS, and the live payload all inlined. Node coverage equals the store entity set, edge coverage equals the store's membership + `uses` (+ `affects` with `--include-adr`) relationships, and every node carries an explicit lifecycle: frozen, an ADR state, or `staged` with an observable `frozen → changing` transition. Chosen over a static snapshot (would drift) and a 2D renderer (loses the ratified 3D radial design); the whole-graph C2 level keeps components and their dependencies from ever being lost at the default view.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| c3-0 | system | Adds a read-only visual projection surface to the product while staying inside the existing Go CLI system shape. | c3-0#n3@v1:sha256:cee3eb278e1317505a0e044598e7ab83b4bfd3a67024817020ae07564393a2ff | Top-down context reviewed; no system body change needed. |
| c3-1 | container | The Go CLI gains a new component command; its Components membership grows by one. | c3-1#n1086@v2:sha256:f7c8f25904e4c5ef4c42311b9e67d9fec19c9243532ecd2cb99d28c00b212ec3 | Parent Delta: none — membership row auto-synthesized; container responsibilities unchanged (additive read-only command). |
| c3-114 | component | The new component that owns topology-to-visual serialization and single-file HTML assembly. | c3-114#n1059@v1:sha256:b1a461f193f88817bab487f504166b427805515d79177d31f239d38ade4b12e8 | New component authored with Goal, Parent Fit, Purpose, Governance, Contract, and Derived Materials; governed by rule-wrap-error-cause and rule-output-via-helpers. |

## Verification

| Check | Result |
| --- | --- |
| go test ./cmd -run Explore (node/edge coverage, lifecycle presence, --include-adr, self-contained output) | Pass — 4 tests green |
| In-browser window.C3_EXPLORER: rendered node ids == store node ids, rendered edges == payload edges | 41/41 nodes, 79/79 edges at C2 |
| In-browser nodesWithoutStatus() empty; staged facts carry frozen→changing transition | 0 without status; 24 staged with transitions |
| In-browser interactions: selectNodeById + setLevel + currentSelection | Pass — selection matches |
| c3 check with c3-114 present | ok: true, 42 entities |
