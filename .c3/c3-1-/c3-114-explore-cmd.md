---
id: c3-114
c3-seal: 385aa0bf9399c4ecfb2d8a51549653df5354ac5cbd10460c5277f493181c3aac
title: visualize-cmd
type: component
category: feature
parent: c3-1
goal: 'Emit a self-contained, interactive visual layer for the C3 model: serialize the live topology, dependency edges, and lifecycle status into a single HTML architecture explorer sourced straight from the store.'
uses:
    - rule-output-via-helpers
    - rule-wrap-error-cause
---

## Goal

Emit a self-contained, interactive visual layer for the C3 model: serialize the live topology, dependency edges, and lifecycle status into a single HTML architecture explorer sourced straight from the store.

## Parent Fit

| Field | Value |
| --- | --- |
| Parent | c3-1 |
| Role | The read-only visual projection of the CLI: the command that turns the queryable store into a shareable 3D C4 explorer. |
| Boundary | Owns topology-to-visual serialization and single-file HTML assembly; it leaves entity persistence to the store, canonical rendering to doc-model, and change-unit semantics to change-cmds. |
| Collaboration | Reads entities and relationships through the store; reads change-unit patch targets through changeset to derive staging; embeds vendored assets via Go embed. |

## Purpose

Serve a faithful, interactive mirror of the architecture as a city: `visualize` (alias `explore`) walks every store entity into a node with a derived archetype, district and footprint, every `contains`, canvas-owned relationship (`uses`, `depends_on`, `encloses`, `flow_from`, `flow_to`), `affects` and `flow_step` into an edge, and stamps each node with an explicit lifecycle, eval verdict and status key. `explore_layout.go` derives districts, streets, docks and dock-to-dock routes so the HTML renderer and `--export scene.json` share one geometry; the payload is validated fail-closed against the canvases before either is written. Non-goals: mutating any fact, laying out in the browser, validating canvas shape (read-cmds `check`), or checking fact-to-code conformance (`eval`).

## Governance

| Reference | Type | Governs | Precedence | Notes |
| --- | --- | --- | --- | --- |
| rule-wrap-error-cause | rule | Every store, filesystem, or serialization failure crossing the explore boundary wraps its cause with the stage and the entity or path that failed | Keeps the export diagnosable to its root cause | RunExplore/buildExplorePayload wrap errors with fmt.Errorf(... : %w, err). |
| rule-output-via-helpers | rule | Command output (the written-file summary) goes through the shared writer, not ad-hoc printing | Output stays consistent with the rest of the CLI surface | Writes the summary line to the injected io.Writer. |

## Contract

| Surface | Direction | Contract | Boundary | Evidence |
| --- | --- | --- | --- | --- |
| visualize | IN | Reads all entities, every relationship, eval verdicts, the eval `code:` bindings (files, loc, tech), the non-terminal change-unit patch targets and the canvas definitions; never writes to the store or the .c3/ tree | Read-only; a hidden ADR is excluded unless --include-adr is passed; `explore` is an alias | cli/cmd/explore.go buildExplorePayload; cli/cmd/explore_schema.go exploreAllowedFor |
| HTML payload and scene export | OUT | Emits payload v2 (schemaVersion 2) whose node set equals the store entity set and whose edge set equals the store's membership, canvas-owned, affects and flow-step edges, every node carrying lifecycle, statusKey, archetype, district, layout and docks, plus districts, roads and one route per routed edge; the same geometry is written as ObjectLoader JSON by --export | Single offline HTML (three.js, renderer, data inlined); scene.json objects carry userData.c3 with deterministic uuids | cli/cmd/explore.go renderExplorerHTML; cli/cmd/explore_layout.go layoutCity; cli/cmd/explore_export.go buildSceneJSON; cli/cmd/explore_test.go |

## Derived Materials

| Material | Must derive from | Allowed variance | Evidence |
| --- | --- | --- | --- |
| cli/cmd/explore.go | Contract | Ring/level mapping and lifecycle-to-visual encoding may vary as long as node/edge coverage mirrors the store and every node keeps an explicit status | go test ./cmd -run Explore |
| cli/cmd/assets/explorer/* | Purpose | The renderer's visual design and vendored engine version may vary while the output stays a single self-contained file with no network dependency | go test ./cmd -run TestRunExplore_EmitsSelfContainedHTML |

## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-102 | Reads every entity and its relationships to build the payload | store API: AllEntities, RelationshipsFrom, EvalMatch | cli/cmd/explore.go imports internal/store |
| c3-104 | Reads non-terminal change-unit patch folders to derive staged nodes | changeset.ReadPatchDir | cli/cmd/explore.go imports internal/changeset |
| c3-109 | Parses flags and writes the summary through the shared writer | cmd options + output helpers | cli/cmd/options.go --serve/--port/--schema/--file; RunExplore(opts, w) |
