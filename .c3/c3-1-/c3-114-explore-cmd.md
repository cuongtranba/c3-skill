---
id: c3-114
c3-seal: e8c62898596526b0b44a4a332a7e7fa1e7ced95e5f89023a2c9b359031a142bd
title: explore-cmd
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

Serve a faithful, interactive mirror of the architecture: `explore` walks every store entity into a node, every membership and `uses` relationship (and, with `--include-adr`, change-unit `affects`) into an edge, and stamps each node with an explicit lifecycle — frozen fact, ADR state, or change-unit staged — then assembles a single HTML file with the 3D engine and the live payload inlined so it opens offline. Non-goals: mutating any fact, validating canvas shape (read-cmds `check`), or checking fact-to-code conformance (`eval`).

## Governance

| Reference | Type | Governs | Precedence | Notes |
| --- | --- | --- | --- | --- |
| rule-wrap-error-cause | rule | Every store, filesystem, or serialization failure crossing the explore boundary wraps its cause with the stage and the entity or path that failed | Keeps the export diagnosable to its root cause | RunExplore/buildExplorePayload wrap errors with fmt.Errorf(... : %w, err). |
| rule-output-via-helpers | rule | Command output (the written-file summary) goes through the shared writer, not ad-hoc printing | Output stays consistent with the rest of the CLI surface | Writes the summary line to the injected io.Writer. |

## Contract

| Surface | Direction | Contract | Boundary | Evidence |
| --- | --- | --- | --- | --- |
| explore | IN | Reads all entities plus their membership/uses relationships and the non-terminal change-unit patch targets; never writes to the store or the .c3/ tree | Read-only; a hidden ADR is excluded unless --include-adr is passed | cli/cmd/explore.go buildExplorePayload |
| HTML payload | OUT | Emits a self-contained HTML file whose embedded window.C3_DATA node set equals the store entity set and whose edge set equals the store's membership/uses (plus affects) edges, each node carrying an explicit lifecycle | Single file, no external runtime: three.js, OrbitControls, JS, CSS, and data are all inlined | cli/cmd/explore.go renderExplorerHTML; cli/cmd/explore_test.go |

## Derived Materials

| Material | Must derive from | Allowed variance | Evidence |
| --- | --- | --- | --- |
| cli/cmd/explore.go | Contract | Ring/level mapping and lifecycle-to-visual encoding may vary as long as node/edge coverage mirrors the store and every node keeps an explicit status | go test ./cmd -run Explore |
| cli/cmd/assets/explorer/* | Purpose | The renderer's visual design and vendored engine version may vary while the output stays a single self-contained file with no network dependency | go test ./cmd -run TestRunExplore_EmitsSelfContainedHTML |
