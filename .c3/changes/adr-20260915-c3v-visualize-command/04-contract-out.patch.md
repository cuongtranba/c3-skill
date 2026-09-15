---
target: c3-114
scope: block
base: c3-114#n1021@v1:sha256:9a2b5571cec70a1ce4e1be26793d2f38e80dc1081b10aa034ad3bcc7ede85682
---
| HTML payload and scene export | OUT | Emits payload v2 (schemaVersion 2) whose node set equals the store entity set and whose edge set equals the store's membership, canvas-owned, affects and flow-step edges, every node carrying lifecycle, statusKey, archetype, district, layout and docks, plus districts, roads and one route per routed edge; the same geometry is written as ObjectLoader JSON by --export | Single offline HTML (three.js, renderer, data inlined); scene.json objects carry userData.c3 with deterministic uuids | cli/cmd/explore.go renderExplorerHTML; cli/cmd/explore_layout.go layoutCity; cli/cmd/explore_export.go buildSceneJSON; cli/cmd/explore_test.go |
