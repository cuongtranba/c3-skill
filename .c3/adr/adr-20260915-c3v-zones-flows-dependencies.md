---
id: adr-20260915-c3v-zones-flows-dependencies
c3-seal: aeb982d867df055aee50492bbe95e4d1e0b37328714d5b72684a2236fe810ac9
title: c3v-zones-flows-dependencies
type: adr
goal: 'Author the model the c3v visualizer renders: three boundary facts (infra, security, network perimeters of c3-design), two flow facts (change apply, visualize), and a Dependencies section on every Go CLI component and the wrapper/thin-client/harness components, so component-to-component depends_on edges, flows and zones exist as frozen facts instead of prose.'
status: done
date: "2026-09-15"
---

## Goal

Author the model the c3v visualizer renders: three boundary facts (infra, security, network perimeters of c3-design), two flow facts (change apply, visualize), and a Dependencies section on every Go CLI component and the wrapper/thin-client/harness components, so component-to-component depends_on edges, flows and zones exist as frozen facts instead of prose.

## Context

Until now the store held no component-to-component wiring: every `uses` edge was a component citing a ref or rule, collaboration lived in Parent Fit prose, and the only boundary was a free-text field on containers. A visualizer can only draw what facts record, so the city view (districts, roads, traffic, zones) had nothing to render. The canvases now carry the shape (Dependencies section with an `edge: depends_on` column; `boundary` and `flow` fact-types with `encloses` and `flow_from`/`flow_to` columns); this unit fills them with facts verified against the Go import graph and the wrapper.

## Decision

Wire dependencies as body-owned edge columns rather than frontmatter lists, so the store, `graph`, and the visualizer share one source and the freeze applies. Model perimeters as first-class `boundary` facts (nested via `parent:`) rather than a `zones:` field, so each has a goal, a mediation and reviewable crossings. Model execution paths as `flow` facts with ordered steps rather than ad-hoc mermaid, so traffic in the visualizer is derived from facts.

## Affected Topology

| Entity | Type | Why affected | Evidence | Governance review |
| --- | --- | --- | --- | --- |
| c3-0 | system | Gains three boundary facts and two flow facts as new top-level fact types; system shape unchanged | c3-0#n3@v1:sha256:cee3eb278e1317505a0e044598e7ab83b4bfd3a67024817020ae07564393a2ff | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-1 | container | Its components gain Dependencies sections; membership unchanged | c3-1#n640@v1:sha256:f7c8f25904e4c5ef4c42311b9e67d9fec19c9243532ecd2cb99d28c00b212ec3 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-114 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-114#n1002@v1:sha256:b1a461f193f88817bab487f504166b427805515d79177d31f239d38ade4b12e8 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-112 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-112#n950@v1:sha256:3abf885d265f42cbc830b4c11fe638130be6afcd9127302172c893461adad372 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-104 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-104#n740@v1:sha256:430d075e478122854ce9a8ed4b1ed09e8b7651e95ab0638b5faa817a58910f03 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-101 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-101#n665@v1:sha256:063a003da99e6006346ac8b448d2f8192af740dc7fc1b5def92614fa4b7a4ee6 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-110 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-110#n897@v1:sha256:4630a107a83da0e8da00b79e9d7d01400ab1f0bd332cb1e157c9e46188b4dad6 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-111 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-111#n923@v1:sha256:22bfaf83cbf9c03059e0b77b544f21dbaf5d2dbdf2dcb6e40ec9b417e6bf4f0a | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-113 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-113#n977@v1:sha256:2685c09a8835ccc01d5181128ed81be0c0ce338a4862945638c8049b222bbac9 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-108 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-108#n842@v1:sha256:24d48b0deec2f5a52b766b36e36c519ed5f2a7dc9a0b30738afcc0da197853a7 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-107 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-107#n815@v1:sha256:7ef9fe2f74ce2cbc3479d4dd78dd78cd64c63085111606d631d3722d239c3b8a | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-203 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-203#n1121@v1:sha256:54edea1eb796d0b265904816dad70fc6cf7ee1e15430a72c1d27662a9ad038ce | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-301 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-301#n1159@v1:sha256:70569c86bdff790c595cf02aa8a10440bc66d7c9f6db5e0869f260baa121e555 | Parent Delta: none — additive optional section; membership rows synthesized |
| c3-401 | component | Gains a Dependencies section wiring depends_on edges the visualizer renders as roads | c3-401#n1199@v1:sha256:2617afa616475d88cbedd7236b7d93626e6d58c4e58afbd57957d4b87ffc5c2c | Parent Delta: none — additive optional section; membership rows synthesized |

## Verification

| Check | Result |
| --- | --- |
| `c3x check` after apply | ok: true, every new fact valid against its canvas |
| `sqlite3 .c3/c3.db 'select rel_type,count(*) from relationships group by 1'` | rows for depends_on, encloses, flow_from, flow_to |
| `c3x graph flow-change-apply --json` | the flow's participants appear in the route graph |
| `c3x visualize --export /tmp/scene.json` | every boundary, flow and depends_on edge present in the payload |
