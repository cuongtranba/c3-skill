---
id: c3-1
c3-seal: bca38af8a90a20e9ebaf66f852b35a213ebd408112345a824b2dcb42f918d0b2
title: Go CLI
type: container
parent: c3-0
goal: Provide every c3x operation as a single cross-compiled Go binary — the engine that reads, writes, validates, and freezes the architecture graph.
---

# Go CLI

## Goal

Provide every c3x operation as a single cross-compiled Go binary — the engine that reads, writes, validates, and freezes the architecture graph.

## Components

| ID | Name | Category | Status | Goal Contribution |
| --- | --- | --- | --- | --- |
| c3-101 | doc-model |  | active | Turn a .c3/ markdown file into structured frontmatter and a node tree, and bridge those documents to and from the store's nodes. |
| c3-102 | store |  | active | Persist the architecture graph — entities, relationships, nodes, versions, code-map, and semantic vectors — behind one SQLite-backed API. |
| c3-103 | schema |  | active | Define the canvas types a fact may be — built-in and project-declared — and validate that a fact's body matches its canvas shape. |
| c3-104 | changeset |  | active | Run the change-unit saga: parse a folder of patch material, gate it against the frozen facts, and commit it atomically as the only legal mutation of a fact. |
| c3-105 | walker |  | active | Discover every fact file under the .c3/ directory tree and parse each one for check, list, and import. |
| c3-106 | codemap-lib |  | active | Parse the code-map that binds entities to source globs, match files against it, and validate that every binding resolves. |
| c3-107 | runtime-support |  | active | Bootstrap the CLI and host the shared runtime — resolve the .c3/ directory, dispatch the command, serialize output, and serialize concurrent mutations behind a coordinator. |
| c3-110 | read-cmds |  | active | Answer questions about the architecture graph without changing it: read a fact, list the topology, map a file to its owners, walk the graph, search the corpus, and validate structural integrity. |
| c3-111 | author-cmds |  | active | Create and edit facts and canvases before they freeze: scaffold a new project, add an entity from a body, replace content or fields, author canvas definitions, and manage each component's code-map. |
| c3-112 | change-cmds |  | active | Drive the change-unit lifecycle from the command line: scaffold and view a unit, run the apply gates, preview the result, inspect derivation obligations, supersede a decision, and enforce the freeze that makes a change-unit the only way to mutate a fact. |
| c3-113 | lifecycle-cmds |  | active | Keep the .c3/ store and its canonical markdown coherent across its lifecycle: import a tree into the database, export and sync it back out, repair drift, run the status migration, delete an entity safely, and install Git guardrails. |
| c3-108 | eval-engine | foundation | active | Run a fact's conformance pipeline — check a frozen claim against the uncontrolled external it governs — and produce a one-off, stamped verdict (holds / drift / needs-judgement) that is never an apply gate. |
| c3-109 | cmd-support | foundation | active | Provide the shared command-layer scaffolding for the c3x CLI — the authoritative command registry that drives help text, the global argument parser that turns argv into typed options, and the common output helper every command reuses. |
| c3-114 | explore-cmd | feature | active | Emit a self-contained, interactive visual layer for the C3 model: serialize the live topology, dependency edges, and lifecycle status into a single HTML architecture explorer sourced straight from the store. |
| c3-115 | report-cmd | feature | active | Turn friction with C3 into a filed-ready, de-duplicated GitHub issue about c3x itself, without the CLI ever touching the network. |

## Responsibilities

Own the entire behavior of C3: parse and render `.c3/` documents, persist the entity-relationship graph, validate canvas conformance, run the change-unit saga that is the only legal mutation path, map facts to the code they govern, and compose a filed-ready report of C3's own defects. The skill (c3-2) and npm client (c3-3) only invoke this binary; no architecture logic lives outside it.

## Complexity Assessment

High-cohesion layered design: foundation libraries (doc-model, store, schema, changeset, walker, codemap, runtime-support) under a thin command surface grouped by read / author / change / lifecycle intent.
