---
target: c3-101
scope: block
base: c3-101#n523@v1:sha256:e98416c1b91e64944eb8c67c35744f855db8250cf8d47556d7a201d0e0aa0ddc
---
WriteEntity / ReadEntity / RenderMarkdown | OUT | WriteEntity verifies that the body's table rows survive the parse-render round trip, then parses the body, inserts the node tree, snapshots a version, and reseals the entity merkle in one store transaction; ReadEntity renders stored nodes back to markdown that round-trips | Refuses the write, before parsing, when a data row carries more cells than its table header, naming the row and the cells that would be lost — past the parse the overflow is already gone and the truncation is silent; otherwise mutations enlist in the passed store transaction and commit or roll back as one unit, never leaving a half-updated fact | content/bridge_test.go, content/fidelity_test.go, content/render_test.go round-trips
