---
target: boundary-c3-tree-write
scope: whole
type: boundary
title: c3 tree write path
parent: boundary-developer-workstation
---
## Goal

Only a few components may mutate the canonical .c3/ tree or its cache, and frozen facts may change only through an applied change-unit. Enclosing exactly those components makes every write path reviewable and keeps the freeze guard the single gate.

## Perimeter

| Kind | Mediation | Evidence |
| --- | --- | --- |
| security | Frozen-fact guard on write/set/delete plus the change apply gate stack (drift, canvas, morph, retire); atomic all-or-nothing | this fact |

## Members

| Member | Role | Notes |
| --- | --- | --- |
| c3-104 | Applies patches and writes merged facts |  |
| c3-112 | Drives change apply |  |
| c3-111 | Creates facts and edits canvases and change-docs |  |
| c3-113 | Rebuilds, imports, exports and deletes |  |
| c3-102 | The cache every write lands in |  |

## Crossings

| Path | Control | Evidence |
| --- | --- | --- |
| Direct `write`/`set`/`delete` on a frozen fact | Refused by the freeze guard, naming the change-unit path | cli/cmd/freeze.go |
| `change apply` | Four mechanical gates, then one transaction | cli/internal/changeset apply gates |

