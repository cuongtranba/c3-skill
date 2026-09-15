---
target: c3-101
scope: insert
base: c3-101@v1:sha256:3816c34f2013f3b632c488f39aab59bcf5a0139bb37ec80c1f0fea95f9cdecba
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-103 | Canvas definitions drive parsing, edge-column extraction and validation | schema.Canvas / DefinitionForDir | cli/internal/content imports internal/schema |
| c3-102 | Persists the parsed node tree, seals and relationships | store node/entity API | cli/internal/content imports internal/store |

