---
target: c3-114
scope: block
base: c3-114#n1020@v1:sha256:5265dbaefe32558f050a25ad4d87f14d5e73113c4eb5e7819a0ba51002c95f92
---
| visualize | IN | Reads all entities, every relationship, eval verdicts, the eval `code:` bindings (files, loc, tech), the non-terminal change-unit patch targets and the canvas definitions; never writes to the store or the .c3/ tree | Read-only; a hidden ADR is excluded unless --include-adr is passed; `explore` is an alias | cli/cmd/explore.go buildExplorePayload; cli/cmd/explore_schema.go exploreAllowedFor |
