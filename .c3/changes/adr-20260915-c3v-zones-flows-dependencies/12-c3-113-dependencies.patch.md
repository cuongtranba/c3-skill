---
target: c3-113
scope: insert
base: c3-113@v1:sha256:80adc81c7405d8faaa83857231dd14617088a4ff9f33ab7d42e130b7910513ca
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-105 | Walks .c3/ to rebuild the disposable cache | walker.Walk | cli/cmd/import.go, repair.go |
| c3-102 | Rebuilds and reseals the cache from canonical files | store.Open / EnsureLocalCache | cli/cmd/repair.go imports internal/store |

