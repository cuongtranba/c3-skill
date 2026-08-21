---
target: c3-1
scope: block
base: c3-1#n1293@v2:sha256:777030cb0c4fa32feb13f5cb4f20ca4a4290ac6dca72f1251e802d73174cec5f
---
Own the entire behavior of C3: parse and render `.c3/` documents, persist the entity-relationship graph, validate canvas conformance, run the change-unit saga that is the only legal mutation path, map facts to the code they govern, and compose a filed-ready report of C3's own defects. The skill (c3-2) and npm client (c3-3) only invoke this binary; no architecture logic lives outside it.
