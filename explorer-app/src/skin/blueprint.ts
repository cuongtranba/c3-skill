import * as THREE from "three";
import type { MaterialSet, Palette, Skin } from "./types";
import { industrialArchetypes } from "./archetypes/industrial";
import { inkEdges } from "./kit";
import { groundText } from "../scene/label";

/* Engineering blueprint: pale paper, matte white-blue volumes, every edge inked, roads as
 * drafted lines, no bloom, flat daylight. It reuses the industrial silhouettes — the
 * builders only ever read materials from the context — and re-dresses them. */

const BG = "#e4ebf3";
const INK = "#1d3557";
const SANS = "Inter, system-ui, -apple-system, 'Segoe UI', sans-serif";
const MONO = "ui-monospace, Menlo, monospace";

const palette: Palette = {
  blue: "#1f5fbf",
  violet: "#5b3fc4",
  amber: "#b3731a",
  red: "#c03030",
  cyan: "#0f8a9e",
  green: "#2f8f56",
  muted: "#55677a",
  subtle: "#7c8a99",
  text: "#1c2733",
  line: "#9fb0c2",
};

const matte = (color: string): THREE.MeshStandardMaterial => new THREE.MeshStandardMaterial({ color, roughness: 1, metalness: 0 });

const materials: MaterialSet = {
  concrete: matte("#eef2f6"),
  concrete2: matte("#e3e9f0"),
  graphite: matte("#d5dee8"),
  gunmetal: matte("#f6f8fb"),
  steel: matte("#c9d5e1"),
  darkMetal: matte("#aebdcd"),
  metal: matte("#dbe3ec"),
  hazard: matte(INK),
  glass: new THREE.MeshStandardMaterial({ color: "#bcd3ea", roughness: 0.6, metalness: 0, transparent: true, opacity: 0.45 }),
  road: matte("#d9e2ec"),
  trench: matte("#cfd9e4"),
  ground: matte(BG),
};

const inkLine = new THREE.LineBasicMaterial({ color: INK, transparent: true, opacity: 0.55 });
const draftLine = new THREE.LineBasicMaterial({ color: INK, transparent: true, opacity: 0.16 });
const hatchLine = new THREE.LineBasicMaterial({ color: INK, transparent: true, opacity: 0.1 });
const inkStrip = new THREE.MeshBasicMaterial({ color: INK, toneMapped: false });

/** Drafted rectangle on the ground plane. */
function rect(x: number, z: number, w: number, d: number, y: number, mat: THREE.LineBasicMaterial): THREE.LineLoop {
  const pts = [
    new THREE.Vector3(x - w / 2, y, z - d / 2),
    new THREE.Vector3(x + w / 2, y, z - d / 2),
    new THREE.Vector3(x + w / 2, y, z + d / 2),
    new THREE.Vector3(x - w / 2, y, z + d / 2),
  ];
  const loop = new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(pts), mat);
  loop.userData.noExport = true;
  return loop;
}

export const blueprint: Skin = {
  id: "blueprint",
  label: "Blueprint",
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
    lifecycleFallback: palette.subtle,
    districtTints: { sector: "#dde6ef", hq: "#d6e0eb", governance: "#e9eff5" },
    selection: { color: INK },
  },
  materials,
  extraMaterials: [inkLine, draftLine, hatchLine, inkStrip],
  archetypes: industrialArchetypes,
  // Ink every volume: thin dark edges follow each mesh so the silhouettes read on pale paper.
  decorateNode(built) {
    const meshes: THREE.Mesh[] = [];
    built.group.traverse((o) => {
      const m = o as THREE.Mesh;
      if (m.isMesh && !m.userData.noExport && !m.userData.blink && m.geometry.type !== "PlaneGeometry") meshes.push(m);
    });
    for (const m of meshes) m.add(inkEdges(m, inkLine));
  },
  environment: {
    ground(ctx, g, ext) {
      const span = ext.size * 2.4 + 300;
      const ground = new THREE.Mesh(new THREE.PlaneGeometry(span, span), ctx.m.ground);
      ground.rotation.x = -Math.PI / 2;
      ground.position.set(ext.cx, -0.02, ext.cz);
      ground.receiveShadow = true;
      ground.name = "ground";
      g.add(ground);
      // Drafting grid: a fine 10-unit mesh under a bolder 50-unit one.
      const fine = new THREE.GridHelper(span, Math.round(span / 10), INK, INK);
      fine.material.dispose();
      fine.material = hatchLine;
      fine.position.set(ext.cx, 0, ext.cz);
      fine.userData.noExport = true;
      const bold = new THREE.GridHelper(span, Math.round(span / 50), INK, INK);
      bold.material.dispose();
      bold.material = draftLine;
      bold.position.set(ext.cx, 0.005, ext.cz);
      bold.userData.noExport = true;
      g.add(fine, bold);
    },
    district(ctx, g, d) {
      const tint = ctx.skin.tokens.districtTints[d.kind] ?? ctx.skin.tokens.districtTints.sector;
      if (d.y > 0) {
        const plat = new THREE.Mesh(new THREE.BoxGeometry(d.w, d.y, d.d), matte(tint));
        plat.position.set(d.x, d.y / 2, d.z);
        plat.receiveShadow = true;
        plat.name = `${d.id}:platform`;
        plat.add(inkEdges(plat, inkLine));
        g.add(plat);
      } else {
        const apron = new THREE.Mesh(new THREE.PlaneGeometry(d.w, d.d), matte(tint));
        apron.rotation.x = -Math.PI / 2;
        apron.position.set(d.x, 0.01, d.z);
        apron.receiveShadow = true;
        g.add(apron);
      }
      // Dimension-style double outline around the footprint.
      g.add(rect(d.x, d.z, d.w, d.d, d.y + 0.03, inkLine));
      g.add(rect(d.x, d.z, d.w + 2.4, d.d + 2.4, 0.03, draftLine));
      if (ctx.labels) {
        const lw = Math.min(26 + d.title.length * 0.4, d.w * 0.8);
        const lbl = groundText(ctx.skin.labels, d.title, lw);
        lbl.position.set(d.x - d.w / 2 + lw / 2 + 1.5, d.y + 0.04, d.z + d.d / 2 - 3.2);
        g.add(lbl);
      }
    },
    // Blueprints carry no scenery: only what the model names is drawn.
    props() {},
  },
  lighting: {
    hemisphere: { sky: "#ffffff", ground: "#c9d4df", intensity: 2.6 },
    key: { color: "#ffffff", intensity: 1.1, shadows: false },
    rim: { color: "#dfe8f2", intensity: 0.6 },
    floodlight: {},
    exposure: 1.0,
    toneMapping: THREE.NoToneMapping,
    fog: (homeDist) => ({ color: BG, density: THREE.MathUtils.clamp(0.25 / homeDist, 0.0002, 0.0015) }),
    bloom: null,
  },
  roads: {
    shoulder: null,
    trench: materials.trench,
    rails: inkStrip,
    markerLamp: null,
    channel: { radius: 0.1, bodyScale: 1.0, emissive: 0.0, activeBoost: 1.0 },
    packet: { radius: 0.18, length: 1.2, glow: 1.0 },
  },
  labels: {
    fonts: { sans: SANS, mono: MONO },
    tag: { fill: "rgba(250,252,254,0.92)", stroke: INK, title: palette.text, meta: palette.muted, status: INK },
    stencil: "#5c6f85",
    sizes: { title: 30, meta: 19, pad: 12, scale: 18 },
  },
  chrome: {
    "--bg": BG,
    "--panel": "#f7f9fb",
    "--panel-2": "#e6edf4",
    "--panel-glass": "rgba(247, 249, 251, 0.94)",
    "--line": "#b9c6d4",
    "--line-2": "#94a5b8",
    "--line-soft": "#d3dce6",
    "--text": palette.text,
    "--muted": palette.muted,
    "--subtle": palette.subtle,
    "--blue": palette.blue,
    "--blue-soft": "rgba(31, 95, 191, 0.1)",
    "--cyan": palette.cyan,
    "--cyan-soft": "rgba(15, 138, 158, 0.4)",
    "--amber": palette.amber,
    "--red": palette.red,
    "--sil": "#c5d3e2",
    "--on-accent": "#ffffff",
  },
};
