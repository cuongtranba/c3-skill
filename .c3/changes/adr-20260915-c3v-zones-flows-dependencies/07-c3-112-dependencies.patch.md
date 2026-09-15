---
target: c3-112
scope: insert
base: c3-112@v1:sha256:97c0ba5851cd77bb3d41388cf748a8ca6a198e1b3e55437fc80fb504545fd644
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-104 | Runs the change-unit gate stack and applies patches atomically | changeset.Apply / ReadPatchDir / drift gates | cli/cmd/change.go imports internal/changeset |
| c3-102 | Reads the change-doc and targets, writes the merged result | store entity + relationship API | cli/cmd/change.go imports internal/store |
| c3-101 | Renders and seals the merged canonical bodies | content.WriteEntity / ReadEntity | cli/cmd/change.go imports internal/content |
| c3-103 | Validates merged bodies against their canvas | schema.DefinitionForDir | cli/cmd/change.go imports internal/schema |

