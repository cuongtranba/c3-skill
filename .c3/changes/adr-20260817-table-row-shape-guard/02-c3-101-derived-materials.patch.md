---
target: c3-101
scope: block
base: c3-101#n527@v1:sha256:d114a42b983fcc6d9a2ec0548504d6ae92e040028802efced27f804297822147
---
cli/internal/{frontmatter,markdown,content}/**.go | Contract | Parser internals (AST mapping, node-hash layout) may vary as long as documents round-trip; a table row's column count may not change across that round trip, and SplitRowCells is the one splitter that decides where a row's cells begin, so cell escaping is contract rather than free variance | go test ./internal/frontmatter/... ./internal/markdown/... ./internal/content/...
