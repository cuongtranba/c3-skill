import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls";
import { easeInOutQuad } from "./constants";
import { getVisibleGraph, layoutNodes } from "./layout";
import { diffPayload } from "./liveDiff";
import { addNodeMesh, buildRingGuides, type RingGuide } from "./nodes";
import { addEdgeMesh } from "./edges";
import { lifecycleOf, type C3Event, type C3Payload, type Level } from "../data";
import type { BasicMesh, EdgeRec, LNode, NodeMesh } from "./sceneTypes";

export interface EdgeRef {
  id: string;
  title: string;
}

export interface NodeSelection {
  kind: "node";
  id: string;
  title: string;
  type: string;
  lifecycle: string;
  goal?: string;
  parent?: string;
  staged?: boolean;
  stagedBy?: string[];
  transition?: { from: string; to: string; by: string } | null;
  contains: EdgeRef[];
  uses: EdgeRef[];
  usedBy: EdgeRef[];
}

export interface EdgeSelection {
  kind: "edge";
  edgeKind: string;
  fromTitle: string;
  toTitle: string;
  crossedLabels: string[];
}

export type Selection = NodeSelection | EdgeSelection | null;

export interface TooltipInfo {
  x: number;
  y: number;
  text: string;
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
  focusContainer: string | null;
  query: string;
  dimmed: string[];
  dimmedRings: string[];
  lifecycleCounts: Record<string, number>;
  selection: Selection;
  tooltip: TooltipInfo | null;
  timeline: TimelineSnap;
  lastUpdate: LastUpdate | null;
}

interface CamFly {
  flying: boolean;
  t: number;
  fromPos: THREE.Vector3;
  toPos: THREE.Vector3;
  fromTgt: THREE.Vector3;
  toTgt: THREE.Vector3;
}

function isMeshLike(o: THREE.Object3D): o is THREE.Mesh<THREE.BufferGeometry, THREE.Material> {
  return (o as THREE.Mesh).isMesh === true || (o as THREE.Sprite).isSprite === true;
}

export class ExplorerScene {
  private data: C3Payload;
  private canvas: HTMLCanvasElement;

  // Default to the full graph so the AG-1 no-lost-nodes check holds at load.
  private level: Level = "all";
  private focusContainer: string | null = null;
  private selectedNode: LNode | null = null;
  private selection: Selection = null;
  private query = "";
  private dimmedLifecycles = new Set<string>();
  private dimmedRings = new Set<string>();
  private _t = 0;
  private cam: CamFly | null = null;

  private tlActive = false;
  private tlIndex = 0;
  private tlPlaying = false;
  private tlTimer: ReturnType<typeof setTimeout> | null = null;
  private tlSpeed = 1;
  private tlSavedLevel: Level = "all";
  private tlSavedFocus: string | null = null;
  private tlVisibleSet: Set<string> | null = null;

  private renderer!: THREE.WebGLRenderer;
  private scene!: THREE.Scene;
  private camera!: THREE.PerspectiveCamera;
  private controls!: OrbitControls;
  private raycaster!: THREE.Raycaster;
  private pointer!: THREE.Vector2;
  private worldGroup!: THREE.Group;

  private nodeMeshes: NodeMesh[] = [];
  private edgeMeshes: BasicMesh[] = [];
  private edgeRecs: EdgeRec[] = [];
  private nodeById: Record<string, LNode> = {};
  private hover: LNode | null = null;
  private hoverEdge: EdgeRec | null = null;
  private tooltip: TooltipInfo | null = null;
  private raf: number | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private lastUpdate: LastUpdate | null = null;
  private keys = new Set<string>();
  private isolationEdges: Set<EdgeRec> | null = null;
  private ringGuides: RingGuide[] = [];

  ready = false;

  private listeners = new Set<() => void>();
  private snap: Snapshot;

  constructor(canvas: HTMLCanvasElement, data: C3Payload) {
    this.canvas = canvas;
    this.data = data;

    const first = (data.nodes || []).find((n) => n.type === "container");
    if (first) this.focusContainer = first.id;

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
    const visible = getVisibleGraph(this.data, this.level, this.focusContainer, this.tlActive, this.tlVisibleSet);
    const counts: Record<string, number> = {};
    visible.nodes.forEach((n) => {
      const lc = lifecycleOf(n);
      counts[lc] = (counts[lc] || 0) + 1;
    });
    return {
      ready: this.ready,
      level: this.level,
      focusContainer: this.focusContainer,
      query: this.query,
      dimmed: Array.from(this.dimmedLifecycles),
      dimmedRings: Array.from(this.dimmedRings),
      lifecycleCounts: counts,
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
    };
  }

  /* ─── verification API surface ────────────────────────────────── */
  renderedNodeIds(): string[] {
    return this.nodeMeshes.map((m) => (m.userData.node as LNode).id);
  }
  allDataNodeIds(): string[] {
    return (this.data.nodes || []).map((n) => n.id);
  }
  renderedEdgeCount(): number {
    return this.edgeRecs.length;
  }
  dataEdgeCount(): number {
    return (this.data.edges || []).length;
  }
  nodesWithoutStatus(): string[] {
    return this.nodeMeshes.filter((m) => !m.userData.hasStatusGlyph).map((m) => (m.userData.node as LNode).id);
  }
  currentSelection(): { id: string; lifecycle: string } | null {
    return this.selectedNode ? { id: this.selectedNode.id, lifecycle: lifecycleOf(this.selectedNode) } : null;
  }
  selectNodeById(id: string): boolean {
    const mesh = this.nodeMeshes.find((m) => (m.userData.node as LNode).id === id);
    if (!mesh) return false;
    this.selectNode(mesh.userData.node as LNode);
    return true;
  }

  events(): C3Event[] {
    return this.data.events || [];
  }

  getData(): C3Payload {
    return this.data;
  }

  cameraPosition(): { x: number; y: number; z: number } {
    const p = this.camera.position;
    return { x: p.x, y: p.y, z: p.z };
  }

  visibleNodeIds(): string[] {
    return this.nodeMeshes
      .filter((m) => {
        const n = m.userData.node as LNode;
        return !n._grp || n._grp.visible;
      })
      .map((m) => (m.userData.node as LNode).id);
  }

  /* ─── scene setup ─────────────────────────────────────────────── */
  private initScene(): void {
    const canvas = this.canvas;
    const w = canvas.clientWidth || window.innerWidth;
    const h = canvas.clientHeight || window.innerHeight;

    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.setSize(w, h);
    this.renderer.outputEncoding = THREE.sRGBEncoding;

    this.scene = new THREE.Scene();
    this.scene.background = new THREE.Color("#f5f4f0");
    this.scene.fog = new THREE.Fog("#f5f4f0", 100, 230);

    this.camera = new THREE.PerspectiveCamera(46, w / h, 0.5, 3000);
    this.camera.position.set(6, 74, 88);

    this.controls = new OrbitControls(this.camera, this.renderer.domElement);
    this.controls.enableDamping = true;
    this.controls.dampingFactor = 0.08;
    this.controls.minDistance = 15;
    this.controls.maxDistance = 180;
    this.controls.maxPolarAngle = 1.44;
    this.controls.target.set(0, 0, 0);

    this.scene.add(new THREE.HemisphereLight(0xffffff, 0xe4e2dc, 1.05));
    const dl = new THREE.DirectionalLight(0xffffff, 0.52);
    dl.position.set(34, 70, 26);
    this.scene.add(dl);
    const dl2 = new THREE.DirectionalLight(0xffffff, 0.2);
    dl2.position.set(-40, 30, -30);
    this.scene.add(dl2);

    const ground = new THREE.Mesh(
      new THREE.CircleGeometry(130, 64),
      new THREE.MeshStandardMaterial({ color: "#eeede8", roughness: 1, metalness: 0 }),
    );
    ground.rotation.x = -Math.PI / 2;
    ground.position.y = -0.05;
    this.scene.add(ground);

    this.raycaster = new THREE.Raycaster();
    this.pointer = new THREE.Vector2(-2, -2);

    this.worldGroup = new THREE.Group();
    this.scene.add(this.worldGroup);

    const dom = this.renderer.domElement;
    dom.addEventListener("pointermove", this.onPointerMove);
    dom.addEventListener("pointerdown", this.onPointerDown);
    dom.addEventListener("pointerup", this.onPointerUp);
    dom.addEventListener("dblclick", this.onDblClick);

    this.resizeObserver = new ResizeObserver(() => {
      const nw = canvas.clientWidth,
        nh = canvas.clientHeight;
      if (!nw || !nh) return;
      this.camera.aspect = nw / nh;
      this.camera.updateProjectionMatrix();
      this.renderer.setSize(nw, nh);
    });
    this.resizeObserver.observe(canvas.parentElement || document.body);

    window.addEventListener("resize", this.onWindowResize);
    window.addEventListener("keydown", this.onKeyDown);
    window.addEventListener("keyup", this.onKeyUp);
    window.addEventListener("blur", this.onWindowBlur);

    this.ready = true;
    this.rebuild();
    this.loop();
  }

  private onWindowResize = (): void => {
    const nw = this.canvas.clientWidth || window.innerWidth;
    const nh = this.canvas.clientHeight || window.innerHeight;
    if (!nw || !nh) return;
    this.camera.aspect = nw / nh;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(nw, nh);
  };

  dispose(): void {
    if (this.raf !== null) cancelAnimationFrame(this.raf);
    if (this.tlTimer !== null) clearTimeout(this.tlTimer);
    const dom = this.renderer.domElement;
    dom.removeEventListener("pointermove", this.onPointerMove);
    dom.removeEventListener("pointerdown", this.onPointerDown);
    dom.removeEventListener("pointerup", this.onPointerUp);
    dom.removeEventListener("dblclick", this.onDblClick);
    window.removeEventListener("resize", this.onWindowResize);
    window.removeEventListener("keydown", this.onKeyDown);
    window.removeEventListener("keyup", this.onKeyUp);
    window.removeEventListener("blur", this.onWindowBlur);
    this.resizeObserver?.disconnect();
    this.clearWorld();
    this.renderer.dispose();
  }

  /* ─── rebuild ─────────────────────────────────────────────────── */
  private clearWorld(): void {
    while (this.worldGroup.children.length) {
      const c = this.worldGroup.children[this.worldGroup.children.length - 1];
      c.traverse((o) => {
        if (isMeshLike(o)) {
          o.geometry?.dispose();
          const mat = o.material as THREE.Material & { map?: THREE.Texture };
          if (mat) {
            mat.map?.dispose();
            mat.dispose();
          }
        }
      });
      this.worldGroup.remove(c);
    }
    this.nodeMeshes = [];
    this.edgeMeshes = [];
    this.edgeRecs = [];
    this.ringGuides = [];
    this.nodeById = {};
    this.hover = null;
    this.hoverEdge = null;
  }

  private rebuild(): void {
    this.clearWorld();
    const { nodes, edges } = getVisibleGraph(this.data, this.level, this.focusContainer, this.tlActive, this.tlVisibleSet);

    layoutNodes(nodes, edges);
    nodes.forEach((n) => {
      this.nodeById[n.id] = n;
    });

    this.ringGuides = buildRingGuides(nodes, this.worldGroup);
    nodes.forEach((n) => this.nodeMeshes.push(addNodeMesh(n, this.worldGroup)));
    edges.forEach((e) => {
      const built = addEdgeMesh(e, this.nodeById, this.worldGroup);
      if (built) {
        this.edgeMeshes.push(built.pickTube);
        this.edgeRecs.push(built.rec);
      }
    });

    // Fresh meshes start undimmed; re-apply any active lifecycle/ring filter.
    if (this.dimmedLifecycles.size || this.dimmedRings.size) this.applyFilterDim();

    // A rebuild (level switch, live update) recreates every mesh; carry an
    // active selection's isolation over to the fresh graph or drop it if the
    // node no longer exists.
    if (this.selectedNode) {
      const nn = this.nodeById[this.selectedNode.id];
      if (nn) {
        this.selectedNode = nn;
        this.applyIsolation(nn);
      } else {
        this.selectedNode = null;
        this.selection = null;
      }
    }

    this.emit();
  }

  /* ─── level / focus / legend ──────────────────────────────────── */
  setLevel(lvl: Level): void {
    this.level = lvl;
    this.query = "";
    this.clearSelectionInternal();
    this.rebuild();
    this.resetCamera();
  }

  setFocusContainer(id: string): void {
    this.focusContainer = id;
    this.clearSelectionInternal();
    this.rebuild();
    this.resetCamera();
  }

  toggleLifecycle(lc: string): void {
    if (this.dimmedLifecycles.has(lc)) this.dimmedLifecycles.delete(lc);
    else this.dimmedLifecycles.add(lc);
    this.applyFilterDim();
    this.emit();
  }

  toggleRing(key: string): void {
    if (this.dimmedRings.has(key)) this.dimmedRings.delete(key);
    else this.dimmedRings.add(key);
    this.applyFilterDim();
    this.emit();
  }

  private applyFilterDim(): void {
    this.nodeMeshes.forEach((mesh) => {
      const n = mesh.userData.node as LNode;
      const lc = lifecycleOf(n);
      const dimmed =
        (this.dimmedLifecycles.size > 0 && this.dimmedLifecycles.has(lc)) ||
        (this.dimmedRings.size > 0 && this.dimmedRings.has(n.ring || "infra"));
      const grp = n._grp;
      if (!grp) return;
      grp.traverse((o) => {
        if (!isMeshLike(o)) return;
        const base = o.userData.baseOpacity !== undefined ? (o.userData.baseOpacity as number) : 1;
        const mat = o.material as THREE.Material;
        if (!o.userData.isLabel) {
          mat.transparent = true;
          mat.opacity = dimmed ? base * 0.25 : base;
        } else {
          mat.opacity = dimmed ? 0.15 : 1;
        }
      });
    });
  }

  /* ─── search ──────────────────────────────────────────────────── */
  private matches(n: LNode): boolean {
    if (!this.query) return false;
    return ((n.title || "") + " " + n.id + " " + (n.type || "") + " " + (n.lifecycle || ""))
      .toLowerCase()
      .includes(this.query);
  }

  setQuery(raw: string): void {
    this.query = raw.trim().toLowerCase();
    this.applySearch();
    this.emit();
  }

  selectFirstMatch(): void {
    const hit = this.nodeMeshes.find((m) => this.matches(m.userData.node as LNode));
    if (hit) this.selectNode(hit.userData.node as LNode);
  }

  private applySearch(): void {
    const qi = this.query;
    this.nodeMeshes.forEach((mesh) => {
      const n = mesh.userData.node as LNode;
      const hit = !!qi && this.matches(n);
      n._searchHit = hit;
      const grp = n._grp;
      if (!grp) return;
      if (qi) {
        grp.traverse((o) => {
          if (!isMeshLike(o)) return;
          const base = o.userData.baseOpacity !== undefined ? (o.userData.baseOpacity as number) : 1;
          const mat = o.material as THREE.Material;
          if (!o.userData.isLabel) {
            mat.transparent = true;
            mat.opacity = hit ? base : base * 0.17;
          } else {
            mat.opacity = hit ? 1 : 0.13;
          }
        });
      } else {
        grp.traverse((o) => {
          if (!isMeshLike(o)) return;
          const base = o.userData.baseOpacity !== undefined ? (o.userData.baseOpacity as number) : 1;
          (o.material as THREE.Material).opacity = base;
        });
      }
    });
    if (!qi) this.highlightConnections(this.hover);
  }

  /* ─── picking / pointer ───────────────────────────────────────── */
  private ndcFromEvent(e: PointerEvent | MouseEvent): void {
    const rect = this.renderer.domElement.getBoundingClientRect();
    this.pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    this.pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
  }

  private pickNode(): NodeMesh | null {
    this.raycaster.setFromCamera(this.pointer, this.camera);
    const hits = this.raycaster.intersectObjects(this.nodeMeshes, false);
    for (const h of hits) {
      const n = (h.object as NodeMesh).userData.node as LNode;
      if (!n._grp || n._grp.visible) return h.object as NodeMesh;
    }
    return null;
  }

  private pickEdge(): EdgeRec | null {
    this.raycaster.setFromCamera(this.pointer, this.camera);
    const hits = this.raycaster.intersectObjects(this.edgeMeshes, false);
    for (const h of hits) {
      const rec = h.object.userData.rec as EdgeRec;
      if (rec.tube.visible) return rec;
    }
    return null;
  }

  private onPointerMove = (e: PointerEvent): void => {
    this.ndcFromEvent(e);
    const m = this.pickNode();

    if (m) {
      this.renderer.domElement.style.cursor = "pointer";
      const n = m.userData.node as LNode;
      if (this.hoverEdge) {
        this.hoverEdge = null;
        this.highlightConnections(this.hover);
      }
      if (this.hover !== n) {
        this.hover = n;
        this.highlightConnections(n);
      }
      this.tooltip = {
        x: e.clientX,
        y: e.clientY,
        text: (n.title || n.id) + " · " + (n.type || "") + " · " + lifecycleOf(n),
      };
      this.emit();
      return;
    }

    const rec = this.pickEdge();
    if (rec) {
      this.renderer.domElement.style.cursor = "pointer";
      if (this.hover) {
        this.hover = null;
        this.highlightConnections(null);
      }
      if (this.hoverEdge !== rec) {
        this.hoverEdge = rec;
        this.boostEdge(rec);
      }
      const na = this.nodeById[rec.from],
        nb = this.nodeById[rec.to];
      this.tooltip = {
        x: e.clientX,
        y: e.clientY,
        text:
          (na ? na.title || na.id : rec.from) + " → " + (nb ? nb.title || nb.id : rec.to) + " · " + rec.kind,
      };
      this.emit();
      return;
    }

    this.renderer.domElement.style.cursor = "grab";
    let changed = false;
    if (this.hoverEdge) {
      this.hoverEdge = null;
      this.highlightConnections(this.hover);
      changed = true;
    }
    if (this.hover) {
      this.hover = null;
      this.highlightConnections(null);
      changed = true;
    }
    if (this.tooltip) {
      this.tooltip = null;
      changed = true;
    }
    if (changed) this.emit();
  };

  /* ─── keyboard free-fly (WASD / arrows) ───────────────────────── */
  private static readonly MOVE_KEYS = new Set([
    "w", "a", "s", "d", "arrowup", "arrowdown", "arrowleft", "arrowright",
  ]);

  private onKeyDown = (e: KeyboardEvent): void => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const t = e.target as HTMLElement | null;
    if (t && (t.tagName === "INPUT" || t.tagName === "SELECT" || t.tagName === "TEXTAREA")) return;
    const key = e.key.toLowerCase();
    if (!ExplorerScene.MOVE_KEYS.has(key)) return;
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

    // Speed scales with zoom so travel feels constant at any distance.
    const speed = Math.max(this.camera.position.distanceTo(this.controls.target) * 0.9, 14) * 0.016;
    move.normalize().multiplyScalar(speed);
    this.camera.position.add(move);
    this.controls.target.add(move);
    if (this.cam) this.cam.flying = false;
  }

  private downX = 0;
  private downY = 0;
  private onPointerDown = (e: PointerEvent): void => {
    this.downX = e.clientX;
    this.downY = e.clientY;
  };
  private onPointerUp = (e: PointerEvent): void => {
    if (Math.abs(e.clientX - this.downX) < 5 && Math.abs(e.clientY - this.downY) < 5) this.onSingleClick(e);
  };
  private onDblClick = (e: MouseEvent): void => {
    this.ndcFromEvent(e);
    const m = this.pickNode();
    if (!m) return;
    const n = m.userData.node as LNode;
    if (n.type === "container" || n.type === "system") {
      this.focusContainer = n.id;
      this.setLevel("component");
    } else {
      this.selectNode(n);
    }
  };

  private onSingleClick(e: PointerEvent): void {
    this.ndcFromEvent(e);
    const m = this.pickNode();
    if (m) {
      this.selectNode(m.userData.node as LNode);
      return;
    }
    const rec = this.pickEdge();
    if (rec) this.selectEdge(rec);
  }

  /* ─── highlight / dim ─────────────────────────────────────────── */
  private highlightConnections(n: LNode | null): void {
    // With no hover target, keep the selected node's edges highlighted so the
    // isolated effect area never falls back to the dim base look.
    const focus = n ?? this.selectedNode;
    this.edgeRecs.forEach((r) => {
      // Isolation wins: an edge outside the selected node's effect area stays
      // fully hidden no matter what hover does.
      if (this.isolationEdges && !this.isolationEdges.has(r)) {
        r.particles.forEach((p) => (p.visible = false));
        return;
      }
      const touch = !!focus && (r.from === focus.id || r.to === focus.id);
      const op = focus ? (touch ? 0.88 : 0.06) : r.baseOpacity;
      r.tube.material.opacity = op;
      r.arrowMeshes.forEach((ar) => {
        ar.material.opacity = focus ? (touch ? 0.92 : 0.05) : 0.8;
      });
      r.gates.forEach((g) => {
        g.material.opacity = focus ? (touch ? 0.8 : 0.04) : 0.5;
      });
      r.particles.forEach((p) => {
        p.visible = !focus || touch;
        if (!focus || touch) p.scale.setScalar(focus && touch ? 1.45 : 1);
      });
    });
    this.nodeMeshes.forEach((mesh) => {
      const nn = mesh.userData.node as LNode;
      const grp = nn._grp;
      if (!grp) return;
      let connected = !focus || nn.id === focus.id;
      if (focus && !connected) {
        connected = this.edgeRecs.some(
          (r) => (r.from === focus.id && r.to === nn.id) || (r.to === focus.id && r.from === nn.id),
        );
      }
      const dim = !!focus && !connected;
      grp.traverse((o) => {
        if (!isMeshLike(o)) return;
        const base = o.userData.baseOpacity !== undefined ? (o.userData.baseOpacity as number) : 1;
        const mat = o.material as THREE.Material;
        if (!o.userData.isLabel) {
          mat.transparent = true;
          mat.opacity = dim ? base * 0.22 : base;
        } else {
          mat.opacity = dim ? 0.12 : 1;
        }
      });
      mesh.material.emissiveIntensity =
        n && nn.id === n.id ? 0.32 : this.selectedNode && this.selectedNode.id === nn.id ? 0.38 : 0;
    });
  }

  private boostEdge(rec: EdgeRec): void {
    this.highlightConnections(null);
    this.edgeRecs.forEach((r) => {
      if (r === rec) return;
      r.tube.material.opacity = Math.min(r.tube.material.opacity, 0.07);
      r.arrowMeshes.forEach((a2) => {
        a2.material.opacity = 0.06;
      });
      r.gates.forEach((g2) => {
        g2.material.opacity = 0.04;
      });
      r.particles.forEach((p) => {
        p.visible = false;
      });
    });
    rec.tube.material.opacity = 0.95;
    rec.arrowMeshes.forEach((a2) => {
      a2.material.opacity = 1;
    });
    rec.gates.forEach((g2) => {
      g2.material.opacity = 0.88;
    });
    rec.particles.forEach((p) => {
      p.visible = true;
      p.scale.setScalar(1.6);
    });
  }

  /* ─── selection isolation ─────────────────────────────────────── */
  // "Show only the effect area": everything not wired to the selected node is
  // hidden entirely, so the neighborhood reads as its own small scene.
  private applyIsolation(n: LNode): void {
    const connected = new Set<string>([n.id]);
    this.edgeRecs.forEach((r) => {
      if (r.from === n.id) connected.add(r.to);
      if (r.to === n.id) connected.add(r.from);
    });
    this.nodeMeshes.forEach((mesh) => {
      const nn = mesh.userData.node as LNode;
      if (nn._grp) nn._grp.visible = connected.has(nn.id);
    });
    this.isolationEdges = new Set();
    this.edgeRecs.forEach((r) => {
      const on = r.from === n.id || r.to === n.id;
      r.tube.visible = on;
      r.arrowMeshes.forEach((a) => (a.visible = on));
      r.gates.forEach((g) => (g.visible = on));
      r.particles.forEach((p) => (p.visible = on));
      if (on) this.isolationEdges!.add(r);
    });

    // Light up the rings the effect area sits on; fade the rest so the eye
    // reads which tiers the selection spans.
    const activeRings = new Set<string>();
    connected.forEach((id) => {
      const cn = this.nodeById[id];
      if (cn) activeRings.add(cn.ring || "infra");
    });
    this.ringGuides.forEach((g) => {
      const on = activeRings.has(g.key);
      g.band.material.opacity = on ? 0.12 : 0.012;
      g.orb.material.opacity = on ? 0.55 : 0.05;
      g.label.material.opacity = on ? 1 : 0.15;
    });
  }

  private clearIsolation(): void {
    this.isolationEdges = null;
    this.ringGuides.forEach((g) => {
      g.band.material.opacity = 0.045;
      g.orb.material.opacity = 0.18;
      g.label.material.opacity = 1;
    });
    this.nodeMeshes.forEach((mesh) => {
      const nn = mesh.userData.node as LNode;
      if (nn._grp) nn._grp.visible = true;
    });
    this.edgeRecs.forEach((r) => {
      r.tube.visible = true;
      r.arrowMeshes.forEach((a) => (a.visible = true));
      r.gates.forEach((g) => (g.visible = true));
      r.particles.forEach((p) => (p.visible = true));
    });
  }

  /* ─── selection ───────────────────────────────────────────────── */
  private selectNode(n: LNode): void {
    this.selectedNode = n;

    const ref = (id: string): EdgeRef => {
      const on = this.nodeById[id];
      return { id, title: on ? on.title || on.id : id };
    };
    const contains = this.edgeRecs.filter((r) => r.kind === "contains" && r.from === n.id).map((r) => ref(r.to));
    const uses = this.edgeRecs.filter((r) => r.kind === "uses" && r.from === n.id).map((r) => ref(r.to));
    const usedBy = this.edgeRecs.filter((r) => r.kind === "uses" && r.to === n.id).map((r) => ref(r.from));

    this.selection = {
      kind: "node",
      id: n.id,
      title: n.title || n.id,
      type: n.type || "node",
      lifecycle: lifecycleOf(n),
      goal: n.goal,
      parent: n.parent,
      staged: n.staged,
      stagedBy: n.stagedBy,
      transition: n.transition ? { from: n.transition.from, to: n.transition.to, by: n.transition.by } : null,
      contains,
      uses,
      usedBy,
    };

    this.applyIsolation(n);
    this.highlightConnections(n);
    this.flyTo(n);
    this.emit();
  }

  private selectEdge(rec: EdgeRec): void {
    this.selectedNode = null;
    const na = this.nodeById[rec.from],
      nb = this.nodeById[rec.to];
    this.selection = {
      kind: "edge",
      edgeKind: rec.kind,
      fromTitle: na ? na.title || na.id : rec.from,
      toTitle: nb ? nb.title || nb.id : rec.to,
      crossedLabels: rec.crossedLabels,
    };
    this.boostEdge(rec);
    const mid = rec.curve.getPoint(0.5);
    const dir = this.camera.position.clone().sub(this.controls.target).normalize();
    const toPos = mid.clone().add(dir.multiplyScalar(28));
    toPos.y = Math.max(toPos.y, 14);
    this.cam = {
      flying: true,
      t: 0,
      fromPos: this.camera.position.clone(),
      toPos,
      fromTgt: this.controls.target.clone(),
      toTgt: mid.clone(),
    };
    this.emit();
  }

  clearSelection(): void {
    this.clearSelectionInternal();
    this.highlightConnections(this.hover);
    this.emit();
  }

  private clearSelectionInternal(): void {
    const hadSelection = this.selectedNode !== null;
    this.selectedNode = null;
    this.selection = null;
    if (hadSelection) this.clearIsolation();
  }

  /* ─── camera ──────────────────────────────────────────────────── */
  private flyTo(n: LNode): void {
    const tgt = new THREE.Vector3(n._x ?? 0, n._hgt || 1.5, n._z ?? 0);
    const dir = this.camera.position.clone().sub(this.controls.target).normalize();
    // Frame the neighborhood, not the node's face: stay wide and never zoom
    // in past where the camera already is.
    const base = n.type === "system" || n.type === "container" ? 58 : 48;
    const dist = Math.max(base, this.camera.position.distanceTo(this.controls.target) * 0.75);
    const toPos = tgt.clone().add(dir.multiplyScalar(dist));
    toPos.y = Math.max(toPos.y, 22);
    this.cam = {
      flying: true,
      t: 0,
      fromPos: this.camera.position.clone(),
      toPos,
      fromTgt: this.controls.target.clone(),
      toTgt: tgt,
    };
  }

  private resetCamera(): void {
    this.cam = {
      flying: true,
      t: 0,
      fromPos: this.camera.position.clone(),
      toPos: new THREE.Vector3(6, 74, 88),
      fromTgt: this.controls.target.clone(),
      toTgt: new THREE.Vector3(0, 0, 0),
    };
  }

  /* ─── animation loop ──────────────────────────────────────────── */
  private loop = (): void => {
    this.raf = requestAnimationFrame(this.loop);
    this._t += 0.016;

    this.edgeRecs.forEach((r) => {
      r.particles.forEach((p) => {
        if (!p.visible) {
          p.material.opacity = 0;
          return;
        }
        p.userData.t = ((p.userData.t as number) + (p.userData.speed as number) * 0.016) % 1;
        const pos = r.curve.getPoint(p.userData.t as number);
        p.position.copy(pos);
        p.material.transparent = true;
        p.material.opacity = 0.9;
        if (r.kind === "affects")
          p.material.opacity = 0.5 + 0.5 * Math.sin(this._t * 6 + (p.userData.t as number) * Math.PI * 2);
      });
    });

    // born-scale (ease-out ~0.01→1 over 0.6s) + amber flash: used by both the
    // timeline replay and live updates, so it runs unconditionally.
    this.nodeMeshes.forEach((mesh) => {
      const n = mesh.userData.node as LNode;
      const grp = n._grp;
      if (!grp) return;
      if (n._bornAt !== undefined) {
        const elapsed = this._t - n._bornAt;
        if (elapsed < 0.6) {
          const eased = 1 - Math.pow(1 - elapsed / 0.6, 3);
          grp.scale.setScalar(Math.max(0.01, Math.min(1, eased)));
        } else {
          grp.scale.setScalar(1);
        }
      }
      if (n._flashUntil !== undefined && this._t < n._flashUntil) {
        mesh.material.emissive = new THREE.Color("#d98a2b");
        mesh.material.emissiveIntensity = 0.25 + 0.35 * (0.5 + 0.5 * Math.sin(this._t * 6));
      } else if (n._flashUntil !== undefined && this._t >= n._flashUntil) {
        delete n._flashUntil;
        mesh.material.emissive = new THREE.Color(0x000000);
        mesh.material.emissiveIntensity = 0;
      }
      // live action pulse (mint = read, amber = mutation)
      if (n._pulseUntil !== undefined && this._t < n._pulseUntil) {
        mesh.material.emissive = new THREE.Color(n._pulseColor || "#2fa89a");
        mesh.material.emissiveIntensity = 0.2 + 0.3 * (0.5 + 0.5 * Math.sin(this._t * 5));
      } else if (n._pulseUntil !== undefined && this._t >= n._pulseUntil) {
        delete n._pulseUntil;
        delete n._pulseColor;
        mesh.material.emissive = new THREE.Color(0x000000);
        mesh.material.emissiveIntensity = 0;
      }
    });

    // staged node amber pulse (emissive sine)
    this.nodeMeshes.forEach((mesh) => {
      const n = mesh.userData.node as LNode;
      const lc = lifecycleOf(n);
      if (lc === "staged" || n.staged) {
        mesh.material.emissive = new THREE.Color("#d98a2b");
        mesh.material.emissiveIntensity = 0.15 + 0.22 * (0.5 + 0.5 * Math.sin(this._t * 3.5));
      }
    });

    if (this.query) {
      const pulse = 0.5 + 0.5 * Math.sin(this._t * 4);
      this.nodeMeshes.forEach((mesh) => {
        if ((mesh.userData.node as LNode)._searchHit) {
          mesh.material.emissive = new THREE.Color("#2fa89a");
          mesh.material.emissiveIntensity = 0.22 + pulse * 0.45;
        }
      });
    }

    if (this.selectedNode && this.selectedNode._mesh) {
      this.selectedNode._mesh.material.emissiveIntensity = 0.28 + 0.22 * (0.5 + 0.5 * Math.sin(this._t * 2.8));
    }

    this.applyKeyboardMove();

    if (this.cam && this.cam.flying) {
      this.cam.t += 0.022;
      const t = Math.min(this.cam.t, 1);
      const e = easeInOutQuad(t);
      this.camera.position.lerpVectors(this.cam.fromPos, this.cam.toPos, e);
      this.controls.target.lerpVectors(this.cam.fromTgt, this.cam.toTgt, e);
      if (t >= 1) this.cam.flying = false;
    }

    this.controls.update();
    this.renderer.render(this.scene, this.camera);
  };

  /* ─── live mode ───────────────────────────────────────────────── */
  applyLiveData(next: C3Payload): void {
    const diff = diffPayload(this.data, next);
    if (this.tlActive) this.toggleTimeline(false);
    this.data = next;
    if (this.focusContainer && !next.nodes.some((n) => n.id === this.focusContainer)) {
      const first = next.nodes.find((n) => n.type === "container");
      this.focusContainer = first ? first.id : null;
    }
    this.rebuild();
    diff.added.forEach((id) => {
      const n = this.nodeById[id];
      if (n) n._bornAt = this._t;
    });
    diff.changed.forEach((id) => {
      const n = this.nodeById[id];
      if (n) n._flashUntil = this._t + 1.6;
    });
    this.lastUpdate = {
      ts: Date.now(),
      added: diff.added.length,
      removed: diff.removed.length,
      changed: diff.changed.length,
    };
    this.emit();
  }

  pulseNodes(ids: string[], color: "mint" | "amber"): void {
    const hex = color === "amber" ? "#d98a2b" : "#2fa89a";
    ids.forEach((id) => {
      const n = this.nodeById[id];
      if (!n) return;
      n._pulseUntil = this._t + 1.6;
      n._pulseColor = hex;
    });
  }

  /* ─── timeline mode ───────────────────────────────────────────── */
  timelineActive(): boolean {
    return this.tlActive;
  }
  timelineIndex(): number {
    return this.tlIndex;
  }

  private tlComputeVisibleSet(idx: number): Set<string> {
    const events = this.events();
    const allNodes = this.data.nodes || [];

    const adrEventIndex: Record<string, number> = {};
    events.forEach((ev, i) => {
      adrEventIndex[ev.id] = i;
    });

    const visible = new Set<string>();
    for (let i = 0; i <= idx; i++) {
      const ev = events[i];
      if (!ev) continue;
      (ev.creates || []).forEach((id) => visible.add(id));
    }

    allNodes.forEach((n) => {
      if (n.type !== "adr") return;
      if (visible.has(n.id)) return;
      const evIdx = adrEventIndex[n.id];
      if (evIdx === undefined || evIdx <= idx) visible.add(n.id);
    });

    return visible;
  }

  private tlFlyToCentroid(ids: string[]): void {
    if (!ids.length) return;
    let sx = 0,
      sz = 0,
      count = 0;
    ids.forEach((id) => {
      const n = this.nodeById[id];
      if (n && n._x !== undefined) {
        sx += n._x;
        sz += n._z ?? 0;
        count++;
      }
    });
    if (!count) return;
    const tgt = new THREE.Vector3(sx / count, 0, sz / count);
    const dir = this.camera.position.clone().sub(this.controls.target).normalize();
    const toPos = tgt.clone().add(dir.multiplyScalar(48));
    toPos.y = Math.max(toPos.y, 20);
    this.cam = {
      flying: true,
      t: 0,
      fromPos: this.camera.position.clone(),
      toPos,
      fromTgt: this.controls.target.clone(),
      toTgt: tgt,
    };
  }

  goToEvent(idx: number): void {
    const events = this.events();
    if (!events.length) return;
    idx = Math.max(0, Math.min(idx, events.length - 1));
    this.tlIndex = idx;

    const ev = events[idx];
    this.tlVisibleSet = this.tlComputeVisibleSet(idx);
    this.rebuild();

    if (ev) {
      (ev.creates || []).forEach((id) => {
        const n = this.nodeById[id];
        if (n) n._bornAt = this._t;
      });
      (ev.modifies || []).forEach((id) => {
        const n = this.nodeById[id];
        if (n) n._flashUntil = this._t + 1.6;
      });
      const camIds = (ev.creates || []).concat(ev.modifies || []);
      if (camIds.length) this.tlFlyToCentroid(camIds);
    }

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
    const events = this.events();
    if (this.tlIndex >= events.length - 1) {
      this.tlPause();
      return;
    }
    this.goToEvent(this.tlIndex + 1);
    this.tlTimer = setTimeout(this.tlAdvance, 2200 / this.tlSpeed);
  };

  tlPlay(): void {
    if (!this.tlActive) return;
    const events = this.events();
    if (!events.length) return;
    if (this.tlIndex >= events.length - 1) this.goToEvent(0);
    this.tlPlaying = true;
    this.tlTimer = setTimeout(this.tlAdvance, 2200 / this.tlSpeed);
    this.emit();
  }

  toggleTimeline(on?: boolean): void {
    const events = this.events();
    if (!events.length) return;

    if (on === undefined) on = !this.tlActive;

    if (on && !this.tlActive) {
      this.tlSavedLevel = this.level;
      this.tlSavedFocus = this.focusContainer;

      this.tlActive = true;
      this.tlIndex = 0;
      this.tlPlaying = false;

      this.level = "all";
      this.clearSelectionInternal();
      this.query = "";

      this.goToEvent(0);
    } else if (!on && this.tlActive) {
      this.tlPause();
      this.tlActive = false;
      this.tlIndex = 0;
      this.tlVisibleSet = null;

      this.level = this.tlSavedLevel;
      this.focusContainer = this.tlSavedFocus;

      this.rebuild();
      this.resetCamera();
    }
  }
}
