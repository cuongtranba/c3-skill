# c3v mockups — review artifacts, not shipped

Static, self-contained HTML pages used to review the c3v (C3 visualize) design before
any model or Go change lands. They load three.js from a CDN import map and carry
hardcoded sample data; nothing here is built, embedded, or tested by CI.

| File | Reviews |
|------|---------|
| `c3v-city.html` | **v5 — dark retro-futurist infrastructure base (vertical slice, per `3d-infrastructure-city-handoff.md`).** The change-apply sector as a tactical base: concrete sector platforms with stencil labels and hazard marks, floodlight masts, six industrial archetypes (comms tower, command centre, compute plant, storage bunker, transmit facility, perimeter outpost) with functional detail (vents, fans, dishes, antenna lamps, tanks, maintenance doors), embedded cable trenches as primary trunks with per-edge lanes, secondary conduits, dock terminals, packets only on the active flow, three-level LOD tags (`name / ID / ● STATUS`), RTS camera, tactical selection (brackets + floor ring + lit foundation), console-style inspector, semantic motion only (radar, fans, lamps, reactor) with `prefers-reduced-motion` support. Payload-v2 nodes remain the single source of truth; archetype/district/lanes are a derived city model. |
| `c3v-node-workspace-v3.html` | v3 — dark "hardware module" workspace (glass-shelled cards with ports). Rejected: reads as monitors on a floor. Kept for comparison. |
| `c3v-node-gallery-v2-light.html` | v2 — light-theme specimen gallery. Kept for comparison. |

Open a file directly in a browser (`file://` works). This directory sits outside
`explorer-app/src/`, so the dev bundle watcher in `cli/cmd/explore_devbundle.go`
ignores it and it is outside `c3-114`'s eval code surface.
