---
target: c3-108
scope: insert
base: c3-108@v1:sha256:d7930ea1d42192ec2c21bc3175046ef8b99505957288d466348cb185848c3cf5
---
## Dependencies

| Depends on | Interaction | Contract | Evidence |
| --- | --- | --- | --- |
| c3-106 | Resolves a fact's code globs to the external files it governs | codemap.GlobFiles | cli/internal/eval imports internal/codemap |
| c3-101 | Reads the fact body whose claim is being checked | content.ReadEntity | cli/internal/eval imports internal/content |

