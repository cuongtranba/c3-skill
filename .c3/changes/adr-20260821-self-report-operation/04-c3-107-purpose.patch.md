---
target: c3-107
scope: block
base: c3-107#n780@v1:sha256:5956c6f32217ee02efa7cf2ccd66ca245e9767372a7575a31693a9aae19ee8fd
---
Parse argv into options and dispatch to a command handler (`main.go`), resolve the project's `.c3/` directory by walking up from the working dir (`config`), serialize a mutating command through a per-`.c3/` unix-socket leader so two writers never race (`coord`), append every command and its failure cause to the activity trail that the live explorer tails and `report` reads (`activity`), and marshal command results to TOON for machine consumers (`toon`). Non-goals: owning any command's behavior, validating canvases (schema), or persisting facts (store) — it is the bootstrap and the shared runtime, not the work.
