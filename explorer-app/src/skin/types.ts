import type * as THREE from "three";
import type { C3Node, C3Payload, District } from "../types";

/* The Skin is the whole visual layer of the city behind one object: colours, materials,
 * archetype builders, ground and district dressing, lighting, road styling, label styling
 * and the CSS chrome. The scene (CityScene / CityWorld / InfrastructureNode / roads /
 * traffic) reads everything visual from the active skin and owns nothing visual itself,
 * so a new look is a new Skin, never a scene change. See explorer-app/SKINS.md. */

export type Motion = "none" | "breathe" | "pulse" | "pulse-slow";

export interface KindStyle {
  color: string;
  label: string;
}

export interface StatusDef {
  text: string;
  color: string;
  motion: Motion;
}

/** A resolved status: the payload's key plus the skin's text / colour / motion for it. */
export interface StatusStyle extends StatusDef {
  key: string;
}

/** Semantic palette. `blue` is the accent, `red` the alarm, `amber` the warning, `cyan` the
 * "changing" hue, `violet` the flow hue; `muted`/`subtle`/`text`/`line` are the greys. */
export interface Palette {
  blue: string;
  violet: string;
  amber: string;
  red: string;
  cyan: string;
  green: string;
  muted: string;
  subtle: string;
  text: string;
  line: string;
}

export interface SkinTokens {
  background: string;
  palette: Palette;
  /** Edge kinds are open strings; unknown kinds resolve to `palette.subtle` + the kind name. */
  kinds: Record<string, KindStyle>;
  /** Keyed by payload statusKey; `unchecked` is the fallback and must exist. */
  status: Record<string, StatusDef>;
  boundaryKinds: Record<string, string>;
  boundaryFallback: string;
  lifecycle: Record<string, string>;
  lifecycleFallback: string;
  /** Keyed by district kind (sector / hq / governance). */
  districtTints: Record<string, string>;
  selection: { color: string };
}

/** Named material slots every builder and environment hook draws from. Builders never import
 * a concrete material table; they read `ctx.m.<slot>`, so a skin re-dresses them by supplying
 * its own set. */
export interface MaterialSet {
  concrete: THREE.Material;
  concrete2: THREE.Material;
  graphite: THREE.Material;
  gunmetal: THREE.Material;
  steel: THREE.Material;
  darkMetal: THREE.Material;
  metal: THREE.Material;
  hazard: THREE.Material;
  glass: THREE.Material;
  road: THREE.Material;
  trench: THREE.Material;
  ground: THREE.Material;
}

export interface Spinner {
  obj: THREE.Object3D;
  axis: "x" | "y" | "z";
  speed: number;
}

/** What a build receives: the skin, its materials, and the collectors for semantic motion
 * and lights. `labels` is false when there is no 2D canvas (tests). */
export interface SkinContext {
  skin: Skin;
  m: MaterialSet;
  palette: Palette;
  spinners: Spinner[];
  lights: THREE.Light[];
  labels: boolean;
}

export interface Built {
  group: THREE.Group;
  roofY: number;
  /** Where the status lamp pole stands, in the group's local space. */
  lightAt: [number, number, number];
  core?: THREE.Mesh;
}

/** `acc` is the per-node accent material (breathes / dims); `lamp` the blinking obstruction lamp material. */
export type ArchetypeBuilder = (node: C3Node, acc: THREE.Material, lamp: THREE.Material, ctx: SkinContext) => Built;

export interface Extent {
  minX: number;
  maxX: number;
  minZ: number;
  maxZ: number;
  cx: number;
  cz: number;
  size: number;
}

/** The environment hooks fill groups the scene owns (named + tagged for export and the
 * inspector tree). Lights go into `ctx.lights` and get `userData.noExport`. */
export interface EnvironmentHooks {
  ground(ctx: SkinContext, g: THREE.Group, ext: Extent): void;
  district(ctx: SkinContext, g: THREE.Group, d: District): void;
  props(ctx: SkinContext, g: THREE.Group, payload: C3Payload): void;
}

export interface Lighting {
  hemisphere: { sky: string; ground: string; intensity: number };
  key: { color: string; intensity: number; shadows: boolean };
  rim: { color: string; intensity: number } | null;
  /** Floodlight intensity (candela) per district kind; the environment hook may read it. */
  floodlight: Record<string, number>;
  exposure: number;
  toneMapping: THREE.ToneMapping;
  /** Fog for a base whose home camera distance is `homeDist`; null for none. */
  fog(homeDist: number): { color: string; density: number } | null;
  bloom: { strength: number; radius: number; threshold: number } | null;
}

export interface RoadStyle {
  /** Ribbon under each street / avenue; null draws none. */
  shoulder: THREE.Material | null;
  trench: THREE.Material | null;
  rails: THREE.Material | null;
  markerLamp: { color: string; intensity: number } | null;
  /** Route channel tube: radius, diffuse multiplier of the kind colour, idle emissive, boost when lit. */
  channel: { radius: number; bodyScale: number; emissive: number; activeBoost: number };
  packet: { radius: number; length: number; glow: number };
}

export interface LabelStyle {
  fonts: { sans: string; mono: string };
  tag: { fill: string; stroke: string; title: string; meta: string; status: string };
  /** Flat ground text (district titles, zone names). */
  stencil: string;
  sizes: { title: number; meta: number; pad: number; scale: number };
}

export interface Skin {
  id: string;
  label: string;
  tokens: SkinTokens;
  materials: MaterialSet;
  /** Materials the skin shares across nodes beyond the named slots (outline inks, decals).
   * Like the slots they survive world rebuilds and are released when the skin is switched away. */
  extraMaterials?: THREE.Material[];
  archetypes: Record<string, ArchetypeBuilder>;
  /** Optional pass over every built node (ink edges, outlines, decals). */
  decorateNode?(built: Built, node: C3Node, ctx: SkinContext): void;
  environment: EnvironmentHooks;
  lighting: Lighting;
  roads: RoadStyle;
  labels: LabelStyle;
  /** CSS custom properties applied to `:root` (`--bg`, `--panel`, …). */
  chrome: Record<string, string>;
}
