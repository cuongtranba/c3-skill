---
target: c3-110
scope: insert
base: c3-110@v1:sha256:ec40ed5526634eedad14fd1dd0fd47d006f980ca31b15a18dcc1f99ef9096f10
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-102 | Answers list/read/graph/search from the cache | store query API + FTS | cli/cmd/read.go, graph.go, search.go import internal/store |
| c3-101 | Re-renders entity bodies and sections for read --cite | content.ReadEntity | cli/cmd/read.go imports internal/content |
| c3-106 | Maps files and globs to owning facts for lookup | codemap.CodeMap / GlobFiles | cli/cmd/lookup.go imports internal/codemap |

