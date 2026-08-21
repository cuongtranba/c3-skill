---
id: c3-115
c3-seal: 77db05c1a917292e2d492f7de2aa73134185c37051e496d9afb0e47a471e501a
title: report-cmd
type: component
category: feature
parent: c3-1
goal: Turn friction with C3 into a filed-ready, de-duplicated GitHub issue about c3x itself, without the CLI ever touching the network.
uses:
    - rule-dispatcher-error-hint
    - rule-output-via-helpers
    - rule-wrap-error-cause
---

## Goal

Turn friction with C3 into a filed-ready, de-duplicated GitHub issue about c3x itself, without the CLI ever touching the network.

## Parent Fit

| Field | Value |
| --- | --- |
| Parent | c3-1 |
| Role | The CLI's self-improvement path: the one command whose subject is c3x rather than the user's architecture. |
| Boundary | Owns report composition — kind classification, redaction, fingerprinting, and the consent policy at `.c3/report.json`; it neither authenticates to GitHub nor sends anything, and it reads no entity. |
| Collaboration | Reads the failure cause and command trail runtime-support appends to `.c3/activity.jsonl`; renders through the shared output helpers and hands the agent the `gh` commands that dedupe and file. |

## Purpose

Compose a GitHub issue for one of three defect kinds — `fault` (c3x misbehaved), `guidance` (a reference contradicts the CLI), `gap` (a needed operation does not exist) — grounding a fault in the last failed command recorded in the activity trail, stripping filesystem paths so a public tracker never receives a home directory or a private project name, and stamping a version-independent fingerprint so the same defect resurfacing after an upgrade lands on the issue that already exists. Persist the per-project consent policy that decides whether an agent may file without asking. Non-goals: calling GitHub or holding any credential (the agent files it), reporting user-input errors such as schema violations (that is c3x working), and reading or mutating any fact.

## Governance

| Reference | Type | Governs | Precedence | Notes |
| --- | --- | --- | --- | --- |
| rule-output-via-helpers | rule | The envelope and the consent policy serialize through the shared output helpers, never ad-hoc printing | Keeps report's machine output identical in shape to every other command | RunReport and runReportConsent both end in WriteObjectOutput. |
| rule-dispatcher-error-hint | rule | Every refusal — unknown kind, missing summary, missing subject, consent off, corrupt policy — names the way forward on a `hint:` line | A report command that fails opaquely defeats its own purpose | go test ./cmd -run TestUserFacingErrorHints. |
| rule-wrap-error-cause | rule | Policy encode and write failures wrap their cause with the operation and the path that failed | Keeps a failed consent write diagnosable | fmt.Errorf("write %s: %w", path, err). |

## Contract

| Surface | Direction | Contract | Boundary | Evidence |
| --- | --- | --- | --- | --- |
| report kind | IN | Accepts fault, guidance, or gap plus a summary; guidance and gap additionally require a subject, because the subject and not the wording is what holds the fingerprint steady across rewordings | User-input errors are out of scope by construction — the kind set has no member for them | cli/cmd/report.go buildReportEnvelope; cli/cmd/report_test.go TestReportSubjectRequiredForProseKinds |
| issue envelope | OUT | Emits title, labels, body, and a fingerprint that excludes the c3x version, with filesystem paths stripped (filenames kept, directories elided) and entity ids preserved; makes no network call | The `gh` commands ride the help[] line — filing is the agent's act, behind the consent gate | cli/cmd/report.go renderReportBody; cli/cmd/report_test.go TestReportFingerprintIgnoresVersion, TestReportRedactsFilesystemPaths |
| consent policy | IN/OUT | Reads and writes `.c3/report.json`, defaulting to ask, rejecting any field but `consent` so the file cannot become a credential store, and refusing to build a report at all when set to off | Absent `.c3/` yields the ask default rather than an error — a report must survive an unreadable project | cli/cmd/report.go readReportPolicy; cli/cmd/report_test.go TestReportConsentRejectsUnknownFieldInPolicyFile |

## Derived Materials

| Material | Must derive from | Allowed variance | Evidence |
| --- | --- | --- | --- |
| cli/cmd/report.go | Contract | Title wording, body section order, and the fingerprint's normalization may vary while the fingerprint stays version-independent and paths stay stripped | go test ./cmd -run TestReport |
| skills/c3/references/report.md | Purpose | The procedure's prose may vary while it states the same three kinds, the same never-report exclusions, and the same consent gate | go test ./cmd -run TestReport; python3 scripts/test_skill_cli_contract.py |
| .github/ISSUE_TEMPLATE/bug_report.yml | Contract | Field help text may vary while its field labels match the generated body's headings, so human and agent issues share one shape | .github/ISSUE_TEMPLATE/bug_report.yml labels mirror cli/cmd/report.go renderReportBody |
