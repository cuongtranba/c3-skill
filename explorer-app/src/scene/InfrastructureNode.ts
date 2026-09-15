import * as THREE from "three";
import type { C3Edge, C3Node, Dock } from "../types";
import type { Palette, SkinContext, StatusStyle } from "../skin/types";
import { kindStyle, statusOf } from "../skin/resolve";
import { box, cyl, disposeTree, emissive, strip, type Mesh } from "../skin/kit";
import { ARCH_HEIGHT } from "./constants";
import { labelSprite } from "./label";
import { consolidateStatic } from "./consolidate";

/* One payload node as a building: the skin's archetype builder makes the silhouette; this
 * class adds what every node has regardless of look — the status lamp, dock terminals,
 * selection FX, LOD tags — and drives the per-frame semantic motion. */

export interface DockPart {
  dock: Dock;
  kind: string;
  slit: THREE.MeshBasicMaterial;
}

export interface BuildNodeOptions {
  labels: boolean;
}

export interface FrameState {
  t: number;
  dt: number;
  reduced: boolean;
  blinkOn: boolean;
  hovered: boolean;
  selected: boolean;
  cameraPos: THREE.Vector3;
  /** LOD distances scale with the base so tags appear from the home view. */
  lod: { far: number; near: number };
}

const UNIT_HEIGHT: Record<string, number> = ARCH_HEIGHT;

export class InfrastructureNode {
  readonly id: string;
  readonly node: C3Node;
  readonly group = new THREE.Group();
  readonly w: number;
  readonly d: number;
  readonly roofY: number;
  readonly status: StatusStyle;
  readonly pickMeshes: THREE.Object3D[] = [];
  /** Static meshes folded into merged shells at build time — a draw-call diagnostic. */
  readonly mergedParts: number;
  readonly docks: DockPart[] = [];
  readonly position: THREE.Vector3;

  intensity = 1;
  targetIntensity = 1;
  /** Facet / level / timeline visibility, applied on top of neighbourhood dimming. */
  hidden = false;
  bornAt?: number;
  flashUntil?: number;
  pulseUntil?: number;
  pulseColor?: string;

  private readonly palette: Palette;
  private readonly acc: THREE.MeshBasicMaterial;
  private readonly lamp: Mesh;
  private readonly lampMat: THREE.MeshBasicMaterial;
  private readonly fx: THREE.Group;
  private readonly ring: THREE.Mesh<THREE.RingGeometry, THREE.MeshBasicMaterial>;
  private readonly labelM: THREE.Sprite | null;
  private readonly labelN: THREE.Sprite | null;
  private readonly blinkers: THREE.Object3D[] = [];
  private readonly dockColor: (kind: string) => string;

  constructor(node: C3Node, edgesById: Map<string, C3Edge>, ctx: SkinContext, opts: BuildNodeOptions) {
    const { skin, m: M } = ctx;
    const P = skin.tokens.palette;
    this.palette = P;
    this.dockColor = (kind) => kindStyle(skin, kind).color;
    this.id = node.id;
    this.node = node;
    this.status = statusOf(skin, node);
    this.w = node.layout.w;
    this.d = node.layout.d;
    this.position = new THREE.Vector3(node.layout.x, node.layout.y, node.layout.z);

    this.acc = emissive(this.status.motion === "breathe" ? P.cyan : P.blue, 1.0);
    const obstruction = emissive(P.red, 1.6);
    const builder = skin.archetypes[node.archetype] ?? skin.archetypes.plant ?? Object.values(skin.archetypes)[0];
    const b = builder(node, this.acc, obstruction, ctx);
    skin.decorateNode?.(b, node, ctx);
    this.roofY = b.roofY;
    this.group.add(b.group);
    this.group.position.copy(this.position);
    this.group.userData.c3 = { id: node.id, type: node.type, archetype: node.archetype, district: node.district };
    this.group.name = node.id;

    // Status lamp on a short pole. Colour + motion pattern + inspector text; never colour alone.
    const [lx, ly, lz] = b.lightAt;
    this.lampMat = emissive(this.status.color, 1.8);
    this.lamp = new THREE.Mesh(new THREE.SphereGeometry(0.26, 12, 8), this.lampMat);
    this.lamp.position.set(lx, ly + 0.55, lz);
    this.lamp.userData.statusLamp = true;
    b.group.add(this.lamp);
    b.group.add(cyl(0.05, 0.55, M.steel, lx, ly, lz, 6));

    this.buildDocks(edgesById, ctx);
    // Bake the static shell into one mesh per shared material (see consolidate.ts).
    this.mergedParts = consolidateStatic(this.group, ctx.spinners);

    // Tactical selection FX (hidden until selected): lit foundation, corner brackets, floor ring.
    const sel = skin.tokens.selection.color;
    this.fx = new THREE.Group();
    this.fx.visible = false;
    this.fx.userData.noExport = true;
    const fw = this.w + 3;
    const fd = this.d + 3;
    const bl = 2.2;
    const bm = emissive(sel, 1.3);
    for (const [sx, sz] of [
      [-1, -1],
      [1, -1],
      [1, 1],
      [-1, 1],
    ]) {
      this.fx.add(strip(bl, 0.06, 0.16, sx * (fw / 2 - bl / 2), 0.08, (sz * fd) / 2, bm));
      this.fx.add(strip(0.16, 0.06, bl, (sx * fw) / 2, 0.08, sz * (fd / 2 - bl / 2), bm));
    }
    const r = Math.max(fw, fd) * 0.62;
    this.ring = new THREE.Mesh(
      new THREE.RingGeometry(r, r + 0.18, 64),
      new THREE.MeshBasicMaterial({ color: sel, transparent: true, opacity: 0.5, side: THREE.DoubleSide, toneMapped: false }),
    );
    this.ring.rotation.x = -Math.PI / 2;
    this.ring.position.y = 0.06;
    this.fx.add(this.ring);
    const found = new THREE.Mesh(
      new THREE.PlaneGeometry(fw, fd),
      new THREE.MeshBasicMaterial({ color: sel, transparent: true, opacity: 0.08, depthWrite: false, toneMapped: false }),
    );
    found.rotation.x = -Math.PI / 2;
    found.position.y = 0.05;
    this.fx.add(found);
    this.group.add(this.fx);

    // LOD tags: medium = name; near = name / ID / ● STATUS.
    if (opts.labels) {
      this.labelM = labelSprite(skin.labels, [node.title]);
      this.labelN = labelSprite(skin.labels, [node.title, node.id.toUpperCase(), this.status.text], this.status);
      for (const l of [this.labelM, this.labelN]) {
        l.position.set(0, this.roofY + 4.6, 0);
        l.visible = false;
        this.group.add(l);
      }
    } else {
      this.labelM = null;
      this.labelN = null;
    }

    this.group.traverse((o) => {
      if (o.userData.blink) this.blinkers.push(o);
      if ((o as THREE.Mesh).isMesh && !o.userData.blink && !o.userData.noExport && !this.isUnderFx(o)) {
        o.userData.c3Id = node.id;
        this.pickMeshes.push(o);
      }
    });
  }

  private isUnderFx(o: THREE.Object3D): boolean {
    for (let p: THREE.Object3D | null = o; p; p = p.parent) if (p === this.fx) return true;
    return false;
  }

  /** Armoured cable terminals; routes terminate exactly at the payload's dock points. */
  private buildDocks(edgesById: Map<string, C3Edge>, ctx: SkinContext): void {
    const M = ctx.m;
    // One slit material per edge kind per building: every dock of a kind dims with
    // the building anyway, and sharing the instance lets the slits bake into one mesh.
    const slitByKind = new Map<string, THREE.MeshBasicMaterial>();
    for (const dock of this.node.docks ?? []) {
      const edge = edgesById.get(dock.edgeId);
      const kind = edge?.kind ?? "depends_on";
      const col = this.dockColor(kind);
      const dir = dock.face === "s" ? [0, 1] : dock.face === "n" ? [0, -1] : dock.face === "e" ? [1, 0] : [-1, 0];
      const grp = new THREE.Group();
      // Local: the payload dock point sits 1.4 outside the face; the terminal body is pulled 0.7 back.
      grp.position.set(dock.x - this.position.x - dir[0] * 0.7, 0, dock.z - this.position.z - dir[1] * 0.7);
      if (dir[0] !== 0) grp.rotation.y = Math.PI / 2;
      grp.add(box(1.3, 1.0, 1.4, M.steel));
      grp.add(box(0.9, 0.5, 0.5, M.darkMetal, 0, 1.0, 0));
      let slitMat = slitByKind.get(kind);
      if (!slitMat) {
        slitMat = emissive(col, 1.2);
        slitByKind.set(kind, slitMat);
      }
      const side = dir[0] !== 0 ? dir[0] : dir[1];
      grp.add(strip(0.7, 0.07, 0.05, 0, 0.6, side * 0.72, slitMat));
      grp.userData.dock = dock.id;
      this.group.add(grp);
      this.docks.push({ dock, kind, slit: slitMat });
    }
  }

  /** Nominal export height for the archetype (matches the Go scene export). */
  static unitHeight(archetype: string): number {
    return UNIT_HEIGHT[archetype] ?? 9;
  }

  setSelected(on: boolean): void {
    this.fx.visible = on;
  }

  /** Per-frame semantic motion: breathe, pulse, blink, LOD labels, neighbourhood dimming. */
  update(f: FrameState): void {
    const st = this.status;
    const P = this.palette;
    const t = f.reduced ? 0 : f.t;
    this.intensity += (this.targetIntensity - this.intensity) * 0.1;
    const vis = this.hidden ? 0 : this.intensity;
    const boost = f.hovered || f.selected ? 1.3 : 1;

    let k = 1.0;
    if (st.motion === "breathe") k = 0.7 + 0.8 * (0.5 + 0.5 * Math.sin(t * 1.8));
    this.acc.color.set(st.motion === "breathe" ? P.cyan : P.blue).multiplyScalar(k * vis * vis * boost);

    let lk = 1.8;
    if (st.motion === "pulse") lk = 0.5 + 1.6 * Math.max(0, Math.sin(t * 2.6));
    else if (st.motion === "pulse-slow") lk = 1.0 + 0.8 * (0.5 + 0.5 * Math.sin(t * 1.1));
    let lampColor = st.color;
    if (this.pulseUntil !== undefined) {
      if (f.t < this.pulseUntil) {
        lampColor = this.pulseColor || P.cyan;
        lk = 1.2 + 1.6 * (0.5 + 0.5 * Math.sin(f.t * 8));
      } else {
        this.pulseUntil = undefined;
        this.pulseColor = undefined;
      }
    }
    if (this.flashUntil !== undefined) {
      if (f.t < this.flashUntil) {
        lampColor = P.amber;
        lk = 1.2 + 1.6 * (0.5 + 0.5 * Math.sin(f.t * 6));
      } else this.flashUntil = undefined;
    }
    this.lampMat.color.set(lampColor).multiplyScalar(lk * Math.max(vis, 0.15));
    for (const o of this.blinkers) o.visible = f.blinkOn;

    if (this.bornAt !== undefined) {
      const elapsed = f.t - this.bornAt;
      if (elapsed < 0.6 && !f.reduced) {
        const eased = 1 - Math.pow(1 - elapsed / 0.6, 3);
        this.group.scale.setScalar(Math.max(0.01, Math.min(1, eased)));
      } else {
        this.group.scale.setScalar(1);
        this.bornAt = undefined;
      }
    }

    if (this.fx.visible) this.ring.material.opacity = 0.35 + 0.2 * Math.sin(t * 2.4);

    const dist = f.cameraPos.distanceTo(this.position);
    const showN = dist < f.lod.near;
    const showM = !showN && dist < f.lod.far;
    const ls = THREE.MathUtils.clamp(dist / (f.lod.far * 0.58), 0.3, 1.1);
    if (this.labelN && this.labelM) {
      this.labelN.visible = !this.hidden && showN && vis > 0.4;
      this.labelM.visible = !this.hidden && showM && vis > 0.4;
      for (const l of [this.labelM, this.labelN]) l.scale.copy(l.userData.baseScale as THREE.Vector3).multiplyScalar(ls);
      this.labelN.material.opacity = this.labelM.material.opacity = 0.35 + 0.65 * vis;
    }
    for (const dk of this.docks) dk.slit.color.set(this.dockColor(dk.kind)).multiplyScalar(1.2 * Math.max(vis, 0.1));
  }

  dispose(): void {
    disposeTree(this.group);
  }
}
