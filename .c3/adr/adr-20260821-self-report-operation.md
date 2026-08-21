---
id: adr-20260821-self-report-operation
c3-seal: 6f9efd5a145aa42a4d18c2440d1b9e484e3ad05d218931a0a3c93e22f29c06d9
title: self-report-operation
type: adr
goal: 'Give C3 a producer side for its own defect queue: add a `report` operation that turns friction an agent hits mid-task — a c3x crash, a reference the CLI contradicts, a missing operation — into a filed-ready, de-duplicated GitHub issue on `cuongtranba/c3-skill`. The CLI composes the issue and stops there; the agent files it with `gh` behind a per-project consent gate.'
status: done
date: "2026-08-21"
---

## Goal

Give C3 a producer side for its own defect queue: add a `report` operation that turns friction an agent hits mid-task — a c3x crash, a reference the CLI contradicts, a missing operation — into a filed-ready, de-duplicated GitHub issue on `cuongtranba/c3-skill`. The CLI composes the issue and stops there; the agent files it with `gh` behind a per-project consent gate.

## Context

C3's maintenance loop already drains a GitHub issue queue: it takes the lowest unassigned open issue, fixes it test-first, gates it, merges, and verifies the release. Nothing fills that queue except a human writing an issue by hand.

The signal is discarded exactly where it is sharpest. The agent operating C3 is the only witness to a defect — it sees the panic, the gate refusing valid input, the `help[]` hint that dead-ends — and that observation is gone by the next turn. Every issue the loop has closed so far was this shape, reconstructed by hand long after the fact.

Two constraints shape the answer. C3 runs inside other people's projects, so a report can carry a home directory or a private project name onto a public tracker unless paths are stripped first. And a defect worth reporting is often a defect that broke the store, so the operation cannot depend on the store opening.

Affected: the Go CLI container (c3-1) gains a command component; runtime-support (c3-107) already writes the activity trail the report reads but does not declare it; operation-references (c3-202) enumerates the reference set and is already stale by one op.

## Decision

Split composition from transmission. `c3x report <fault|guidance|gap>` builds the envelope — title, labels, body, fingerprint — and makes no network call; the returned `help[]` line hands the agent the `gh` commands that dedupe and file. Keeping the network out of Go means no credential ever lives in c3x, the whole command is testable by string comparison, and the outward-facing act stays where a human can veto it.

Three consequences follow from that split:

Reportable kinds are a closed set of three — `fault`, `guidance`, `gap`. User-input errors are deliberately excluded: a schema violation is c3x working, and filing those would bury real defects.

The fingerprint excludes the c3x version and normalizes away indices, quoted values, and paths, so one defect resurfacing after an upgrade lands on the issue that already exists rather than opening a fresh one every release. It hashes the raw text but the body carries only redacted text — filenames survive because they locate the defect, directories are elided because they identify the user.

Consent lives at `.c3/report.json`, following the `.c3/runtime.json` precedent for operational metadata that is not a frozen fact: root-level JSON, unknown fields rejected so the file cannot become a credential store, invisible to the seal because both `.c3/` walkers gate on `.md`.

The command dispatches before the store opens and tolerates a missing `.c3/`, because an unreadable project is precisely when a report is worth the most.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| c3-115 | component | New command component owning report composition, redaction, fingerprinting, and the consent policy | c3-115#n1243@v1:sha256:6a78ce6b6344b815b54227a585057ba7950cad6d98b9e724b886615a61102730 "Turn friction with C3 into a filed-ready, de-duplicated GitHub issue about c3x itself, without the CLI ever touching the network." | Must cite rule-output-via-helpers, rule-dispatcher-error-hint, rule-wrap-error-cause |
| c3-107 | component | Owns the activity trail the report reads for a fault's cause; the trail entry gains an error field and the file is now declared in its code binding | c3-107#n780@v1:sha256:717765e3f4f243e85254ba5a5ab59fc963bfb46b6e7428df4e06998925826924 "Parse argv into options and dispatch to a command handler (`main.go`), resolve the project's `.c3/` directory by walking up from the working dir (`config`), ser" | Purpose must name the trail it owns |
| c3-1 | container | Its behavior list gains self-reporting, a category of work the CLI did not previously do | c3-1#n1293@v2:sha256:aa8371bae74a94e198f835412e0d522a915ec875303345c431996030f21361a3 "Own the entire behavior of C3: parse and render `.c3/` documents, persist the entity-relationship graph, validate canvas conformance, run the change-unit saga t" | Parent Delta: updated |
| c3-202 | component | The reference set gains report.md; its enumerated op list was already stale by one op (eval) | c3-202#n1032@v1:sha256:a15f95667a5511fe424b0afb8386dd979d8b3d011b04e9e89299d74ec0e9de70 "Carry references/*.md — onboard, query, audit, change, ref, rule, canvas, sweep, eval, and report — each the playbook for one classified op: onboard walks the a" | Count and enumeration must match the shipped set |

| c3-0 | system | The system's distribution story gains a self-improvement loop: friction reported from any project reaches this repo's issue queue | c3-0#n3@v1:sha256:cee3eb278e1317505a0e044598e7ab83b4bfd3a67024817020ae07564393a2ff "Build and distribute C3 — a knowledge-graph architecture-docs tool that holds a codebase's architecture as frozen, verifiable facts — shipped three ways: a Go C" | No section change; named for top-down closure |
| c3-2 | container | The skill gains a tenth operation reference and a router row for it | c3-2#n985@v1:sha256:2d82a852ad3334c4cdb9a8e1da0589896efd762ebcb6d3089500157464c525b8 "Teach an agent to operate C3 through shared skill instructions, Claude plugin packaging, and a wrapper that runs the selected C3 runtime." | Parent Delta: none — the container's framing already covers carrying references |

## Compliance Refs

| Ref | Why required | Evidence | Action |
| --- | --- | --- | --- |
| ref-frontmatter-docs | report.md joins references/ and c3-115 joins the frozen fact set; both must carry the standard frontmatter-plus-canvas-sections shape this ref defines | ref-frontmatter-docs#n1203@v1:sha256:d4f7719668519e2f2a93de15969bc53c8f0105e7e073231a2f36d7c2626cb361 "Standardize every `.c3/` document as YAML frontmatter plus canvas-shaped markdown sections." | comply |
| ref-eval-determinism | This ADR adds .c3/eval/c3-115.yaml and extends c3-107's binding, so both must yield the same verdict for an unchanged subject | ref-eval-determinism#n1185@v1:sha256:d914f393b17de0202b7ae4cdde4df7d173c51fd820b2695487a29efb06f514d7 "A conformance verdict must mean the same thing every time it is computed for an unchanged subject. Without a reproducibility rule an eval that \"holds\" today cou" | comply |

| ref-cross-compiled-binary | Inherited by naming c3-2 and c3-0; report ships inside the existing binary and adds no platform, asset, or build variant. | ref-cross-compiled-binary#n1176@v1:sha256:cea27fa9abdd975d6298f23e899ceb48f2945fde1e933915fdfada258a190136 | N.A - no change to the binary matrix. |
| ref-fat-thin-distribution | Inherited by naming c3-2 and c3-0; report.md rides the skill archive that already carries references/, and the thin client is untouched. | ref-fat-thin-distribution#n1194@v1:sha256:cf8e08bcc48d161ef4120914a8505499068ac512709adeae244258fd1618b031 | N.A - no change to the fat/thin split. |

## Compliance Rules

| Rule | Why required | Evidence | Action |
| --- | --- | --- | --- |
| rule-dispatcher-error-hint | report's refusals — unknown kind, missing summary or subject, consent off, corrupt policy — are user-facing dispatcher errors and must each name the way forward | rule-dispatcher-error-hint#n1212@v1:sha256:bd662000c1bc5b93d0b1cc4cf532cc1dc6e4766e5bda6b544f8aab14d21f7dc4 "Make every user-facing CLI failure recoverable: a wrong invocation should tell the user what to do next, not just that something was wrong." | comply |
| rule-output-via-helpers | The envelope and the consent policy are structured results and must serialize through the shared helpers rather than ad-hoc printing | rule-output-via-helpers#n1225@v1:sha256:b5ac8121ffc54be6c8f87ec133e69658fea023e7e73da3859fb85a33869afa29 "Keep machine output uniform: one place decides TOON vs JSON and honors agent mode, so every command speaks the same serialization." | comply |
| rule-wrap-error-cause | Consent policy encode, read, and write failures cross a layer boundary and must carry the operation, the path, and the underlying cause | rule-wrap-error-cause#n1237@v1:sha256:b9e4edb84b11060973de3fe6e5c0ab7b5605aa690e00e886335b054bdaab710f "Preserve the failure chain: an error that crosses a function boundary should say what this layer was doing and still carry the underlying cause." | comply |

## Verification

| Check | Result |
| --- | --- |
| cd cli && go test ./... | Passes, including TestUserFacingErrorHints (AST hint gate) and the TestReport suite |
| c3x eval c3-115 | holds — report.go contains no network or exec call |
| c3x eval c3-107 | holds — activity.go now resolves under its code binding |
| c3x check | ok, no seal drift after apply |
| python3 scripts/test_skill_cli_contract.py | Passes with report added to the bare-c3 alternation |
| c3x report gap --subject diff --summary "no fact diff" | Emits an envelope with a fingerprint and makes no network call |
