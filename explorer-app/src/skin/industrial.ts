import * as THREE from "three";
import type { C3Payload, District, Street } from "../types";
import type { EnvironmentHooks, Extent, MaterialSet, Palette, Skin, SkinContext } from "./types";
import { industrialArchetypes } from "./archetypes/industrial";
import { box, cyl, emissive, strip } from "./kit";
import { groundText } from "../scene/label";
import { hash } from "../scene/constants";

/* The default skin: handoff palette 3d-infrastructure-city v5. Blue is the only saturated
 * accent and stays a small share of the frame; governance conduits are near-neutral; the
 * frame is 70–85 % dark and the world brings its own light (floodlights, windows, lamps). */

const BG = "#060708";

const palette: Palette = {
  blue: "#3C8DFF",
  violet: "#7657FF",
  amber: "#E9A83E",
  red: "#E04747",
  cyan: "#26C6DA",
  green: "#61D67C",
  muted: "#8a95a3",
  subtle: "#5c6a7d",
  text: "#dfe4ea",
  line: "#3a4149",
};

/* One industrial material family shared by environment, buildings and roads:
 * dark steel, gunmetal, graphite, reinforced concrete. */
const materials: MaterialSet = {
  concrete: new THREE.MeshStandardMaterial({ color: "#2B2D2F", roughness: 0.95, metalness: 0.0 }),
  concrete2: new THREE.MeshStandardMaterial({ color: "#232527", roughness: 0.95, metalness: 0.0 }),
  graphite: new THREE.MeshStandardMaterial({ color: "#1A1E24", roughness: 0.72, metalness: 0.2 }),
  gunmetal: new THREE.MeshStandardMaterial({ color: "#252B32", roughness: 0.6, metalness: 0.35 }),
  steel: new THREE.MeshStandardMaterial({ color: "#343b44", roughness: 0.42, metalness: 0.75 }),
  darkMetal: new THREE.MeshStandardMaterial({ color: "#101216", roughness: 0.55, metalness: 0.6 }),
  metal: new THREE.MeshStandardMaterial({ color: "#2d343d", roughness: 0.45, metalness: 0.7 }),
  hazard: new THREE.MeshStandardMaterial({ color: "#E9A83E", roughness: 0.8, metalness: 0.0 }),
  glass: new THREE.MeshPhysicalMaterial({
    color: "#6f8fb8",
    roughness: 0.15,
    metalness: 0,
    transmission: 0.55,
    thickness: 0.6,
    transparent: true,
    opacity: 0.6,
  }),
  road: new THREE.MeshStandardMaterial({ color: "#0d0f12", roughness: 0.9, metalness: 0.05 }),
  trench: new THREE.MeshStandardMaterial({ color: "#07080a", roughness: 1.0, metalness: 0.0 }),
  ground: new THREE.MeshStandardMaterial({ color: "#0a0b0e", roughness: 0.96, metalness: 0.02 }),
};

const rnd = (seed: number): number => {
  const x = Math.sin(seed) * 10000;
  return x - Math.floor(x);
};

/** A mast plus a warm-white spot onto the platform; physically motivated intensities (candela). */
function floodlight(ctx: SkinContext, g: THREE.Group, x: number, z: number, tx: number, tz: number, intensity: number): void {
  const l = new THREE.SpotLight("#d6dceb", intensity, 140, 0.8, 0.7, 1.3);
  l.position.set(x, 22, z);
  l.target.position.set(tx, 0, tz);
  l.userData.noExport = true;
  l.target.userData.noExport = true;
  g.add(l, l.target);
  ctx.lights.push(l);
  g.add(cyl(0.16, 22, ctx.m.darkMetal, x, 0, z, 8, 0.22));
  g.add(box(1.4, 0.5, 0.9, ctx.m.metal, x, 21.8, z));
  const lamp = box(1.1, 0.12, 0.6, emissive("#cfd6e6", 1.1), x, 21.5, z);
  lamp.rotation.x = 0.5;
  g.add(lamp);
}

/* The ground is constructed, not graph paper: an asphalt plane, faint concrete seams,
 * maintenance hatches and recessed lights. Districts are concrete foundations with stencil
 * labels, hazard chevrons on the front edge and a floodlight mast per sector. Props are
 * substations, coolers and utility boxes along the trunks — context, never nodes. */
export const industrialEnvironment: EnvironmentHooks = {
  ground(ctx, g, ext: Extent) {
    const span = ext.size * 2.4 + 300;
    const ground = new THREE.Mesh(new THREE.PlaneGeometry(span, span), ctx.m.ground);
    ground.rotation.x = -Math.PI / 2;
    ground.position.set(ext.cx, -0.02, ext.cz);
    ground.receiveShadow = true;
    ground.name = "ground";
    g.add(ground);

    const seams = new THREE.GridHelper(span, Math.round(span / 30), "#111317", "#111317");
    (seams.material as THREE.Material).transparent = true;
    (seams.material as THREE.Material).opacity = 0.5;
    seams.position.set(ext.cx, 0, ext.cz);
    seams.userData.noExport = true;
    g.add(seams);

    const hatchN = Math.round(40 * Math.max(1, ext.size / 200));
    const lampN = Math.round(22 * Math.max(1, ext.size / 200));
    const hatch = new THREE.InstancedMesh(new THREE.BoxGeometry(1.6, 0.06, 1.6), ctx.m.steel, hatchN);
    const lamps = new THREE.InstancedMesh(new THREE.CylinderGeometry(0.18, 0.18, 0.05, 8), emissive(ctx.palette.muted, 0.35), lampN);
    const m = new THREE.Matrix4();
    const w = ext.maxX - ext.minX + 120;
    const d = ext.maxZ - ext.minZ + 120;
    for (let i = 0; i < hatchN; i++) {
      m.setPosition(ext.minX - 60 + rnd(i * 3.1) * w, 0.01, ext.minZ - 60 + rnd(i * 7.7) * d);
      hatch.setMatrixAt(i, m);
    }
    for (let i = 0; i < lampN; i++) {
      m.setPosition(ext.minX - 60 + rnd(i * 5.3 + 1) * w, 0.02, ext.minZ - 60 + rnd(i * 2.9 + 2) * d);
      lamps.setMatrixAt(i, m);
    }
    hatch.receiveShadow = true;
    hatch.userData.noExport = true;
    lamps.userData.noExport = true;
    g.add(hatch, lamps);
  },

  district(ctx, g, d: District) {
    const tints = ctx.skin.tokens.districtTints;
    if (d.y > 0) {
      const plat = new THREE.Mesh(new THREE.BoxGeometry(d.w, d.y, d.d), new THREE.MeshStandardMaterial({ color: tints[d.kind] ?? tints.hq, roughness: 0.95, metalness: 0 }));
      plat.position.set(d.x, d.y / 2, d.z);
      plat.receiveShadow = true;
      plat.castShadow = true;
      plat.name = `${d.id}:platform`;
      g.add(plat);
      const lip = new THREE.Mesh(new THREE.BoxGeometry(d.w + 1.2, 0.25, d.d + 1.2), ctx.m.concrete2);
      lip.position.set(d.x, 0.125, d.z);
      lip.receiveShadow = true;
      g.add(lip);
      for (let x = -d.w / 2 + 3; x < d.w / 2 - 3; x += 6) {
        const h = new THREE.Mesh(new THREE.BoxGeometry(2.2, 0.02, 0.5), ctx.m.hazard);
        h.position.set(d.x + x, d.y + 0.02, d.z + d.d / 2 - 0.6);
        g.add(h);
      }
      // Masts stand at the platform corners, one per ~70 units of the long side, alternating
      // sides so a deep sector is lit end to end; each aims at the platform centre line.
      const intensity = ctx.skin.lighting.floodlight[d.kind] ?? ctx.skin.lighting.floodlight.sector ?? 0;
      const longZ = d.d >= d.w;
      const along = longZ ? d.d : d.w;
      const masts = Math.max(1, Math.ceil(along / 70));
      for (let i = 0; i < masts; i++) {
        const t = masts === 1 ? -1 : (i / (masts - 1)) * 2 - 1;
        const side = i % 2 === 0 ? 1 : -1;
        const mx = longZ ? d.x + side * (d.w / 2 - 3) : d.x + t * (d.w / 2 - 3);
        const mz = longZ ? d.z + t * (d.d / 2 - 3) : d.z - side * (d.d / 2 - 2);
        const tx = longZ ? d.x - side * d.w * 0.15 : mx - t * d.w * 0.2;
        const tz = longZ ? mz - t * d.d * 0.2 : d.z + side * d.d * 0.15;
        floodlight(ctx, g, mx, mz, tx, tz, intensity);
      }
    } else {
      // Governance perimeter: a flat concrete apron with a painted edge line, no platform.
      const apron = new THREE.Mesh(new THREE.BoxGeometry(d.w, 0.04, d.d), new THREE.MeshStandardMaterial({ color: tints[d.kind] ?? tints.governance, roughness: 1 }));
      apron.position.set(d.x, 0.0, d.z);
      apron.receiveShadow = true;
      g.add(apron);
      g.add(strip(d.w, 0.02, 0.3, d.x, 0.03, d.z - d.d / 2, emissive(ctx.palette.line, 0.6)));
      g.add(strip(d.w, 0.02, 0.3, d.x, 0.03, d.z + d.d / 2, emissive(ctx.palette.line, 0.6)));
    }
    if (ctx.labels) {
      const lw = Math.min(26 + d.title.length * 0.4, d.w * 0.8);
      const lbl = groundText(ctx.skin.labels, d.title, lw);
      lbl.position.set(d.x - d.w / 2 + lw / 2 + 1.5, d.y + 0.03, d.z + d.d / 2 - 3.2);
      g.add(lbl);
    }
  },

  /** Placed deterministically from the road ids. */
  props(ctx, g, payload: C3Payload) {
    const M = ctx.m;
    const sub = (x: number, z: number): void => {
      const s = new THREE.Group();
      s.add(box(4.5, 0.3, 3.5, M.concrete2));
      s.add(cyl(0.7, 1.6, M.darkMetal, -1.1, 0.3, 0, 12));
      s.add(cyl(0.7, 1.6, M.darkMetal, 0.9, 0.3, 0, 12));
      s.add(box(0.5, 1.0, 2.6, M.steel, 2.0, 0.3, 0));
      s.add(cyl(0.05, 2.2, M.steel, -2.0, 0.3, 1.4, 6));
      s.add(strip(0.3, 0.3, 0.3, -2.0, 2.6, 1.4, emissive(ctx.palette.amber, 1.2)));
      s.position.set(x, 0, z);
      g.add(s);
    };
    const cooler = (x: number, z: number): void => {
      const c = new THREE.Group();
      c.add(cyl(1.6, 3.2, M.concrete2, 0, 0, 0, 20, 2.0));
      c.add(cyl(1.7, 0.2, M.steel, 0, 3.2, 0, 20));
      c.position.set(x, 0, z);
      g.add(c);
    };
    const util = (x: number, z: number): void => {
      g.add(box(1.2, 0.9, 1.0, M.darkMetal, x, 0, z));
    };

    const majors = payload.roads.streets.filter((s) => s.major);
    for (const t of majors) {
      const h = hash(t.id);
      const len = t.x1 - t.x0;
      // Pipe run along the north shoulder, with supports.
      const pipe = cyl(0.22, len * 0.85, M.steel, (t.x0 + t.x1) / 2, 0.9, t.z - t.width / 2 - 3.4, 10);
      pipe.rotation.z = Math.PI / 2;
      g.add(pipe);
      for (let x = t.x0 + len * 0.1; x < t.x1 - len * 0.05; x += Math.max(18, len / 6)) g.add(box(0.6, 1.0, 0.6, M.darkMetal, x, 0, t.z - t.width / 2 - 3.4));
      sub(t.x0 + 6 + (h % 7), t.z + t.width / 2 + 5);
      sub(t.x1 - 8 - ((h >> 3) % 7), t.z - t.width / 2 - 7);
      cooler(t.x0 + len * 0.12, t.z + t.width / 2 + 8);
      cooler(t.x0 + len * 0.12 + 4, t.z + t.width / 2 + 8);
    }
    const minors: Street[] = payload.roads.streets.filter((s) => !s.major);
    minors.forEach((s, i) => {
      if (i % 2) return;
      const h = hash(s.id);
      util(s.x0 - 2.5 - (h % 3), s.z + (i % 4 === 0 ? 3 : -3));
    });
    payload.roads.avenues.forEach((a, i) => {
      if (i % 2) return;
      util(a.x + (i % 4 === 0 ? 3.2 : -3.2), (a.z0 + a.z1) / 2 + (hash(a.id) % 20) - 10);
    });
  },
};

export const industrial: Skin = {
  id: "industrial",
  label: "Industrial",
  tokens: {
    background: BG,
    palette,
    kinds: {
      depends_on: { color: palette.blue, label: "depends on" },
      uses: { color: palette.subtle, label: "cites" },
      flow_step: { color: palette.violet, label: "flow step" },
      affects: { color: palette.amber, label: "affects" },
      encloses: { color: palette.muted, label: "encloses" },
      flow_from: { color: palette.violet, label: "flow from" },
      flow_to: { color: palette.violet, label: "flow to" },
      contains: { color: palette.line, label: "contains" },
    },
    status: {
      stable: { text: "STABLE", color: palette.green, motion: "none" },
      changing: { text: "CHANGING", color: palette.cyan, motion: "breathe" },
      drift: { text: "DRIFT", color: palette.red, motion: "pulse" },
      review: { text: "REVIEW", color: palette.amber, motion: "pulse-slow" },
      unchecked: { text: "UNCHECKED", color: palette.muted, motion: "none" },
      open: { text: "OPEN", color: palette.blue, motion: "pulse-slow" },
      accepted: { text: "ACCEPTED", color: palette.cyan, motion: "none" },
      done: { text: "DONE", color: palette.green, motion: "none" },
      superseded: { text: "SUPERSEDED", color: palette.subtle, motion: "none" },
      governance: { text: "GOVERNANCE", color: palette.muted, motion: "none" },
    },
    boundaryKinds: { security: palette.red, network: palette.cyan, infra: palette.muted },
    boundaryFallback: palette.violet,
    lifecycle: {
      frozen: palette.green,
      staged: palette.cyan,
      open: palette.blue,
      accepted: palette.cyan,
      done: palette.muted,
      superseded: palette.subtle,
    },
    lifecycleFallback: "#6b7280",
    districtTints: { sector: "#22252a", hq: "#1e2126", governance: "#15171b" },
    selection: { color: palette.blue },
  },
  materials,
  archetypes: industrialArchetypes,
  environment: industrialEnvironment,
  lighting: {
    // 70–85 % dark. A dim cold moon key for silhouettes and shadows; the rest is
    // floodlights, windows and lamps that the world brings with it.
    hemisphere: { sky: "#3f4c60", ground: "#05060a", intensity: 2.0 },
    key: { color: "#b9c6dd", intensity: 2.6, shadows: true },
    rim: null,
    floodlight: { sector: 1600, hq: 1400, governance: 0 },
    exposure: 1.1,
    toneMapping: THREE.ACESFilmicToneMapping,
    // Fog thins with the base so the far edge stays readable from home (~25 % at home distance).
    fog: (homeDist) => ({ color: BG, density: THREE.MathUtils.clamp(0.6 / homeDist, 0.0005, 0.0038) }),
    bloom: { strength: 0.28, radius: 0.4, threshold: 0.95 },
  },
  roads: {
    shoulder: materials.road,
    trench: materials.trench,
    rails: materials.steel,
    markerLamp: { color: palette.muted, intensity: 0.6 },
    channel: { radius: 0.08, bodyScale: 0.12, emissive: 0.16, activeBoost: 4.0 },
    packet: { radius: 0.15, length: 1.4, glow: 2.2 },
  },
  labels: {
    fonts: { sans: "Inter, system-ui, -apple-system, 'Segoe UI', sans-serif", mono: "ui-monospace, Menlo, monospace" },
    tag: { fill: "rgba(8,9,11,0.82)", stroke: "#3a4149", title: "#e4e9f2", meta: "#8a95a3", status: "#c9d0da" },
    stencil: "#4b5158",
    sizes: { title: 30, meta: 19, pad: 12, scale: 18 },
  },
  chrome: {
    "--bg": BG,
    "--panel": "#0a0b0d",
    "--panel-2": "#15181d",
    "--panel-glass": "rgba(10, 11, 13, 0.94)",
    "--line": "#2a2f36",
    "--line-2": "#3a4149",
    "--line-soft": "#1f2429",
    "--text": "#dfe4ea",
    "--muted": "#8a95a3",
    "--subtle": "#5a626d",
    "--blue": "#3c8dff",
    "--blue-soft": "rgba(60, 141, 255, 0.08)",
    "--cyan": "#26c6da",
    "--cyan-soft": "rgba(38, 198, 218, 0.4)",
    "--amber": "#e9a83e",
    "--red": "#e04747",
    "--sil": "#1c2333",
    "--on-accent": BG,
  },
};
