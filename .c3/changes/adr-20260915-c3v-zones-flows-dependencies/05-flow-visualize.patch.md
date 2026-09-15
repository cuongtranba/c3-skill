---
target: flow-visualize
scope: whole
type: flow
title: visualize
---
## Goal

Project the live model into the self-contained architecture explorer, from the wrapper invocation to the HTML written for the browser.

## Steps

| Seq | From | To | Action | Evidence |
| --- | --- | --- | --- | --- |
| 1 | c3-203 | c3-107 | Exec the platform binary with `visualize --file out.html` | skills/c3/bin/c3x.sh exec |
| 2 | c3-107 | c3-114 | Dispatch the visual layer without logging an activity entry | cli/main.go case explore/visualize |
| 3 | c3-114 | c3-102 | Read every entity, relationship and eval verdict | cli/cmd/explore.go buildExplorePayload |
| 4 | c3-114 | c3-104 | Read non-terminal patch folders to mark staged facts | cli/cmd/explore.go collectStaging |

## Failure Paths

| At step | Failure | Outcome |
| --- | --- | --- |
| 3 | Payload fails schema validation | Refuses to generate and lists every issue |

