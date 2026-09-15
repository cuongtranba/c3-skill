---
target: c3-107
scope: insert
base: c3-107@v1:sha256:26e198981b42a245053766e1bf24151ff0bef75d51c7021762191ff99adfaf28
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-110 | Dispatches every read-only command | runCommand switch in cli/main.go | cli/main.go case list/check/read/graph/search/lookup |
| c3-111 | Dispatches the authoring commands | runCommand switch in cli/main.go | cli/main.go case add/write/set/canvas/schema |
| c3-112 | Dispatches the change-unit saga | runCommand switch in cli/main.go | cli/main.go case change |
| c3-113 | Dispatches repair/import/export/delete | runCommand switch in cli/main.go | cli/main.go case repair/export/delete |
| c3-114 | Dispatches the visual layer and skips its activity trail | runCommand switch in cli/main.go | cli/main.go case explore; activity skip |
| c3-115 | Dispatches the self-report operation | runCommand switch in cli/main.go | cli/main.go case report |
| c3-102 | Opens the store and ensures the local cache before any command | store.Open + EnsureLocalCache | cli/main.go imports internal/store |

