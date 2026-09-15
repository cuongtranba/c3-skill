---
target: c3-111
scope: insert
base: c3-111@v1:sha256:1eec648d6ff584dd4bb07e88cf74297cfa2a0dd0653d031bbdcd13d7ad74e373
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-101 | Writes newly authored facts and canvases as canonical markdown | content.WriteEntity | cli/cmd/add.go, write.go import internal/content |
| c3-103 | Validates a new fact against its canvas before it is sealed | schema.DefinitionForDir / Validate | cli/cmd/add.go, write.go import internal/schema |
| c3-102 | Inserts the new entity and its edges | store entity API | cli/cmd/add.go, write.go import internal/store |

