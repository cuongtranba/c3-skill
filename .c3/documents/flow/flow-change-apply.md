---
id: flow-change-apply
c3-seal: 5fbdd1240f54f83a1321e364eafcc03af8b3d3dbd958e98eaa9b9919e2ab4360
title: change apply
type: flow
goal: Apply a change-unit so its patches land on frozen facts atomically, from the wrapper invocation down to the sealed canonical markdown on disk.
---

## Goal

Apply a change-unit so its patches land on frozen facts atomically, from the wrapper invocation down to the sealed canonical markdown on disk.

## Steps

| Seq | From | To | Action | Evidence |
| --- | --- | --- | --- | --- |
| 1 | c3-203 | c3-107 | Exec the platform binary with `change apply <adr-id>` | skills/c3/bin/c3x.sh exec |
| 2 | c3-107 | c3-112 | Dispatch the change command after opening the store | cli/main.go case change |
| 3 | c3-112 | c3-104 | Run the drift, canvas, morph and retire gates, then apply every patch | cli/cmd/change.go → internal/changeset |
| 4 | c3-104 | c3-102 | Write merged entities, relationships and node trees in one transaction | cli/internal/changeset apply |
| 5 | c3-104 | c3-101 | Re-render and seal the canonical markdown for every touched fact | cli/internal/content WriteEntity |

## Failure Paths

| At step | Failure | Outcome |
| --- | --- | --- |
| 3 | A cited base drifted | Apply refuses before any write and names the stale anchor |
| 4 | Canvas gate fails on a merged body | Whole unit rolls back; nothing is written |
