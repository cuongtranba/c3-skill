# c3v visualize — payload v2 and city layout contract

## Problem

`c3x explore` projects the model as a radial type-ring graph. It cannot show component↔component dependencies (the model has none), flows, or boundaries, and the visual reads as a graph editor rather than an architecture. The approved redesign (`explorer-app/mockup/c3v-city.html`, handoff `3d-infrastructure-city-handoff.md`) turns the model into a dark infrastructure city: buildings, districts, cable trenches, traffic.

This spec fixes the **payload v2 contract** between the Go CLI (`c3x visualize`, alias `explore`) and the renderer (`explorer-app/`), so both sides can be built in parallel. It also fixes the **city layout algorithm**, which lives in Go so the `--export scene.json` and the live renderer share one geometry.

Everything here is a *derived visualization model*. The domain model (`.c3/` facts, store relationships) stays the single source of truth; nothing in the payload is written back.

## Model prerequisites (M1)

Three canvas additions make the missing relationships authorable. They use the existing edge-column mechanism (`cli/internal/content/edges.go`), so the store needs no Go changes.

| Canvas | Section | Column | Edge | Targets |
|---|---|---|---|---|
| `component`, `container` (new last section `Dependencies`) | `Dependencies` | `Depends on` (reference) | `depends_on` | component, container |
| new fact-type `boundary` | `Perimeter` (`Kind` enum `security|network|infra|N.A - <reason>`) · `Members` | `Member` (reference) | `encloses` | container, component |
| new fact-type `flow` | `Steps` (`Seq` text, `From`, `To`, `Action`, `Evidence`) | `From` / `To` (reference) | `flow_from` / `flow_to` | container, component |

Row order of `Steps` is the step order; `Seq` is authoritative when it parses as an integer.

## Payload v2

Emitted as `window.C3_DATA = {...}` in the HTML, as the `payload` SSE frame in `--serve`, and validated fail-closed before either. `c3x visualize --schema` prints the JSON Schema (draft-07, `$id` `…/architecture-explorer.v2.json`).

```jsonc
{
  "schemaVersion": 2,
  "project": "c3-design",
  "generatedAt": "2026-09-15T10:00:00Z",

  "nodes": [ /* every non-hidden store entity — AG-1 */ {
    "id": "c3-112", "type": "component", "title": "change-cmds", "goal": "…",
    "parent": "c3-1", "level": "component",
    "lifecycle": "staged", "staged": true, "stagedBy": ["adr-…"], "transition": { "from": "frozen", "to": "changing", "by": "adr-…" },
    "category": "feature",                       // component only, may be ""
    "tech": "Go",                                // derived from code globs; "" when unknown
    "boundaries": ["boundary-dev-machine", "boundary-store-write"], // outer → inner closure
    "boundaryKind": "",                          // boundary nodes only: security|network|infra|<enum value>
    "legacyBoundary": "",                        // container free-text `boundary:` field
    "code": { "globs": ["cli/cmd/change*.go"], "files": 9, "loc": 3100 } /* or null */,
    "eval": { "verdict": "holds" }               /* holds|drift|needs-judgement|unchecked, or null when no eval spec */,
    "statusKey": "changing",                     // stable|changing|drift|review|unchecked|open|accepted|done|superseded|governance
    "archetype": "command",                      // see §Archetypes
    "district": "c3-1",                          // district id; "" for zone/route archetypes
    "importance": 1.0,
    "layout": { "x": 0, "y": 0.6, "z": 4, "w": 22, "d": 17 },  // ground-centre, platform height, footprint
    "docks": [ { "id": "c3-112:s:0", "face": "s", "x": -0.9, "z": 13.2, "edgeId": "c3-112→c3-104#depends_on" } ]
  } ],

  "edges": [ /* AG-2: contains + every canvas-owned rel type + affects (+ flow_step) */
    { "id": "c3-1→c3-112#contains", "from": "c3-1", "to": "c3-112", "kind": "contains" },
    { "id": "c3-112→c3-104#depends_on", "from": "c3-112", "to": "c3-104", "kind": "depends_on", "label": "apply unit" },
    { "id": "c3-112→c3-104#flow_step", "from": "c3-112", "to": "c3-104", "kind": "flow_step", "flow": "flow-change-apply", "seq": 2, "label": "dispatch apply" }
  ],

  "flows": [ { "id": "flow-change-apply", "title": "change apply", "steps": [ { "seq": 1, "from": "c3-109", "to": "c3-112", "action": "dispatch" } ] } ],
  "boundaries": [ { "id": "boundary-dev-machine", "kind": "infra", "parent": "", "members": ["c3-1", "c3-2"] } ],

  "districts": [ { "id": "c3-1", "title": "SECTOR 01 · GO CLI", "kind": "sector", "x": 0, "z": 0, "w": 90, "d": 60, "y": 0.6, "members": ["c3-112", "…"] },
                 { "id": "hq", "title": "HQ · C3-DESIGN", "kind": "hq", … }, { "id": "governance", "title": "PERIMETER · GOVERNANCE", "kind": "governance", … } ],
  "roads": {
    "streets": [ { "id": "c3-1:street:0", "district": "c3-1", "z": -30, "x0": -50, "x1": 50, "width": 3, "major": false }, { "id": "main-trunk", "district": "", "z": -80, "x0": -140, "x1": 140, "width": 4.2, "major": true } ],
    "avenues": [ { "id": "c3-1:avenue:w", "district": "c3-1", "x": -48, "z0": -80, "z1": 60, "width": 2.2 } ]
  },
  "routes": [ /* one per non-`contains` edge */
    { "id": "c3-112→c3-104#depends_on", "edgeId": "c3-112→c3-104#depends_on", "kind": "depends_on", "active": true,
      "segments": ["c3-1:street:1"], "lane": 0.35, "sharedWith": "",
      "waypoints": [[-0.9, 0.45, 13.2], [-0.9, 0.16, 14.6], [-0.9, 0.16, -30.35], [-30, 0.16, -30.35], [-30, 0.16, -1.4], [-30, 0.45, 0]] }
  ],

  "events": [ /* unchanged ADR timeline */ ]
}
```

Rules:

- `type` and edge `kind` are **open strings**: allowed node types = ids of `schema.AllDefinitions(c3Dir)`; allowed kinds = `{contains, affects, flow_step}` ∪ every canvas-owned rel type. The old hardcoded enum sets are removed. `level`, `lifecycle`, `eval.verdict`, `statusKey`, `archetype`, `docks[].face`, `districts[].kind` stay closed.
- `statusKey`: `staged → changing`; `adr → its state`; `ref|rule → governance`; else by `eval.verdict`: `holds → stable`, `drift → drift`, `needs-judgement → review`, none/unchecked → `unchecked`.
- `boundaries[]` on a node = the boundaries that `encloses` it, plus those enclosing its `parent` chain, plus each boundary's own `parent` chain; ordered outermost first; deduplicated.
- `tech`: extension histogram of files matched by the node's eval `code:` globs, mapped `.go→Go .ts/.tsx→TypeScript .js/.jsx→JavaScript .py→Python .rs→Rust .md→Markdown .sh→Bash .yaml/.yml→YAML .json→JSON`; the top two joined with ` · `; `""` when no binding.
- `code.loc` counts newline characters in matched files ≤ 1 MiB; binary/huge files are counted in `files` but not `loc`.
- Every non-`contains` edge has exactly one route; `routes[].waypoints` has ≥ 2 points, starts at the source dock and ends at the target dock, and every interior point lies on a street or avenue centreline offset by `lane`. A `flow_step` edge whose endpoints also carry a `depends_on` edge reuses that route's waypoints and sets `sharedWith` to it.
- `active` on a route is true when the edge is a `flow_step`, or a `depends_on` whose endpoints are consecutive steps of some flow, or the source node is `staged`.
- The validator (`explore_schema.go`) checks all of the above, plus: node ids unique; edge endpoints exist; `flow` ids exist and `seq` strictly increases within a flow; boundary members and node `boundaries[]` resolve to `boundary` nodes; every node has `layout.w > 0`, `layout.d > 0`, a known `archetype`, and (unless zone/route) a `district` that exists; every dock's `edgeId` exists; `sharedWith` resolves.

## Archetypes (derived in Go)

| Entity | Archetype | Rule |
|---|---|---|
| system | `headquarters` | always |
| container | `gatehouse` | always (the sector's control building) |
| ref, rule | `outpost` | always |
| adr | `record` | always (only with `--include-adr`) |
| boundary | `zone` | always — a ground marking, layout = members' bounding box + 4 |
| flow | `route` | always — a signpost beside the first step's source dock |
| component | `bunker` | `title+goal` matches `/store|cache|sqlite|database|persist|repositor/i` |
| component | `transmit` | matches `/explore|render|export|report|visual|emit|output/i` |
| component | `comms` | no incoming `depends_on` within its sector and ≥ 1 outgoing |
| component | `command` | exactly one per sector among the remaining: the staged one, else the highest `depends_on` degree, ties by id |
| component | `plant` | everything else |
| other custom fact types | `plant` | |

Footprints (w × d) and importance: headquarters 26×20 / 1.0 · command 22×17 / 1.0 · comms 9×9 / 0.8 · bunker 18×18 / 0.75 · plant 10×10 / 0.6 · transmit 14×10 / 0.6 · gatehouse 7×7 / 0.5 · outpost 5×5 / 0.3 · record 3×3 / 0.2 · zone bbox / 0 · route 2×2 / 0.

## City layout (Go, `cli/cmd/explore_layout.go`)

Deterministic, pure function of the payload's nodes/edges/flows/boundaries. Units are world units; +X east, +Z south (toward the default camera).

1. **Districts.** One `sector` per container (ordered by id), one `hq` for the system, one `governance` for outposts and records. Components with a parent that is not a container (or no parent) fall into the sector of their nearest container ancestor, else into `governance`.
2. **Layering inside a sector.** Longest-path layering over `depends_on` edges whose endpoints are both in the sector (Kahn; a back edge that would create a cycle is ignored for layering, ties broken by id). Row 0 is north. Row pitch = `max(footprint.d in sector) + 14`. Within a row nodes are ordered by id and spaced `max(footprint.w in row) * 2.5` centre-to-centre, centred on the sector's x. The gatehouse sits at the sector's north-west corner.
3. **Sector size.** `w = max(row width) + 2*pad`, `d = rows*pitch + 2*pad`, `pad = 10`. Sectors are laid west→east at `z = 0` with gaps of `3 * max footprint w` (≥ 40). HQ is centred north of the sectors (`z = sectors.top − 60`). Governance is a single east–west row south of the sectors (`z = sectors.bottom + 50`), outposts spaced 3W apart. Platform heights: sector 0.6, hq 1.1, governance 0.
4. **Roads.** Per sector: `rows+1` horizontal **streets** at every row boundary (z between rows, and above row 0 / below the last), width 3, spanning the sector plus 3 units of overshoot each side; two **avenues** at `x = left − 3` and `x = right + 3`, width 2.2, spanning from the main trunk to the governance trench. Globally: the **main trunk** (`major: true`, width 4.2) at `z = sectors.top − 12` spanning all sectors and the HQ; the **governance trench** (`major: true`) at `z = sectors.bottom + 12`. HQ has its own two avenues down to the main trunk.
5. **Docks.** For each non-`contains` edge: the source gets a dock on its face toward the first street the route uses (`s` if that street is south of it, else `n`); the target likewise. Docks on one face are spread 1.8 apart, centred, ordered by edge id; dock `y = layout.y + 0.45`; dock `z = building face ± 1.4`.
6. **Routes.** Build a graph whose vertices are every street×avenue intersection, every street×trunk intersection, and each dock's projection onto its street (same x, street z). Edge weights are Manhattan lengths along the shared street/avenue. Run Dijkstra from the source projection to the target projection; waypoints = `[dock, projection, …intersections…, projection, dock]`, with collinear interior points removed. **Lanes:** routes sharing a street/avenue/trunk segment get a lane index by route-id order; `lane = (index − (n−1)/2) * 0.7`, applied as a perpendicular offset to every interior waypoint (z-offset on horizontal segments, x-offset on vertical ones). Waypoint y = 0.16 on the ground, dock y at the ends.
7. **Zones and routes as nodes.** A `boundary` node's layout is the bounding box of its members' footprints padded by 4 (nested boundaries therefore nest). A `flow` node's layout is a 2×2 spot 3 units west of its first step's source dock.

Complexity: O(V·log V) per route on a graph of a few hundred vertices — fine for 500 nodes.

## `--export scene.json`

`buildSceneJSON(payload)` emits THREE `ObjectLoader` JSON (`metadata {version: 4.6, type: "Object", generator: "c3x visualize"}`): one `Group` per district (platform `BoxGeometry`), one `Group` per node (a `BoxGeometry` of the footprint with archetype height: headquarters 20, command 19, comms 18, bunker 7, plant 9, transmit 8, gatehouse 6, outpost 3, record 1.5, zone 0.1, route 0.5), one `Line` (`BufferGeometry` positions = waypoints) per route. Every object carries `userData.c3 = { id, type, kind?, archetype?, district? }`. UUIDs are `sha256(id)` formatted 8-4-4-4-12 so re-exports diff cleanly. Materials: `MeshStandardMaterial` gunmetal for bodies, `LineBasicMaterial` in the kind colour for routes.

## CLI surface

| Command | Behaviour |
|---|---|
| `c3x visualize [--file out.html] [--include-adr] [--export scene.json]` | self-contained HTML (+ optional scene export) |
| `c3x visualize --serve [--port 8722]` | live server, unchanged SSE frames (`payload` now v2) |
| `c3x visualize --schema` | JSON Schema v2 |
| `c3x explore …` | alias — `options.go` normalises the command name to `visualize` |

`help.go` registers `visualize` (near `graph`); `main.go` skips the activity trail for `visualize` as it did for `explore`. All errors keep the `error: …\nhint: …` shape.

## Renderer contract (`explorer-app/`)

- Consumes payload v2 only; must tolerate `agentRoutes` being absent (reserved key, not emitted yet).
- `window.C3_EXPLORER` keeps every existing member (`renderedNodeIds`, `renderedEdgeCount`, `dataEdgeCount`, `nodesWithoutStatus`, `selectNodeById`, `setLevel`, `currentSelection`, `timeline.*`) and adds `focus(id)`, `setMotion("active"|"all"|"none")`, `facets.set/get/clear`, `exportSceneJSON()`, `inspector.tree()`.
- `renderedNodeIds()` must equal the payload node ids (zones and routes render as ground markings/signposts but are nodes).
- Visual language = `explorer-app/mockup/c3v-city.html` v5: handoff palette, six industrial archetypes, tactical selection, stencil LOD tags, console inspector, semantic motion only, `prefers-reduced-motion` honoured, bloom as enhancement.
- Facets (all conjunctive): type, district, boundary id, boundary kind, `statusKey`, lifecycle, archetype, flow (highlights its steps), text, code glob. Filtering hides; it never re-lays out.
- Live mode: `payload` frames replace the model and rebuild the city (positions come from the payload, so they are stable across rebuilds); `action` frames pulse the touched buildings' status lamps.

## Non-goals

| Non-goal | Rule |
|---|---|
| A second graph command or model | `visualize` reads the store; `graph`/`search` stay the text surfaces (see `2026-07-09-architecture-trace-graph.md`) |
| Writing anything to `.c3/` | archetype/district are derived; a future canvas column may pin them |
| Agent RAG route overlay in this release | `agentRoutes` is reserved; renderer must not depend on it |
