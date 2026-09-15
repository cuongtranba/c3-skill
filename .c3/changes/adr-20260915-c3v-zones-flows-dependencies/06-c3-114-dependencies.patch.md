---
target: c3-114
scope: insert
base: c3-114@v1:sha256:997ceecd63ddcd285db3833bbad4c3d337fbab0cad9b9404fc92cb87605bf32b
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-102 | Reads every entity and its relationships to build the payload | store API: AllEntities, RelationshipsFrom, EvalMatch | cli/cmd/explore.go imports internal/store |
| c3-104 | Reads non-terminal change-unit patch folders to derive staged nodes | changeset.ReadPatchDir | cli/cmd/explore.go imports internal/changeset |
| c3-109 | Parses flags and writes the summary through the shared writer | cmd options + output helpers | cli/cmd/options.go --serve/--port/--schema/--file; RunExplore(opts, w) |

