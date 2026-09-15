---
id: adr-20260915-c3v-visualize-command
c3-seal: 831f8c9f8f3d0b72ead64e9453eb4d4e55ffb29559a76c9d1993c2bfb8e7e2b8
title: c3v-visualize-command
type: adr
goal: 'Evolve the `explore` visual layer into `visualize`: the same read-only projection of the store, now rendered as an infrastructure city whose geometry (districts, streets, docks, routes) is computed in the CLI so the HTML renderer and the `--export scene.json` share one layout, and whose payload v2 carries every canvas-owned relationship, boundaries, flows, code bindings and eval verdicts.'
status: done
date: "2026-09-15"
---

## Goal

Evolve the `explore` visual layer into `visualize`: the same read-only projection of the store, now rendered as an infrastructure city whose geometry (districts, streets, docks, routes) is computed in the CLI so the HTML renderer and the `--export scene.json` share one layout, and whose payload v2 carries every canvas-owned relationship, boundaries, flows, code bindings and eval verdicts.

## Context

`explore` projected the store as a radial type-ring graph that could only draw membership and component→ref/rule `uses` edges; it was undocumented in the skill and its payload hardcoded the entity types it accepted. The model now carries `depends_on`, `encloses` and `flow_from`/`flow_to` edges (adr-20260915-c3v-zones-flows-dependencies), so the projection must render them, and the approved design (docs/specs/2026-09-15-c3v-visualize-payload-v2.md, explorer-app/mockup/c3v-city.html) places that rendering in a city metaphor with the layout owned by Go.

## Decision

Rename the component to `visualize-cmd` and let `visualize` (alias `explore`) emit payload v2: nodes gain archetype, district, layout, docks, code, tech, eval and statusKey; edges are generic over every canvas-owned relationship type; districts, roads and dock-to-dock routes are derived by `explore_layout.go`; `explore_export.go` writes the same geometry as ObjectLoader JSON; the validator derives its allowed node types and edge kinds from the canvases instead of a hardcoded set. The renderer in explorer-app consumes positions and never lays out on its own.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| c3-114 | component | Renamed, purpose and contract rewritten for payload v2, city layout and export | c3-114#n1011@v1:sha256:0780167ebbc429d525efbee6b3fbb5234821fb6055d2b6f68acbbe7c0ae87a8e | Governance rows unchanged: still routes errors with %w and output through the shared writer |
| c3-1 | container | Its explore command becomes visualize; membership row title changes by construction | c3-1#n1514@v2:sha256:f7c8f25904e4c5ef4c42311b9e67d9fec19c9243532ecd2cb99d28c00b212ec3 | Parent Delta: none — additive command rename, responsibilities unchanged |

## Verification

| Check | Result |
| --- | --- |
| `cd cli && go test ./...` | pass — explore/visualize/layout/schema/export suites green |
| `npm test --prefix explorer-app && npm run build --prefix explorer-app` | pass — 38 tests, single-file bundle |
| `c3x visualize --file out.html --export scene.json` on this repo | 38 nodes, 125 edges, 71 routes; renderedNodeIds == store ids |
| `c3x check` | ok: true |
