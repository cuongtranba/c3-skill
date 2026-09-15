---
id: boundary-developer-workstation
c3-seal: 8781de7096acdc8033e8294a97af0c05afc08316681a98d9c5a7b1960ec67a09
title: developer workstation
type: boundary
goal: 'Everything C3 ships runs on the developer''s own machine: the Go CLI, the Claude skill that drives it, and the dev tooling. Nothing here is a hosted service, so the perimeter is the local process and filesystem, and a crossing means leaving that machine.'
---

## Goal

Everything C3 ships runs on the developer's own machine: the Go CLI, the Claude skill that drives it, and the dev tooling. Nothing here is a hosted service, so the perimeter is the local process and filesystem, and a crossing means leaving that machine.

## Perimeter

| Kind | Mediation | Evidence |
| --- | --- | --- |
| infra | Local processes over the filesystem; no daemon, no listening socket except the opt-in explore --serve on 127.0.0.1 | skills/c3/bin/c3x.sh; cli/cmd/explore_serve.go |

## Members

| Member | Role | Notes |
| --- | --- | --- |
| c3-1 | Runs as a single process per command |  |
| c3-2 | Skill instructions and wrapper executed by the agent host |  |
| c3-4 | Build/test programs run by a developer or CI |  |

## Crossings

| Path | Control | Evidence |
| --- | --- | --- |
| `c3x explore --serve` | Binds 127.0.0.1 only; the SSE stream never leaves the host | cli/cmd/explore_serve.go ListenAndServe on 127.0.0.1 |
| Release download | Leaves the workstation through the release-egress perimeter | skills/c3/bin/c3x.sh npm fallback; packages/cli/src/** |
