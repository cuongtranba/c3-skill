---
target: c3-104
scope: insert
base: c3-104@v1:sha256:2e7da1b50fc729e4c1487d361348bad9d5086321c49288f9527df94cb3988576
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-101 | Parses patch bodies and re-renders the merged fact | content bridge + markdown tables | cli/internal/changeset imports internal/content, internal/markdown |
| c3-102 | Writes entities, relationships and node trees in one transaction | store write API | cli/internal/changeset imports internal/store |

