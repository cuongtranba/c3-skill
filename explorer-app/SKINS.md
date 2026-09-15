# Skins

A **skin** is the whole visual layer of the city behind one object: colours, materials, the
archetype builders, ground and district dressing, lighting, road styling, label styling and the
CSS chrome. The scene layer (`src/scene/`) owns geometry, layout consumption, picking, selection,
travel, facets, timeline and live mode; it reads everything visual from the active skin and has no
look of its own. A new look is a new skin, never a scene change.

Skins live in `src/skin/`:

| File | What it holds |
|---|---|
| `types.ts` | The `Skin` contract (`Skin`, `SkinTokens`, `MaterialSet`, `ArchetypeBuilder`, `EnvironmentHooks`, `Lighting`, `RoadStyle`, `LabelStyle`) |
| `kit.ts` | Look-free geometry helpers every builder uses (`box`, `cyl`, `strip`, `emissive`, `vents`, `antenna`, `dish`, `fan`, `inkEdges`) and `disposeTree` |
| `resolve.ts` | Token lookups with fallbacks: `kindStyle`, `statusOf`, `boundaryKindColor`, `lifecycleColor`, `districtTint` |
| `archetypes/industrial.ts` | The eleven industrial silhouettes, one builder per archetype |
| `industrial.ts` | The default skin (handoff palette v5) |
| `blueprint.ts` | The engineering-blueprint skin, reusing the industrial builders with its own materials |
| `index.ts` | `SKINS` registry, `resolveSkin`, `initialSkinId`, `applyChrome`, material lifecycle |

## What a skin owns

Everything a user would call "the look":

- **tokens** – background, the semantic `palette` (blue / violet / amber / red / cyan / green / muted / subtle / text / line), edge `kinds` (colour + label), `status` (text + colour + motion per statusKey), `boundaryKinds`, `lifecycle`, `districtTints`, `selection` colour. Fallbacks for unknown vocabulary are part of the tokens (`boundaryFallback`, `lifecycleFallback`; `status.unchecked` must exist; unknown edge kinds resolve to `palette.subtle`).
- **materials** – the twelve named slots (`concrete concrete2 graphite gunmetal steel darkMetal metal hazard glass road trench ground`). Builders only ever read `ctx.m.<slot>`.
- **archetypes** – one builder per archetype: `(node, acc, lamp, ctx) => { group, roofY, lightAt, core? }`. `acc` is the per-node accent material the scene dims and breathes; `lamp` the blinking obstruction light; `ctx` carries the skin, its materials, the palette, the spinner list for semantic motion, the light list and the `labels` flag.
- **decorateNode** (optional) – a pass over every built node, for outlines, decals, edge lines.
- **environment** – three hooks that fill scene-owned groups: `ground(ctx, g, extent)`, `district(ctx, g, district)`, `props(ctx, g, payload)`.
- **lighting** – hemisphere, key (with or without shadows), optional rim, floodlight intensity per district kind, exposure, tone mapping, `fog(homeDist)`, `bloom` or `null`.
- **roads** – shoulder / trench / rail materials (each nullable), marker lamps, route-channel tube parameters, packet size and glow.
- **labels** – fonts, tag box colours, stencil colour, sizes.
- **chrome** – CSS custom properties written onto `:root` (`--bg`, `--panel`, `--panel-2`, `--panel-glass`, `--line`, `--line-2`, `--line-soft`, `--text`, `--muted`, `--subtle`, `--blue`, `--blue-soft`, `--cyan`, `--cyan-soft`, `--amber`, `--red`, `--sil`, `--on-accent`). `styles.css` uses only these.

## What a skin must not touch

- Positions, footprints, docks, route waypoints, districts, roads: they come from the payload. A builder centres its group on the origin and never reads `node.layout.x/z`. (`zone` reads `layout.w/d` because the marking *is* the footprint.)
- Group names and `userData.c3` on the environment, district, `props`, `nodes` and `roads` groups; `userData.noExport` on anything that must not appear in `exportSceneJSON()`; `userData.statusLamp` (the scene adds it); `userData.blink` (mark blinkers with it, never remove it). The export round-trip and inspector tree tests depend on these.
- `window.C3_EXPLORER`, facets, timeline, picking, selection, camera. `roofY` and `lightAt` are the only things a builder tells the scene about itself.
- `ARCH_LABEL` / `ARCH_HEIGHT` in `src/scene/constants.ts` — the export contract, skin-independent.

## Adding a skin

1. **Create `src/skin/<id>.ts`** exporting a `Skin`. Start from `blueprint.ts` (compact) or `industrial.ts` (everything spelled out).
2. **Tokens.** Fill `tokens` for every status key, edge kind and boundary kind the payload can carry. `skin.test.ts` fails if a skin lacks an explicit entry for anything the dev fixture uses.
3. **Materials.** Build a `MaterialSet`. Anything else you share across nodes (an outline ink, a decal material) goes in `extraMaterials` so it survives world rebuilds and is released with the skin.
4. **Builders and environment.** Reuse `industrialArchetypes` as is, spread and override one (`{ ...industrialArchetypes, command: myCommand }`), or write all eleven. Reuse `industrialEnvironment` or write the three hooks. Skins may add a `decorateNode` for per-node passes.
5. **Register** it in `SKINS` in `src/skin/index.ts`. The TopBar control, `C3_EXPLORER.setSkin(id)`, `?skin=<id>` and `window.C3_SKIN` pick it up from there. Run `npm test` — the registry tests iterate every skin.

## Reusing builders

The industrial builders are look-agnostic by construction: they take materials from `ctx.m` and colours from `ctx.palette`, so the same code produces the dark steel city and the blueprint. A skin that only changes materials and tokens is ~150 lines. A skin can also mix: keep ten industrial silhouettes and replace one, or wrap a builder (`(n, acc, lamp, ctx) => decorate(industrialArchetypes.plant(n, acc, lamp, ctx))`).

## Selection order

`window.C3_SKIN` → `?skin=<id>` → `localStorage["c3v.skin"]` → `industrial`. Unknown ids fall through to the next stage. `CityScene.setSkin(id)` rebuilds lighting, post-processing, environment, buildings, roads and traffic in place — positions come from the payload so nothing moves; camera, selection, facets and timeline persist — then writes the chrome variables to `:root` and persists the id.

## Material lifecycle

A skin's materials are module singletons registered as persistent on first use, so disposing a world (live frame, skin switch) frees geometries and per-node emitters but leaves the skin's slots alone. Switching skins releases the previous skin's materials; three.js re-uploads them if that skin is selected again. `skin.test.ts` asserts no material of skin A appears in a world built with skin B, and that disposing a world never disposes its skin's materials.
