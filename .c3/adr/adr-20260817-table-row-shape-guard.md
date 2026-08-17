---
id: adr-20260817-table-row-shape-guard
c3-seal: 4304fe66d54d9695b7c8e298fafa8551675a6b32b64a49de1a8f1089488f7051
title: table-row-shape-guard
type: adr
goal: Make a table row that carries more cells than its table can hold a hard write failure, instead of letting the markdown parser silently cut the overflow. A row's column count must survive the parse-render round trip that every fact write performs, and the write must abort when it would not.
status: done
date: "2026-08-17"
---

# Refuse a write that would truncate a table row

## Goal

Make a table row that carries more cells than its table can hold a hard write failure, instead of letting the markdown parser silently cut the overflow. A row's column count must survive the parse-render round trip that every fact write performs, and the write must abort when it would not.

## Context

A table cell may hold a literal pipe, written escaped. A union type spelled out in prose does exactly that. Two independent defects combined to delete committed documentation.

Stage one broke the escape: the changeset layer normalized a patched table row by splitting it on raw pipes and rejoining the pieces with a padded delimiter, which rewrote an escaped pipe as a backslash, a space, and a bare pipe. That escapes a space rather than the pipe. In a diff it reads as whitespace churn, so it was reviewed and committed.

Stage two spent the damage. With the pipe now bare, markdown counts it as a column separator, so the row claims more columns than its header declares, and the parser truncates the row at the header's width. The truncation fires on the next write that loads the document, which is typically an apply targeting entirely different facts. One document lost 533 bytes of a row no patch had ever touched.

Nothing connected the two stages: they landed months apart, neither raised an error, and the local cache is gitignored, so the markdown is the only surviving copy. A contributor who does not diff after an apply commits the loss.

## Decision

Split the fix at both stages, and remove the duplication that allowed them to diverge.

One canonical splitter, `markdown.SplitRowCells`, owns the knowledge of where a row's column boundaries are. It splits on unescaped pipes and returns each cell's raw text with escapes intact. The changeset normalizer and the fidelity guard both call it; `markdown.parseCells` layers unescaping on top to yield cell values. Three hand-rolled splitters becoming one is the point: the bug existed because the changeset layer did not use the escape-aware splitter that already existed beside it.

`content.VerifyTableFidelity` closes stage two independently of any cause. `WriteEntity` runs it before parsing and refuses the write when a data row carries more cells than its table's header, naming the document, the offending row, and how many cells would be lost. Refusing before the parse matters: afterwards the overflow cells are already gone and the loss is indistinguishable from a row the author meant to shorten.

The guard is deliberately a refusal rather than a repair. C3's writes are the only sanctioned mutation of a fact, so a write that cannot preserve its input must stop rather than guess which reading the author intended.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| c3-101 | component | Owns the document-to-node bridge; `WriteEntity` gains a precondition that refuses a body whose row shape cannot survive the round trip, and the canonical row splitter lives in its markdown package | c3-101#n523@v1:sha256:5cd469994c2fc4b5b8891972949aee646a155d15dc9a2d21738598fb6c42512b | Confirm the contract states the refusal, and that cell escaping is no longer free to vary |
| c3-104 | component | Its table-row normalizer caused stage one; it now delegates to the shared splitter instead of splitting on raw pipes | c3-104#n597@v1:sha256:c468dad9e624cc4305b6310af7374c0a43a31b8e80a3285c32ae1179b6f9fa7b | Confirm no local row-splitting logic remains in the changeset layer |
| c3-1 | N.A - container unchanged | Parent of both touched components, and every command it exposes inherits the new write precondition, but the container's own facts describe writing a fact generically and name no row-shape behavior, so nothing in c3-1 goes stale. | c3-1#n482@v1:sha256:f7c8f25904e4c5ef4c42311b9e67d9fec19c9243532ecd2cb99d28c00b212ec3 | No container patch. Parent Delta: none — the components' Goal Contribution rows are unchanged by this unit. |
| c3-0 | N.A - system unchanged | The markdown files are the system's only durable copy of a fact, so this is system-level data loss, but no system-level fact is patched: c3-0's Goal and Abstract Constraints describe durability generically and stay true. | c3-0#n3@v1:sha256:cee3eb278e1317505a0e044598e7ab83b4bfd3a67024817020ae07564393a2ff | No patch. Parent Delta: none. |

## Compliance Refs

| Ref | Why required | Evidence | Action |
| --- | --- | --- | --- |
| ref-frontmatter-docs | It is the parsing contract for the markdown shape this ADR guards; the refusal enforces that a fact's table rows round-trip under that contract | ref-frontmatter-docs#n1061@v1:sha256:d4f7719668519e2f2a93de15969bc53c8f0105e7e073231a2f36d7c2626cb361 | comply |

## Compliance Rules

| Rule | Why required | Evidence | Action |
| --- | --- | --- | --- |
| rule-wrap-error-cause | The guard introduces a new write failure; WriteEntity wraps it with the entity id so the caller sees which fact refused and why | rule-wrap-error-cause#n1095@v1:sha256:b9e4edb84b11060973de3fe6e5c0ab7b5605aa690e00e886335b054bdaab710f | comply |

## Enforcement Surfaces

| Surface | Behavior | Evidence |
| --- | --- | --- |
| content.VerifyTableFidelity | Refuses a body whose data row carries more cells than its header, reporting the row and the count that would be lost | cli/internal/content/fidelity_test.go |
| WriteEntity | Runs the guard before parsing, so no caller can commit a truncating body through any command | TestWriteEntity_RefusesRowShapeLoss |
| markdown.SplitRowCells | Single splitter that keeps an escaped pipe inside its cell, including a row whose final cell ends in one | cli/internal/markdown/splitrow_test.go |

## Alternatives Considered

| Alternative | Rejected because |
| --- | --- |
| Repair the broken escape on load and continue | It mutates documents the write never targeted, which is the exact property that made the original loss so hard to attribute. A silent rewrite is what caused this. |
| Warn and let the write proceed | The failure is already silent and misattributed; a warning on a command whose real output is a diff would be missed the same way stage one was. |
| Fix only the changeset splitter | It stops new corruption but leaves every document already carrying a broken escape primed to truncate on its next write. |
| Assert a full source-to-render round trip | Far broader than the failure, and it would reject legitimate normalizations the renderer performs by design. |

## Risks

| Risk | Mitigation | Verification |
| --- | --- | --- |
| A document already carrying a broken escape now fails its next write instead of truncating | Intended: the error names the row and the fix. The repository was swept and the single offender corrected before shipping. | Scan of every markdown file in the repository reports zero offending rows |
| The guard misreads a pipe inside a fenced code block as a column | Fence tracking skips fenced regions when scanning | TestVerifyTableFidelity_IgnoresFencedCode |
| A short row is wrongly rejected | Only rows wider than the header are refused; markdown pads short rows, losing nothing | TestVerifyTableFidelity_AcceptsShortRow |

## Verification

| Check | Result |
| --- | --- |
| go test ./internal/markdown/ ./internal/content/ ./internal/changeset/ | pass |
| go test ./cmd/... ./internal/... | pass |
| c3x check | 42, ok |
| c3x eval | 26 holds, 0 drift |
| Every markdown file in the repository scanned through VerifyTableFidelity | 0 offenders after correcting research/eval/skill-eval/cases/acountee-properties.md |
