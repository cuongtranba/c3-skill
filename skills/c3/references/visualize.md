# Visualize — the model as a city

The **see** beat of Act 1. `visualize` projects the frozen facts into a self-contained 3D infrastructure city: every fact is a building, every wiring edge is a road, districts are containers, traffic is a flow. It is a **read-only projection** of the store — it never writes to `.c3/`, and what it shows is exactly what the facts record (node coverage = store entity set, edge coverage = store relationships). Facts, freeze, and change-units are the shared contract in `SKILL.md`; the model's shape is `canvas.md`'s.

Reach for it when the question is *big picture before detail*: "show me the architecture", "which components sit inside the security perimeter", "how does change apply travel through the system", "what does the whole repo look like".

## Run it

| You want | Run | You get |
|----------|-----|---------|
| One offline HTML file to open or share | `C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" visualize --file out.html` | Single file, no network dependency (three.js, renderer and data inlined) |
| A live view that follows edits | `C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" visualize --serve --port 8722` | `http://127.0.0.1:8722` rebuilding on every mutating command (SSE) |
| The scene for the three.js editor | `C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" visualize --file out.html --export scene.json` | `ObjectLoader` JSON — File → Import at threejs.org/editor; every object carries `userData.c3.id` |
| ADRs as buildings too | add `--include-adr` | Records in the governance perimeter, `affects` roads to staged facts |
| The payload contract | `C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" visualize --schema` | JSON Schema v2 the renderer consumes |

`explore` is an alias of `visualize`; both name the same operation.

## Read the city

| You see | It means |
|---------|----------|
| **District platform** with a stencil label | A container. Its gatehouse tower is the container fact; the buildings on it are its components |
| **Building silhouette** | The archetype, derived from the facts: command centre (best-connected or staged component), compute plant (component), storage bunker (`store/cache/database` in title or goal), comms tower (no incoming `depends_on`), transmit facility (`render/export/report`), perimeter outpost (ref/rule), record post (adr), HQ (system) |
| **Roof lamp** | `statusKey` — STABLE (eval holds) · CHANGING (staged by an open change-unit; the reactor breathes) · DRIFT (eval says code contradicts the fact; red) · REVIEW (needs judgement; amber) · FROZEN/UNCHECKED (no eval spec) |
| **Cable trench** (wide, shared) / **conduit** (narrow) / **dock** | Roads carry wiring edges: blue = `depends_on`, grey = `uses` (cites a ref/rule), purple pulses = a `flow` step, amber = `affects`. Several edges share one trench; each keeps its own lane |
| **Ground perimeter marking** | A `boundary` fact — infra, security or network — enclosing the buildings inside it; nested boundaries nest |
| **Packets moving** | The active flow. *Traffic: active flow* animates only `flow` facts and staged buildings; *all roads* animates every edge; *still* freezes the base |

Click a building → the inspector shows the fact (goal, district, zones, governs, code binding, eval, staged-by, inbound/outbound). Double-click travels the camera to it. Facets (type, district, zone, status, archetype, flow, text, code glob) hide buildings; they never re-lay out the city.

## Make the city say more

The renderer draws only what facts record. When a road, zone or flow is missing, the fix is a **fact**, authored through a change-unit (`change.md`), never a renderer tweak:

| Missing | Author |
|---------|--------|
| A road between two components | A row in the component's `Dependencies` table (`Depends on` = the target id) — the `depends_on` edge |
| A perimeter | A `boundary` fact: `Perimeter.Kind` (security \| network \| infra), `Members` rows (`Member` = the enclosed id); nest with `parent:` — the `encloses` edge. New nested boundaries need a change-unit create patch (a bare `add` gives a custom fact no parent) |
| Traffic | A `flow` fact: ordered `Steps` rows (`Seq`, `From`, `To`, `Action`) — the `flow_from` / `flow_to` edges |

`C3X_MODE=agent bash "<skill-dir>/bin/c3x.sh" schema boundary` and `schema flow` lead with the REJECT-IF each fact must satisfy.

## Verify a render

The HTML exposes `window.C3_EXPLORER` for scripted checks: `renderedNodeIds().length` equals the store's non-hidden entity count, `nodesWithoutStatus()` is empty, `renderedEdgeCount()` equals `dataEdgeCount()`. A payload that fails validation is **refused** before any HTML is written — the CLI prints every issue; fix the facts, not the file.

## Boundaries

- **Conformance** (does the code still match the fact?) is `eval.md`; the city only *shows* the last verdict.
- **Impact** ("what breaks if I change X") is `sweep.md`; roads show wiring, not blast radius.
- **Shape** (adding a column, a fact-type) is `canvas.md`.
