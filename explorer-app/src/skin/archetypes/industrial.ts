import * as THREE from "three";
import type { ArchetypeBuilder } from "../types";
import { boundaryKindColor } from "../resolve";
import { antenna, box, cyl, dish, emissive, fan, strip, vents } from "../kit";
import { groundText } from "../../scene/label";

/* Every archetype is one industrial family with a distinct silhouette. Each element
 * implies a function: vent → cooling · dish → comms · tank → storage · antenna → network
 * · pipe → transport. Builders return the group plus where the status lamp sits.
 *
 * Materials come from `ctx.m`, colours from `ctx.palette`, so another skin re-dresses these
 * silhouettes by supplying its own material set — or replaces one builder at a time:
 * `{ ...industrialArchetypes, command: myCommand }`. */

export const industrialArchetypes: Record<string, ArchetypeBuilder> = {
  comms(_n, acc, lamp, ctx) {
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(9, 1.6, 9, M.concrete));
    g.add(box(6.5, 3.2, 6.5, M.gunmetal, 0, 1.6));
    g.add(box(3.6, 13, 3.6, M.graphite, 0, 4.8));
    g.add(box(4.2, 0.6, 4.2, M.steel, 0, 9));
    g.add(box(4.2, 0.6, 4.2, M.steel, 0, 14));
    vents(ctx, g, 0, 5.6, 3.3, 2.4, 4);
    vents(ctx, g, -3.3, 5.6, 0, 2.4, 4, "z");
    for (let i = 0; i < 3; i++) g.add(strip(0.14, 1.6, 0.06, 1.2, 10.5 + i * 2.2, 1.84, acc));
    g.add(strip(1.6, 0.9, 0.08, 0, 2.6, 3.3, acc));
    dish(ctx, g, 0, 18.6, 0, 2.2, 0, 0.35);
    antenna(ctx, g, 1.4, 17.8, -1.4, 4.5, lamp);
    antenna(ctx, g, -1.4, 17.8, 1.2, 3.2, null);
    for (const sx of [-1, 1]) g.add(box(2.4, 1.4, 1.8, M.steel, sx * 4.2, 1.6, 2.2));
    return { group: g, roofY: 17.8, lightAt: [-2.6, 4.8, -2.6] };
  },
  command(_n, acc, lamp, ctx) {
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(22, 2.0, 17, M.concrete));
    g.add(box(19, 0.5, 14, M.concrete2, 0, 2.0));
    g.add(box(11, 5.5, 11, M.gunmetal, 0, 2.5));
    g.add(box(12, 0.5, 12, M.steel, 0, 8.0));
    g.add(box(6, 6.5, 6, M.graphite, 0, 8.5));
    g.add(box(6.8, 0.6, 6.8, M.steel, 0, 15));
    const core = cyl(1.6, 3.4, acc, 0, 15.6, 0);
    g.add(core);
    const cover = cyl(2.2, 3.8, M.glass, 0, 15.5, 0);
    cover.castShadow = false;
    g.add(cover);
    g.add(cyl(2.5, 0.4, M.steel, 0, 19.3, 0));
    for (const sx of [-1, 1]) {
      g.add(box(4.5, 3.4, 9, M.gunmetal, sx * 7.7, 2.5, 0));
      vents(ctx, g, sx * (7.7 + 2.26), 3.6, 0, 6.5, 5, "z");
      fan(ctx, g, sx * 7.7, 6.0, -2.4, 1.1);
      fan(ctx, g, sx * 7.7, 6.0, 2.4, 1.1);
    }
    for (let i = 0; i < 3; i++) g.add(strip(9.4, 0.1, 0.06, 0, 4.2 + i * 1.5, 5.54, acc));
    g.add(strip(2.4, 2.2, 0.1, 0, 3.6, 5.56, M.darkMetal));
    g.add(strip(2.0, 0.08, 0.06, 0, 4.75, 5.62, emissive(ctx.palette.amber, 0.9)));
    antenna(ctx, g, 2.4, 15, 2.4, 5, lamp);
    antenna(ctx, g, -2.4, 15, -2.4, 3.6, null);
    antenna(ctx, g, 2.4, 15, -2.4, 2.6, null);
    dish(ctx, g, -6.5, 10.5, 5.2, 1.6, 0, 0.5);
    return { group: g, roofY: 19.5, core, lightAt: [-4.2, 8.5, 4.2] };
  },
  plant(_n, acc, lamp, ctx) {
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(10, 1.2, 10, M.concrete));
    g.add(box(7.5, 4.6, 7.5, M.gunmetal, 0, 1.2));
    g.add(box(8, 0.4, 8, M.steel, 0, 5.8));
    g.add(box(4.5, 2.6, 4.5, M.graphite, -1.0, 6.2, 0.8));
    vents(ctx, g, 0, 2.2, 3.8, 5.5, 6);
    vents(ctx, g, -3.8, 2.2, 0, 5.5, 6, "z");
    fan(ctx, g, 2.6, 6.3, -2.4, 0.9);
    g.add(cyl(0.45, 3.2, M.steel, 3.0, 6.2, 2.4, 12));
    g.add(cyl(0.3, 2.4, M.steel, 2.1, 6.2, 3.0, 10));
    g.add(strip(0.1, 3.2, 0.06, 2.9, 3.6, 3.78, acc));
    g.add(strip(3.0, 0.08, 0.06, -1.4, 4.9, 3.78, acc));
    antenna(ctx, g, -2.8, 8.8, -1.4, 2.2, lamp);
    return { group: g, roofY: 8.8, lightAt: [-3.0, 6.2, -3.0] };
  },
  bunker(_n, acc, lamp, ctx) {
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(18, 0.9, 18, M.concrete));
    g.add(box(15, 2.8, 15, M.concrete2, 0, 0.9));
    g.add(box(12, 1.2, 12, M.gunmetal, 0, 3.7));
    g.add(cyl(4.0, 1.6, M.graphite, 0, 4.9, 0));
    g.add(cyl(4.3, 0.3, M.steel, 0, 6.5, 0));
    for (let i = 0; i < 4; i++) g.add(strip(11.8, 0.05, 0.06, 0, 1.3 + i * 0.55, 7.52, i % 2 ? acc : M.darkMetal));
    for (const sx of [-1, 1]) {
      vents(ctx, g, sx * 7.55, 1.6, -4, 4.5, 4, "z");
      vents(ctx, g, sx * 7.55, 1.6, 4, 4.5, 4, "z");
      g.add(cyl(0.9, 2.2, M.steel, sx * 5.2, 4.9, -3.5, 16));
    }
    g.add(box(3.4, 2.4, 1.0, M.darkMetal, 0, 0.9, 7.6));
    g.add(strip(2.6, 0.06, 0.06, 0, 2.9, 8.12, emissive(ctx.palette.amber, 0.8)));
    antenna(ctx, g, 3.2, 6.8, 3.2, 1.8, lamp);
    return { group: g, roofY: 6.8, lightAt: [-3.2, 4.9, 3.2] };
  },
  transmit(_n, acc, lamp, ctx) {
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(14, 1.2, 10, M.concrete));
    g.add(box(11, 3.6, 7.5, M.gunmetal, 0, 1.2));
    g.add(box(11.5, 0.4, 8, M.steel, 0, 4.8));
    g.add(strip(7.5, 1.0, 0.1, 0, 3.0, 3.8, acc));
    vents(ctx, g, -5.55, 2.0, 0, 5, 4, "z");
    dish(ctx, g, -3.2, 7.4, -1.6, 1.4, 0.4);
    dish(ctx, g, 3.4, 7.0, -1.8, 1.1, -0.5);
    g.add(box(2.4, 1.6, 2.4, M.graphite, 3.6, 5.2, 1.6));
    antenna(ctx, g, 0, 5.2, 1.8, 4.6, lamp);
    return { group: g, roofY: 9.8, lightAt: [-4.6, 5.2, 2.4] };
  },
  outpost(_n, acc, lamp, ctx) {
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(5, 0.6, 5, M.concrete));
    g.add(box(3.2, 2.2, 3.2, M.gunmetal, 0, 0.6));
    g.add(box(3.5, 0.3, 3.5, M.steel, 0, 2.8));
    vents(ctx, g, 0, 1.0, 1.65, 2.2, 3);
    g.add(strip(1.2, 0.06, 0.06, 0, 2.2, 1.66, acc));
    antenna(ctx, g, 1.1, 3.1, -1.1, 2.6, lamp);
    return { group: g, roofY: 3.1, lightAt: [-1.1, 3.1, 1.1] };
  },
  headquarters(_n, acc, lamp, ctx) {
    // Armoured HQ: wide stepped base, two wings, a command tower with a lit core and a radar cluster.
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(26, 2.2, 20, M.concrete));
    g.add(box(23, 0.5, 17, M.concrete2, 0, 2.2));
    g.add(box(15, 6.0, 12, M.gunmetal, 0, 2.7));
    g.add(box(16, 0.5, 13, M.steel, 0, 8.7));
    g.add(box(7, 8.0, 7, M.graphite, 0, 9.2));
    g.add(box(7.8, 0.6, 7.8, M.steel, 0, 17.2));
    const core = cyl(1.4, 2.4, acc, 0, 17.8, 0);
    g.add(core);
    const cover = cyl(1.9, 2.8, M.glass, 0, 17.7, 0);
    cover.castShadow = false;
    g.add(cover);
    g.add(cyl(2.2, 0.35, M.steel, 0, 20.5, 0));
    for (const sx of [-1, 1]) {
      g.add(box(5, 3.6, 11, M.gunmetal, sx * 10, 2.7, 0));
      g.add(box(5.4, 0.4, 11.4, M.steel, sx * 10, 6.3, 0));
      vents(ctx, g, sx * 12.55, 3.4, -2.5, 4, 5, "z");
      vents(ctx, g, sx * 12.55, 3.4, 2.5, 4, 5, "z");
      dish(ctx, g, sx * 10, 9.2, -2.6, 1.5, sx * 0.6, sx > 0 ? 0.3 : -0.25);
      fan(ctx, g, sx * 10, 6.7, 3.2, 1.0);
    }
    dish(ctx, g, -4.8, 11.4, 4.2, 1.2, 0.3, 0.45);
    for (let i = 0; i < 3; i++) g.add(strip(12.6, 0.1, 0.06, 0, 4.4 + i * 1.5, 6.04, acc));
    for (let i = 0; i < 4; i++) g.add(strip(0.12, 1.4, 0.06, -2.4 + i * 1.6, 12.5, 3.54, acc));
    g.add(strip(3.0, 2.6, 0.1, 0, 4.0, 6.06, M.darkMetal));
    g.add(strip(2.6, 0.08, 0.06, 0, 5.4, 6.12, emissive(ctx.palette.amber, 0.9)));
    antenna(ctx, g, 2.6, 17.2, 2.6, 5.5, lamp);
    antenna(ctx, g, -2.6, 17.2, -2.6, 4.2, null);
    antenna(ctx, g, 2.6, 17.2, -2.6, 3.0, null);
    antenna(ctx, g, -2.6, 17.2, 2.6, 2.4, null);
    return { group: g, roofY: 21.5, core, lightAt: [-5.0, 8.7, 5.0] };
  },
  gatehouse(_n, acc, lamp, ctx) {
    // Sector control tower at the district corner: squat base, control room with a lit window band, barrier arm.
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(7, 0.8, 7, M.concrete));
    g.add(box(3.6, 4.4, 3.6, M.gunmetal, 0, 0.8));
    g.add(box(4.4, 1.5, 4.4, M.graphite, 0, 5.2));
    g.add(box(4.8, 0.3, 4.8, M.steel, 0, 6.7));
    for (const [x, z, w, d] of [
      [0, 2.23, 3.6, 0.06],
      [0, -2.23, 3.6, 0.06],
      [2.23, 0, 0.06, 3.6],
      [-2.23, 0, 0.06, 3.6],
    ]) g.add(strip(w, 0.5, d, x, 6.0, z, acc));
    vents(ctx, g, 0, 1.6, 1.83, 2.4, 3);
    g.add(box(1.4, 1.6, 1.0, M.steel, 2.4, 0.8, 2.2));
    const arm = new THREE.Mesh(new THREE.CylinderGeometry(0.07, 0.07, 4.2, 8), M.hazard);
    arm.rotation.z = Math.PI / 2;
    arm.position.set(4.6, 2.0, 2.2);
    arm.castShadow = true;
    g.add(arm);
    g.add(strip(0.5, 0.06, 0.06, 0, 2.4, 1.85, emissive(ctx.palette.amber, 0.8)));
    antenna(ctx, g, -1.4, 7.0, -1.4, 2.4, lamp);
    return { group: g, roofY: 7.0, lightAt: [1.4, 7.0, 1.4] };
  },
  record(_n, acc, _lamp, ctx) {
    // Short marker post: a concrete pad, a steel post, a lit id band; the status lamp caps it.
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(3, 0.3, 3, M.concrete2));
    g.add(cyl(0.14, 1.2, M.steel, 0, 0.3, 0, 8));
    g.add(strip(0.5, 0.08, 0.5, 0, 0.9, 0, acc));
    return { group: g, roofY: 1.5, lightAt: [0, 1.5, 0] };
  },
  zone(n, _acc, _lamp, ctx) {
    // Ground perimeter marking in the boundary-kind colour, corner marks, a small sign post.
    const M = ctx.m;
    const g = new THREE.Group();
    const { w, d } = n.layout;
    const col = boundaryKindColor(ctx.skin, n.boundaryKind || "");
    const line = emissive(col, 0.45);
    const t = 0.25;
    g.add(strip(w, 0.04, t, 0, 0.03, -d / 2, line));
    g.add(strip(w, 0.04, t, 0, 0.03, d / 2, line));
    g.add(strip(t, 0.04, d, -w / 2, 0.03, 0, line));
    g.add(strip(t, 0.04, d, w / 2, 0.03, 0, line));
    const bright = emissive(col, 1.2);
    for (const [sx, sz] of [
      [-1, -1],
      [1, -1],
      [1, 1],
      [-1, 1],
    ]) {
      g.add(strip(1.6, 0.05, 0.4, sx * (w / 2 - 0.8), 0.04, sz * d / 2, bright));
      g.add(strip(0.4, 0.05, 1.6, sx * w / 2, 0.04, sz * (d / 2 - 0.8), bright));
    }
    const fill = new THREE.Mesh(
      new THREE.PlaneGeometry(w, d),
      new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: 0.012, depthWrite: false, toneMapped: false }),
    );
    fill.rotation.x = -Math.PI / 2;
    fill.position.y = 0.02;
    fill.userData.noExport = true;
    g.add(fill);
    const sx = -w / 2 + 1.2;
    const sz = -d / 2 + 1.2;
    g.add(box(1.2, 0.2, 1.2, M.concrete2, sx, 0, sz));
    g.add(cyl(0.07, 2.4, M.steel, sx, 0.2, sz, 6));
    g.add(box(1.6, 0.6, 0.08, M.darkMetal, sx, 2.0, sz));
    g.add(strip(1.3, 0.1, 0.06, sx, 2.3, sz + 0.05, bright));
    if (ctx.labels) {
      const lw = Math.min(w * 0.7, 30);
      const label = groundText(ctx.skin.labels, `${(n.boundaryKind || "zone").toUpperCase()} · ${n.title}`, lw);
      label.position.set(sx + 2.4 + lw / 2, 0.05, sz + 1.6);
      g.add(label);
    }
    return { group: g, roofY: 3.2, lightAt: [sx, 2.6, sz] };
  },
  route(_n, _acc, _lamp, ctx) {
    // Flow signpost beside its first step's source dock: pad, post, angled plate with a violet strip.
    const M = ctx.m;
    const g = new THREE.Group();
    g.add(box(2, 0.3, 2, M.concrete2));
    g.add(cyl(0.08, 3.0, M.steel, 0, 0.3, 0, 6));
    const plate = box(1.8, 0.6, 0.08, M.darkMetal, 0, 2.5, 0);
    plate.rotation.y = -0.5;
    g.add(plate);
    const arrow = strip(1.3, 0.12, 0.06, 0, 2.8, 0.06, emissive(ctx.palette.violet, 1.3));
    arrow.rotation.y = -0.5;
    g.add(arrow);
    return { group: g, roofY: 3.6, lightAt: [0, 3.3, 0] };
  },
};
