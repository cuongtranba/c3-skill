import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { EffectComposer } from "three/addons/postprocessing/EffectComposer.js";
import { RenderPass } from "three/addons/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/addons/postprocessing/UnrealBloomPass.js";
import { OutputPass } from "three/addons/postprocessing/OutputPass.js";
import type { C3Edge, C3Event, C3Node, C3Payload } from "../types";
import { lifecycleOf, normalizePayload, type Level } from "../data";
import { ARCH_LABEL } from "./constants";
import { type Skin, applyChrome, kindStyle, persistSkinId, releaseSkinMaterials, resolveSkin, statusOf } from "../skin";
import { CityWorld } from "./CityWorld";
import type { InfrastructureNode } from "./InfrastructureNode";
import type { RouteObject } from "./DataRoadNetwork";
import type { MotionMode } from "./TrafficSystem";

/** Edge kinds the Go layout never routes (docs/specs/2026-09-15-c3v-visualize-payload-v2.md). */
const UNROUTED_KINDS = new Set(["contains", "encloses", "flow_from", "flow_to"]);
import { applyFacets, flowRouteIds, isEmptyFacets, type Facets } from "./facets";
import { sceneToJSON, type SceneJSON } from "./exportScene";
import { buildSceneTree, type SceneTreeNode } from "./sceneTree";
import { neverCreated, timelineVisibleSet } from "./timeline";
import { diffPayload } from "./liveDiff";

export interface EdgeRef {
  edgeId: string;
  id: string;
  title: string;
  kind: string;
  kindLabel: string;
  color: string;
  label?: string;
}

export interface ZoneRef {
  id: string;
  kind: string;
  title: string;
}

export interface NodeSelection {
  kind: "node";
  id: string;
  title: string;
  type: string;
  lifecycle: string;
  statusKey: string;
  statusText: string;
  statusColor: string;
  archetype: string;
  archetypeLabel: string;
  goal?: string;
  parent?: string;
  parentTitle?: string;
  district: string;
  districtTitle: string;
  tech?: string;
  category?: string;
  boundaryKind?: string;
  legacyBoundary?: string;
  zones: ZoneRef[];
  governs: EdgeRef[];
  code?: { globs: string[]; files: number; loc: number } | null;
  eval?: { verdict: string } | null;
  staged?: boolean;
  stagedBy?: string[];
  transition?: { from: string; to: string; by: string } | null;
  members?: string[];
  flowSteps?: { seq: number; from: string; to: string; action?: string }[];
  inbound: EdgeRef[];
  outbound: EdgeRef[];
  /** Kept for chrome that predates the city: contains/uses/usedBy lists. */
  contains: EdgeRef[];
  uses: EdgeRef[];
  usedBy: EdgeRef[];
}

export interface RouteSelection {
  kind: "route";
  id: string;
  edgeKind: string;
  kindLabel: string;
  color: string;
  label?: string;
  fromId: string;
  fromTitle: string;
  toId: string;
  toTitle: string;
  active: boolean;
  segments: string[];
  sharedWith?: string;
  flow?: string;
  seq?: number;
}

export type Selection = NodeSelection | RouteSelection | null;

export interface TooltipInfo {
  x: number;
  y: number;
  text: string;
  sub?: string;
}

export interface TimelineSnap {
  available: boolean;
  active: boolean;
  index: number;
  playing: boolean;
  speed: number;
  eventCount: number;
}

export interface LastUpdate {
  ts: number;
  added: number;
  removed: number;
  changed: number;
}

export interface Snapshot {
  ready: boolean;
  level: Level;
  query: string;
  facets: Facets;
  motion: MotionMode;
  bloom: boolean;
  /** Active skin id; the chrome re-reads colours from `scene.getSkin()` when it changes. */
  skin: string;
  reducedMotion: boolean;
  selection: Selection;
  tooltip: TooltipInfo | null;
  timeline: TimelineSnap;
  lastUpdate: LastUpdate | null;
  visibleCount: number;
  nodeCount: number;
}

interface CamGoal {
  target: THREE.Vector3;
  pos: THREE.Vector3;
}

const MOVE_KEYS = new Set(["w", "a", "s", "d", "arrowup", "arrowdown", "arrowleft", "arrowright"]);
const LEVELS = [1, 0.9, 0.55, 0.3];

/* The renderer: camera, lights, post, picking, selection, travel, live diff and timeline
 * visibility. It never lays anything out — the payload already did — and it owns no look:
 * everything visual comes from the active Skin, which can be swapped in place. */
export class CityScene {
  private data: C3Payload;
  private skin: Skin;
  private readonly canvas: HTMLCanvasElement;
  private readonly reduced: boolean;

  private renderer!: THREE.WebGLRenderer;
  private scene!: THREE.Scene;
  private camera!: THREE.PerspectiveCamera;
  private controls!: OrbitControls;
  private composer!: EffectComposer;
  private bloomPass!: UnrealBloomPass;
  private readonly lightRig = new THREE.Group();
  private key!: THREE.DirectionalLight;
  private world!: CityWorld;
  private home!: CamGoal;
  private lod = { far: 260, near: 110 };
  private camGoal: CamGoal | null = null;

  private readonly raycaster = new THREE.Raycaster();
  private readonly pointer = new THREE.Vector2(-2, -2);
  private readonly timer = new THREE.Timer();
  private t = 0;
  private raf: number | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private readonly keys = new Set<string>();
  private downX = 0;
  private downY = 0;

  private level: Level = "all";
  private facets: Facets = {};
  private motion: MotionMode = "active";
  private bloomOn = true;
  private selectedId: string | null = null;
  private selection: Selection = null;
  private hoverId: string | null = null;
  private hoverRoute: RouteObject | null = null;
  private tooltip: TooltipInfo | null = null;
  private lastUpdate: LastUpdate | null = null;
  private highlightedRoutes: Set<string> | null = null;

  private tlActive = false;
  private tlIndex = 0;
  private tlPlaying = false;
  private tlTimer: ReturnType<typeof setTimeout> | null = null;
  private tlSpeed = 1;
  private tlVisibleSet: Set<string> | null = null;

  ready = false;
  private readonly listeners = new Set<() => void>();
  private snap: Snapshot;

  constructor(canvas: HTMLCanvasElement, data: C3Payload, skin: Skin = resolveSkin(undefined)) {
    this.canvas = canvas;
    this.skin = skin;
    this.data = normalizePayload(data);
    this.reduced = typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches;
    this.snap = this.buildSnapshot();
    this.initScene();
  }

  /* ─── external store contract ─────────────────────────────────── */
  subscribe = (fn: () => void): (() => void) => {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  };

  getSnapshot = (): Snapshot => this.snap;

  private emit(): void {
    this.snap = this.buildSnapshot();
    this.listeners.forEach((fn) => fn());
  }

  private buildSnapshot(): Snapshot {
    const nodes = this.world ? [...this.world.nodes.values()] : [];
    return {
      ready: this.ready,
      level: this.level,
      query: this.facets.text ?? "",
      facets: { ...this.facets },
      motion: this.motion,
      bloom: this.bloomOn,
      skin: this.skin.id,
      reducedMotion: this.reduced,
      selection: this.selection,
      tooltip: this.tooltip,
      timeline: {
        available: this.events().length > 0,
        active: this.tlActive,
        index: this.tlIndex,
        playing: this.tlPlaying,
        speed: this.tlSpeed,
        eventCount: this.events().length,
      },
      lastUpdate: this.lastUpdate,
      visibleCount: nodes.filter((n) => !n.hidden).length,
      nodeCount: nodes.length,
    };
  }

  /* ─── verification surface ────────────────────────────────────── */
  renderedNodeIds(): string[] {
    return [...this.world.nodes.keys()];
  }
  allDataNodeIds(): string[] {
    return this.data.nodes.map((n) => n.id);
  }
  /**
   * Routes drawn plus the edges the city realises without a cable — `contains`
   * as district membership, `encloses` as the zone marking, `flow_from`/`flow_to`
   * as the flow overlay — counted only when both endpoints were built.
   */
  /** Last-frame renderer counters — a draw-call diagnostic for dense bases. */
  renderStats(): { calls: number; triangles: number; geometries: number; textures: number } {
    const r = this.renderer.info;
    return { calls: r.render.calls, triangles: r.render.triangles, geometries: r.memory.geometries, textures: r.memory.textures };
  }

  renderedEdgeCount(): number {
    const uncabled = this.data.edges.filter((e) => UNROUTED_KINDS.has(e.kind) && this.world.nodes.has(e.from) && this.world.nodes.has(e.to)).length;
    return this.world.roads.routes.length + uncabled;
  }
  dataEdgeCount(): number {
    return this.data.edges.length;
  }
  nodesWithoutStatus(): string[] {
    const out: string[] = [];
    for (const b of this.world.nodes.values()) {
      let has = false;
      b.group.traverse((o) => {
        if (o.userData.statusLamp) has = true;
      });
      if (!has) out.push(b.id);
    }
    return out;
  }
  currentSelection(): { id: string; lifecycle: string } | null {
    if (!this.selectedId) return null;
    const n = this.nodeData(this.selectedId);
    return n ? { id: n.id, lifecycle: lifecycleOf(n) } : null;
  }
  selectNodeById(id: string): boolean {
    if (!this.world.nodes.has(id)) return false;
    this.select(id);
    return true;
  }
  focus(id: string): boolean {
    if (!this.world.nodes.has(id)) return false;
    this.select(id);
    this.travelTo(id);
    return true;
  }
  visibleNodeIds(): string[] {
    return [...this.world.nodes.values()].filter((n) => !n.hidden).map((n) => n.id);
  }
  events(): C3Event[] {
    return this.data.events;
  }
  getData(): C3Payload {
    return this.data;
  }
  cameraPosition(): { x: number; y: number; z: number } {
    const p = this.camera.position;
    return { x: p.x, y: p.y, z: p.z };
  }
  exportSceneJSON(): SceneJSON {
    return sceneToJSON(this.world.group);
  }
  inspectorTree(): SceneTreeNode {
    return buildSceneTree(this.world.group);
  }
  getMotion(): MotionMode {
    return this.motion;
  }
  getSkin(): Skin {
    return this.skin;
  }
  skinId(): string {
    return this.skin.id;
  }

  /** Swaps the look in place: lighting, post, environment, buildings, roads. Positions come
   * from the payload so nothing moves; camera, selection, facets and timeline persist. */
  setSkin(id: string): boolean {
    const next = resolveSkin(id);
    if (next.id !== id) return false;
    if (next === this.skin) return true;
    const prev = this.skin;
    this.skin = next;
    applyChrome(next);
    persistSkinId(next.id);
    this.applyLighting();
    this.buildWorld();
    releaseSkinMaterials(prev);
    this.emit();
    return true;
  }

  /* ─── scene setup ─────────────────────────────────────────────── */
  /** The renderer pins the canvas to pixel sizes, so measure the parent, never the canvas itself. */
  private viewportSize(): [number, number] {
    const host = this.canvas.parentElement;
    return [host?.clientWidth || window.innerWidth, host?.clientHeight || window.innerHeight];
  }

  private initScene(): void {
    const canvas = this.canvas;
    const [w, h] = this.viewportSize();

    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: "high-performance" });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.75));
    this.renderer.setSize(w, h);
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.VSMShadowMap;

    this.scene = new THREE.Scene();
    this.scene.add(this.lightRig);

    this.camera = new THREE.PerspectiveCamera(36, w / h, 0.5, 900);
    this.controls = new OrbitControls(this.camera, canvas);
    this.controls.enableDamping = true;
    this.controls.dampingFactor = 0.07;
    // Tactical RTS view: ~48° down, azimuth constrained so the base always reads the same way.
    this.controls.minPolarAngle = 0.45;
    this.controls.maxPolarAngle = 0.95;
    this.controls.minAzimuthAngle = -0.8;
    this.controls.maxAzimuthAngle = 0.8;
    this.controls.minDistance = 22;
    this.controls.screenSpacePanning = false;
    this.controls.mouseButtons = { LEFT: THREE.MOUSE.ROTATE, MIDDLE: THREE.MOUSE.DOLLY, RIGHT: THREE.MOUSE.PAN };
    this.controls.addEventListener("start", () => (this.camGoal = null));

    this.composer = new EffectComposer(this.renderer);
    this.composer.addPass(new RenderPass(this.scene, this.camera));
    this.bloomPass = new UnrealBloomPass(new THREE.Vector2(w, h), 0.28, 0.4, 0.95);
    this.composer.addPass(this.bloomPass);
    this.composer.addPass(new OutputPass());
    this.applyLighting();

    canvas.addEventListener("pointermove", this.onPointerMove);
    canvas.addEventListener("pointerdown", this.onPointerDown);
    canvas.addEventListener("pointerup", this.onPointerUp);
    canvas.addEventListener("dblclick", this.onDblClick);
    canvas.addEventListener("pointerleave", this.onPointerLeave);
    window.addEventListener("resize", this.onWindowResize);
    window.addEventListener("keydown", this.onKeyDown);
    window.addEventListener("keyup", this.onKeyUp);
    window.addEventListener("blur", this.onWindowBlur);
    this.resizeObserver = new ResizeObserver(() => this.onWindowResize());
    this.resizeObserver.observe(canvas.parentElement || document.body);

    this.buildWorld();
    this.camera.position.copy(this.home.pos);
    this.controls.target.copy(this.home.target);
    this.ready = true;
    this.emit();
    this.loop();
  }

  /** Sky, key/rim lights, tone mapping, background and bloom from the active skin; the
   * world's own lights (floodlights) come with the world. Idempotent, so a skin switch reruns it. */
  private applyLighting(): void {
    const L = this.skin.lighting;
    this.lightRig.traverse((o) => (o as THREE.Light).isLight && (o as THREE.Light).dispose());
    this.lightRig.clear();
    this.lightRig.add(new THREE.HemisphereLight(L.hemisphere.sky, L.hemisphere.ground, L.hemisphere.intensity));
    this.key = new THREE.DirectionalLight(L.key.color, L.key.intensity);
    this.key.castShadow = L.key.shadows;
    this.key.shadow.mapSize.set(2048, 2048);
    this.key.shadow.radius = 4;
    this.key.shadow.blurSamples = 10;
    this.key.shadow.bias = -0.0004;
    this.lightRig.add(this.key, this.key.target);
    if (L.rim) {
      const rim = new THREE.DirectionalLight(L.rim.color, L.rim.intensity);
      rim.position.set(-80, 60, -90);
      this.lightRig.add(rim);
    }
    this.renderer.toneMapping = L.toneMapping;
    this.renderer.toneMappingExposure = L.exposure;
    this.scene.background = new THREE.Color(this.skin.tokens.background);
    if (L.bloom) {
      this.bloomPass.strength = L.bloom.strength;
      this.bloomPass.radius = L.bloom.radius;
      this.bloomPass.threshold = L.bloom.threshold;
    }
    this.bloomPass.enabled = this.bloomOn && !!L.bloom;
  }

  private buildWorld(): void {
    if (this.world) {
      this.scene.remove(this.world.group);
      this.world.dispose();
    }
    this.world = new CityWorld(this.data, { labels: true, skin: this.skin });
    this.scene.add(this.world.group);
    this.world.traffic.mode = this.motion;

    const ext = this.world.env.extent;
    const size = ext.size;
    // Home: the mockup's tactical direction (~48° down), pulled back until the projected
    // base fits the viewport — depth foreshortened by the tilt, width by the aspect.
    const dir = new THREE.Vector3(0.08, 0.75, 0.65).normalize();
    const halfTan = Math.tan(THREE.MathUtils.degToRad(this.camera.fov / 2));
    const depth = (ext.maxZ - ext.minZ + 40) * Math.sin(Math.asin(dir.y)) + 30;
    const width = ext.maxX - ext.minX + 40;
    const homeDist = Math.max(120, depth / (2 * halfTan), width / (2 * halfTan * Math.max(this.camera.aspect, 0.8)));
    const target = new THREE.Vector3(ext.cx, 0, ext.cz + size * 0.02);
    const pos = target.clone().add(dir.multiplyScalar(homeDist));
    this.home = { target, pos };
    this.lod = { far: Math.max(260, homeDist * 1.15), near: Math.max(110, homeDist * 0.5) };
    const fog = this.skin.lighting.fog(homeDist);
    this.scene.fog = fog ? new THREE.FogExp2(fog.color, fog.density) : null;
    this.camera.far = Math.max(900, homeDist * 4);
    this.camera.updateProjectionMatrix();
    this.controls.maxDistance = Math.max(280, homeDist * 1.4);

    const reach = size * 0.75 + 60;
    this.key.position.set(ext.cx + 70, 120 + size * 0.2, ext.cz + 30);
    this.key.target.position.set(ext.cx, 0, ext.cz);
    Object.assign(this.key.shadow.camera, { left: -reach, right: reach, top: reach, bottom: -reach, near: 10, far: 320 + size });
    this.key.shadow.camera.updateProjectionMatrix();

    this.applyVisibility();
    this.applyNeighbourhood();
    if (this.selectedId) {
      if (this.world.nodes.has(this.selectedId)) {
        this.world.nodes.get(this.selectedId)?.setSelected(true);
        this.selection = this.nodeSelection(this.selectedId);
      } else {
        this.selectedId = null;
        this.selection = null;
      }
    }
  }

  private onWindowResize = (): void => {
    const [nw, nh] = this.viewportSize();
    if (!nw || !nh) return;
    this.camera.aspect = nw / nh;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(nw, nh);
    this.composer.setSize(nw, nh);
  };

  dispose(): void {
    if (this.raf !== null) cancelAnimationFrame(this.raf);
    if (this.tlTimer !== null) clearTimeout(this.tlTimer);
    const c = this.canvas;
    c.removeEventListener("pointermove", this.onPointerMove);
    c.removeEventListener("pointerdown", this.onPointerDown);
    c.removeEventListener("pointerup", this.onPointerUp);
    c.removeEventListener("dblclick", this.onDblClick);
    c.removeEventListener("pointerleave", this.onPointerLeave);
    window.removeEventListener("resize", this.onWindowResize);
    window.removeEventListener("keydown", this.onKeyDown);
    window.removeEventListener("keyup", this.onKeyUp);
    window.removeEventListener("blur", this.onWindowBlur);
    this.resizeObserver?.disconnect();
    this.controls.dispose();
    this.world.dispose();
    this.composer.dispose();
    this.renderer.dispose();
  }

  /* ─── lookups ─────────────────────────────────────────────────── */
  private nodeData(id: string): C3Node | undefined {
    return this.world.nodes.get(id)?.node;
  }

  private titleOf(id: string): string {
    return this.nodeData(id)?.title || id;
  }

  private edgeRef(e: C3Edge, otherId: string): EdgeRef {
    const ks = kindStyle(this.skin, e.kind);
    return { edgeId: e.id, id: otherId, title: this.titleOf(otherId), kind: e.kind, kindLabel: ks.label, color: ks.color, label: e.label };
  }

  private nodeSelection(id: string): NodeSelection {
    const n = this.nodeData(id) as C3Node;
    const st = statusOf(this.skin, n);
    const edges = this.data.edges;
    const ins = edges.filter((e) => e.to === id && e.kind !== "contains").map((e) => this.edgeRef(e, e.from));
    const outs = edges.filter((e) => e.from === id && e.kind !== "contains").map((e) => this.edgeRef(e, e.to));
    const isGovernance = (oid: string): boolean => {
      const t = this.nodeData(oid)?.type;
      return t === "ref" || t === "rule";
    };
    const governs = outs.filter((r) => isGovernance(r.id));
    const district = this.data.districts.find((d) => d.id === n.district);
    const zones: ZoneRef[] = (n.boundaries ?? []).map((bid) => {
      const zn = this.nodeData(bid);
      const b = this.data.boundaries.find((x) => x.id === bid);
      return { id: bid, kind: zn?.boundaryKind || b?.kind || "", title: zn?.title || bid };
    });
    const boundary = this.data.boundaries.find((b) => b.id === id);
    const flow = this.data.flows.find((f) => f.id === id);
    return {
      kind: "node",
      id,
      title: n.title || id,
      type: n.type,
      lifecycle: lifecycleOf(n),
      statusKey: n.statusKey,
      statusText: st.text,
      statusColor: st.color,
      archetype: n.archetype,
      archetypeLabel: ARCH_LABEL[n.archetype] ?? n.archetype,
      goal: n.goal,
      parent: n.parent || undefined,
      parentTitle: n.parent ? this.titleOf(n.parent) : undefined,
      district: n.district,
      districtTitle: district?.title ?? n.district,
      tech: n.tech || undefined,
      category: n.category || undefined,
      boundaryKind: n.boundaryKind || undefined,
      legacyBoundary: n.legacyBoundary || undefined,
      zones,
      governs,
      code: n.code ?? null,
      eval: n.eval ?? null,
      staged: n.staged,
      stagedBy: n.stagedBy,
      transition: n.transition ?? null,
      members: boundary?.members,
      flowSteps: flow?.steps,
      inbound: ins,
      outbound: outs,
      contains: edges.filter((e) => e.kind === "contains" && e.from === id).map((e) => this.edgeRef(e, e.to)),
      uses: outs.filter((r) => r.kind === "uses"),
      usedBy: ins.filter((r) => r.kind === "uses"),
    };
  }

  private routeSelection(o: RouteObject): RouteSelection {
    const e = o.edge;
    const r = o.route;
    const ks = kindStyle(this.skin, r.kind);
    const fromId = e?.from ?? r.edgeId.split("→")[0] ?? "";
    const toId = e?.to ?? "";
    return {
      kind: "route",
      id: r.id,
      edgeKind: r.kind,
      kindLabel: ks.label,
      color: ks.color,
      label: e?.label,
      fromId,
      fromTitle: this.titleOf(fromId),
      toId,
      toTitle: this.titleOf(toId),
      active: r.active,
      segments: r.segments,
      sharedWith: r.sharedWith || undefined,
      flow: e?.flow,
      seq: e?.seq,
    };
  }

  /* ─── visibility: level ∧ facets ∧ timeline. Hides, never re-lays out. ─── */
  private levelAllows(n: C3Node): boolean {
    switch (this.level) {
      case "context":
        return n.level === "context" || n.type === "system" || n.type === "container";
      case "container":
        return n.type === "system" || n.type === "container" || n.type === "adr";
      default:
        return true;
    }
  }

  private applyVisibility(): void {
    const facetSet = applyFacets(this.data, this.facets);
    const always = this.tlVisibleSet ? neverCreated(this.data.events, this.data.nodes) : null;
    for (const b of this.world.nodes.values()) {
      const n = b.node;
      const tlOk = !this.tlVisibleSet || this.tlVisibleSet.has(n.id) || (always?.has(n.id) ?? false);
      b.hidden = !(this.levelAllows(n) && facetSet.has(n.id) && tlOk);
      b.group.visible = !b.hidden;
    }
    for (const o of this.world.roads.routes) {
      const e = o.edge;
      const a = e ? this.world.nodes.get(e.from) : undefined;
      const z = e ? this.world.nodes.get(e.to) : undefined;
      o.hidden = !!(a?.hidden || z?.hidden);
      o.chan.visible = !o.hidden;
    }
    this.highlightedRoutes = this.facets.flow ? flowRouteIds(this.data, this.facets.flow) : null;
  }

  setLevel(lvl: Level): void {
    this.level = lvl;
    this.applyVisibility();
    this.emit();
  }

  setFacets(patch: Facets): void {
    this.facets = { ...this.facets, ...patch };
    for (const k of Object.keys(this.facets) as (keyof Facets)[]) {
      const v = this.facets[k];
      if (v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) delete this.facets[k];
    }
    this.applyVisibility();
    this.emit();
  }

  getFacets(): Facets {
    return { ...this.facets };
  }

  clearFacets(): void {
    this.facets = {};
    this.applyVisibility();
    this.emit();
  }

  toggleFacetValue(key: "type" | "district" | "boundary" | "boundaryKind" | "statusKey" | "lifecycle" | "archetype", value: string): void {
    const cur = new Set(this.facets[key] ?? []);
    if (cur.has(value)) cur.delete(value);
    else cur.add(value);
    this.setFacets({ [key]: [...cur] });
  }

  setQuery(raw: string): void {
    this.setFacets({ text: raw.trim() });
  }

  selectFirstMatch(): void {
    if (isEmptyFacets(this.facets)) return;
    const first = this.data.nodes.find((n) => !this.world.nodes.get(n.id)?.hidden);
    if (first) this.focus(first.id);
  }

  setMotion(mode: MotionMode): void {
    this.motion = mode;
    this.world.traffic.mode = mode;
    this.emit();
  }

  setBloom(on: boolean): void {
    this.bloomOn = on;
    this.bloomPass.enabled = on && !!this.skin.lighting.bloom;
    this.emit();
  }

  /* ─── selection · neighbourhood · travel ──────────────────────── */
  private applyNeighbourhood(): void {
    const nodes = this.world.nodes;
    const routes = this.world.roads.routes;
    if (!this.selectedId) {
      for (const b of nodes.values()) b.targetIntensity = 1;
      for (const o of routes) o.targetIntensity = 1;
      return;
    }
    const d = this.world.distances(this.selectedId);
    for (const b of nodes.values()) b.targetIntensity = LEVELS[Math.min(d.get(b.id) ?? 3, 3)];
    for (const o of routes) {
      const e = o.edge;
      if (!e) {
        o.targetIntensity = 0.3;
        continue;
      }
      const touches = e.from === this.selectedId || e.to === this.selectedId;
      o.targetIntensity = touches ? 1.6 : LEVELS[Math.min(Math.max(d.get(e.from) ?? 3, d.get(e.to) ?? 3), 3)] * 0.6;
    }
  }

  private select(id: string | null): void {
    if (this.selectedId) this.world.nodes.get(this.selectedId)?.setSelected(false);
    this.selectedId = id;
    if (id) {
      this.world.nodes.get(id)?.setSelected(true);
      this.selection = this.nodeSelection(id);
    } else this.selection = null;
    this.applyNeighbourhood();
    this.emit();
  }

  private selectRoute(o: RouteObject): void {
    if (this.selectedId) this.world.nodes.get(this.selectedId)?.setSelected(false);
    this.selectedId = null;
    this.selection = this.routeSelection(o);
    this.applyNeighbourhood();
    for (const r of this.world.roads.routes) r.targetIntensity = r === o ? 1.8 : 0.35;
    if (o.edge) {
      for (const b of this.world.nodes.values()) b.targetIntensity = b.id === o.edge.from || b.id === o.edge.to ? 1 : 0.45;
    }
    const mid = o.path.getPointAt(0.5);
    const dir = this.camera.position.clone().sub(this.controls.target).normalize();
    this.camGoal = { target: mid, pos: mid.clone().add(dir.multiplyScalar(Math.max(40, o.len * 0.6))) };
    this.emit();
  }

  clearSelection(): void {
    this.select(null);
  }

  travelTo(id: string): void {
    const b = this.world.nodes.get(id);
    if (!b) return;
    const target = b.position.clone().add(new THREE.Vector3(0, b.roofY * 0.45, 0));
    const dir = this.camera.position.clone().sub(this.controls.target).normalize();
    this.camGoal = { target, pos: target.clone().add(dir.multiplyScalar(Math.max(46, b.w * 3.2, b.roofY * 3.0))) };
  }

  resetView(): void {
    this.camGoal = { target: this.home.target.clone(), pos: this.home.pos.clone() };
  }

  /* ─── picking ─────────────────────────────────────────────────── */
  private ndc(e: PointerEvent | MouseEvent): void {
    const rect = this.canvas.getBoundingClientRect();
    this.pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    this.pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
  }

  private pick(e: PointerEvent | MouseEvent): { node?: InfrastructureNode; route?: RouteObject } | null {
    this.ndc(e);
    this.raycaster.setFromCamera(this.pointer, this.camera);
    const hits = this.raycaster.intersectObjects(this.world.pickMeshes, false);
    for (const h of hits) {
      const id = h.object.userData.c3Id as string | undefined;
      if (id) {
        const b = this.world.nodes.get(id);
        if (b && !b.hidden) return { node: b };
        continue;
      }
      const rid = h.object.userData.routeId as string | undefined;
      if (rid) {
        const o = this.world.roads.routes.find((r) => r.route.id === rid);
        if (o && !o.hidden) return { route: o };
      }
    }
    return null;
  }

  private onPointerMove = (e: PointerEvent): void => {
    const hit = this.pick(e);
    const nextHover = hit?.node?.id ?? null;
    const nextRoute = hit?.route ?? null;
    this.canvas.style.cursor = hit ? "pointer" : "";
    let tip: TooltipInfo | null = null;
    if (hit?.node) {
      const n = hit.node.node;
      tip = { x: e.clientX, y: e.clientY, text: n.title, sub: `${ARCH_LABEL[n.archetype] ?? n.archetype} · ${hit.node.status.text}` };
    } else if (hit?.route) {
      const r = hit.route.route;
      const ed = hit.route.edge;
      tip = {
        x: e.clientX,
        y: e.clientY,
        text: kindStyle(this.skin, r.kind).label + (ed?.label ? " · " + ed.label : ""),
        sub: `${ed?.from ?? "?"} → ${ed?.to ?? "?"} · ${r.segments.join(" · ") || r.id}`,
      };
    }
    const changed = nextHover !== this.hoverId || nextRoute !== this.hoverRoute || !!tip !== !!this.tooltip || (tip && this.tooltip && (tip.x !== this.tooltip.x || tip.y !== this.tooltip.y));
    this.hoverId = nextHover;
    this.hoverRoute = nextRoute;
    this.tooltip = tip;
    if (changed) this.emit();
  };

  private onPointerLeave = (): void => {
    if (!this.hoverId && !this.hoverRoute && !this.tooltip) return;
    this.hoverId = null;
    this.hoverRoute = null;
    this.tooltip = null;
    this.emit();
  };

  private onPointerDown = (e: PointerEvent): void => {
    this.downX = e.clientX;
    this.downY = e.clientY;
  };

  private onPointerUp = (e: PointerEvent): void => {
    if (e.button !== 0) return;
    if (Math.abs(e.clientX - this.downX) > 5 || Math.abs(e.clientY - this.downY) > 5) return;
    const hit = this.pick(e);
    if (hit?.node) this.select(hit.node.id);
    else if (hit?.route) this.selectRoute(hit.route);
    else this.select(null);
  };

  private onDblClick = (e: MouseEvent): void => {
    const hit = this.pick(e);
    if (hit?.node) this.focus(hit.node.id);
  };

  private onKeyDown = (e: KeyboardEvent): void => {
    if (e.key === "Escape") {
      this.select(null);
      return;
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const t = e.target as HTMLElement | null;
    if (t && (t.tagName === "INPUT" || t.tagName === "SELECT" || t.tagName === "TEXTAREA")) return;
    const key = e.key.toLowerCase();
    if (!MOVE_KEYS.has(key)) return;
    e.preventDefault();
    this.keys.add(key);
  };

  private onKeyUp = (e: KeyboardEvent): void => {
    this.keys.delete(e.key.toLowerCase());
  };

  private onWindowBlur = (): void => {
    this.keys.clear();
  };

  private applyKeyboardMove(): void {
    if (!this.keys.size) return;
    const fwd = new THREE.Vector3();
    this.camera.getWorldDirection(fwd);
    fwd.y = 0;
    if (fwd.lengthSq() < 1e-6) return;
    fwd.normalize();
    const right = new THREE.Vector3().crossVectors(fwd, new THREE.Vector3(0, 1, 0)).normalize();
    const move = new THREE.Vector3();
    if (this.keys.has("w") || this.keys.has("arrowup")) move.add(fwd);
    if (this.keys.has("s") || this.keys.has("arrowdown")) move.sub(fwd);
    if (this.keys.has("d") || this.keys.has("arrowright")) move.add(right);
    if (this.keys.has("a") || this.keys.has("arrowleft")) move.sub(right);
    if (!move.lengthSq()) return;
    const speed = Math.max(this.camera.position.distanceTo(this.controls.target) * 0.9, 14) * 0.016;
    move.normalize().multiplyScalar(speed);
    this.camera.position.add(move);
    this.controls.target.add(move);
    this.camGoal = null;
  }

  /* ─── frame loop ──────────────────────────────────────────────── */
  private loop = (): void => {
    this.raf = requestAnimationFrame(this.loop);
    this.timer.update();
    const dt = Math.min(this.timer.getDelta(), 0.05);
    this.t = this.timer.getElapsed();
    const reduced = this.reduced;

    if (this.camGoal) {
      this.controls.target.lerp(this.camGoal.target, 0.07);
      this.camera.position.lerp(this.camGoal.pos, 0.07);
      if (this.camera.position.distanceTo(this.camGoal.pos) < 0.05) this.camGoal = null;
    }
    this.applyKeyboardMove();

    // Semantic motion only: radars turn, fans spin, lamps blink, cores breathe. Nothing floats.
    if (!reduced) for (const sp of this.world.spinners) sp.obj.rotation[sp.axis] += dt * sp.speed;
    const blinkOn = reduced ? true : Math.sin(this.t * 1.6) > 0.85;
    const frame = { t: this.t, dt, reduced, blinkOn, hovered: false, selected: false, cameraPos: this.camera.position, lod: this.lod };
    for (const b of this.world.nodes.values()) {
      frame.hovered = this.hoverId === b.id;
      frame.selected = this.selectedId === b.id;
      b.update(frame);
    }
    this.world.traffic.update(dt, reduced, this.hoverRoute, this.highlightedRoutes);

    this.controls.update();
    // Accumulate counters across the composer's passes so renderStats() sees the whole frame.
    this.renderer.info.autoReset = false;
    this.renderer.info.reset();
    this.composer.render();
  };

  /* ─── live mode ───────────────────────────────────────────────── */
  applyLiveData(raw: C3Payload): void {
    const next = normalizePayload(raw);
    const diff = diffPayload(this.data, next);
    if (this.tlActive) this.toggleTimeline(false);
    this.data = next;
    this.buildWorld();
    for (const id of diff.added) {
      const b = this.world.nodes.get(id);
      if (b) b.bornAt = this.t;
    }
    for (const id of diff.changed) {
      const b = this.world.nodes.get(id);
      if (b) b.flashUntil = this.t + 1.6;
    }
    this.lastUpdate = { ts: Date.now(), added: diff.added.length, removed: diff.removed.length, changed: diff.changed.length };
    this.emit();
  }

  /** `action` frames pulse the touched buildings' status lamps: cyan for reads, amber for mutations. */
  pulseNodes(ids: string[], color: "mint" | "amber"): void {
    const P = this.skin.tokens.palette;
    const hex = color === "amber" ? P.amber : P.cyan;
    for (const id of ids) {
      const b = this.world.nodes.get(id);
      if (!b) continue;
      b.pulseUntil = this.t + 1.6;
      b.pulseColor = hex;
    }
  }

  /* ─── timeline ────────────────────────────────────────────────── */
  timelineActive(): boolean {
    return this.tlActive;
  }
  timelineIndex(): number {
    return this.tlIndex;
  }

  private tlFlyToCentroid(ids: string[]): void {
    const pts = ids.map((id) => this.world.nodes.get(id)?.position).filter((p): p is THREE.Vector3 => !!p);
    if (!pts.length) return;
    const tgt = pts.reduce((a, p) => a.add(p), new THREE.Vector3()).multiplyScalar(1 / pts.length);
    tgt.y = 0;
    const dir = this.camera.position.clone().sub(this.controls.target).normalize();
    const dist = Math.max(60, this.camera.position.distanceTo(this.controls.target) * 0.8);
    this.camGoal = { target: tgt, pos: tgt.clone().add(dir.multiplyScalar(dist)) };
  }

  goToEvent(idx: number): void {
    const events = this.events();
    if (!events.length) return;
    idx = Math.max(0, Math.min(idx, events.length - 1));
    const prev = this.tlVisibleSet;
    this.tlIndex = idx;
    this.tlVisibleSet = timelineVisibleSet(events, this.data.nodes, idx);
    this.applyVisibility();
    const ev = events[idx];
    for (const id of ev.creates ?? []) {
      const b = this.world.nodes.get(id);
      if (b && !(prev?.has(id) ?? false)) b.bornAt = this.t;
    }
    for (const id of ev.modifies ?? []) {
      const b = this.world.nodes.get(id);
      if (b) b.flashUntil = this.t + 1.6;
    }
    const camIds = (ev.creates ?? []).concat(ev.modifies ?? []);
    if (camIds.length) this.tlFlyToCentroid(camIds);
    this.emit();
  }

  setTlSpeed(v: number): void {
    this.tlSpeed = v || 1;
    this.emit();
  }

  tlPause(): void {
    this.tlPlaying = false;
    if (this.tlTimer !== null) {
      clearTimeout(this.tlTimer);
      this.tlTimer = null;
    }
    this.emit();
  }

  private tlAdvance = (): void => {
    if (!this.tlActive || !this.tlPlaying) return;
    if (this.tlIndex >= this.events().length - 1) {
      this.tlPause();
      return;
    }
    this.goToEvent(this.tlIndex + 1);
    this.tlTimer = setTimeout(this.tlAdvance, 2200 / this.tlSpeed);
  };

  tlPlay(): void {
    if (!this.tlActive || !this.events().length) return;
    if (this.tlIndex >= this.events().length - 1) this.goToEvent(0);
    this.tlPlaying = true;
    this.tlTimer = setTimeout(this.tlAdvance, 2200 / this.tlSpeed);
    this.emit();
  }

  toggleTimeline(on?: boolean): void {
    if (!this.events().length) return;
    if (on === undefined) on = !this.tlActive;
    if (on && !this.tlActive) {
      this.tlActive = true;
      this.tlIndex = 0;
      this.tlPlaying = false;
      this.select(null);
      this.goToEvent(0);
    } else if (!on && this.tlActive) {
      this.tlPause();
      this.tlActive = false;
      this.tlIndex = 0;
      this.tlVisibleSet = null;
      this.applyVisibility();
      this.resetView();
      this.emit();
    }
  }
}
