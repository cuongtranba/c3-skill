---
target: boundary-developer-workstation
scope: whole
type: boundary
title: developer workstation
---
## Goal

Everything C3 ships runs on the developer's own machine: the Go CLI, the Claude skill that drives it, and the dev tooling. Nothing here is a hosted service, so the perimeter is the local process and filesystem, and a crossing means leaving that machine.

## Perimeter

| Kind | Mediation | Evidence |
| --- | --- | --- |
| infra | Local processes over the filesystem; no daemon, no listening socket except the opt-in explore --serve on 127.0.0.1 | this fact |

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
| Release download | Leaves the workstation through boundary-release-egress | boundary-release-egress |

