/* window.C3_EXPLORER — verification surface. The automated anti-goal checks
 * (unrendered nodes, undrawn edges, broken interactions, missing status) drive
 * this exact shape; never rename an existing member. New members are additive. */

import type { Facets } from "../scene/facets";
import type { SceneJSON } from "../scene/exportScene";
import type { SceneTreeNode } from "../scene/sceneTree";
import type { MotionMode } from "../scene/TrafficSystem";

export interface ExplorerHandle {
  ready: boolean;
  renderedNodeIds(): string[];
  allDataNodeIds(): string[];
  renderedEdgeCount(): number;
  renderStats(): { calls: number; triangles: number; geometries: number; textures: number };
  dataEdgeCount(): number;
  nodesWithoutStatus(): string[];
  selectNodeById(id: string): boolean;
  setLevel(lvl: "context" | "container" | "component" | "all"): void;
  currentSelection(): { id: string; lifecycle: string } | null;
  cameraPosition(): { x: number; y: number; z: number };
  visibleNodeIds(): string[];
  timelineActive(): boolean;
  timelineIndex(): number;
  events(): unknown[];
  goToEvent(idx: number): void;
  tlPlay(): void;
  tlPause(): void;
  toggleTimeline(on?: boolean): void;
  focus(id: string): boolean;
  setMotion(mode: MotionMode): void;
  setFacets(patch: Facets): void;
  getFacets(): Facets;
  clearFacets(): void;
  exportSceneJSON(): SceneJSON;
  inspectorTree(): SceneTreeNode;
  setSkin(id: string): boolean;
  skinId(): string;
}

export interface ExplorerAPI {
  ready: boolean;
  renderedNodeIds(): string[];
  allDataNodeIds(): string[];
  renderedEdgeCount(): number;
  renderStats(): { calls: number; triangles: number; geometries: number; textures: number };
  dataEdgeCount(): number;
  nodesWithoutStatus(): string[];
  selectNodeById(id: string): boolean;
  setLevel(lvl: "context" | "container" | "component" | "all"): void;
  currentSelection(): { id: string; lifecycle: string } | null;
  cameraPosition(): { x: number; y: number; z: number };
  visibleNodeIds(): string[];
  timeline: {
    active(): boolean;
    eventCount(): number;
    index(): number;
    goTo(i: number): boolean;
    play(): void;
    pause(): void;
    visibleNodeIds(): string[];
    toggle(on?: boolean): void;
  };
  focus(id: string): boolean;
  setMotion(mode: MotionMode): void;
  facets: {
    set(patch: Facets): void;
    get(): Facets;
    clear(): void;
  };
  exportSceneJSON(): SceneJSON;
  inspector: {
    tree(): SceneTreeNode;
  };
  /** Re-skins the city in place; false for an unknown id. */
  setSkin(id: string): boolean;
  /** Active skin id. */
  skin(): string;
}

export function buildExplorerAPI(scene: ExplorerHandle): ExplorerAPI {
  return {
    get ready() {
      return scene.ready;
    },
    renderedNodeIds: () => scene.renderedNodeIds(),
    allDataNodeIds: () => scene.allDataNodeIds(),
    renderedEdgeCount: () => scene.renderedEdgeCount(),
    renderStats: () => scene.renderStats(),
    dataEdgeCount: () => scene.dataEdgeCount(),
    nodesWithoutStatus: () => scene.nodesWithoutStatus(),
    selectNodeById: (id) => scene.selectNodeById(id),
    setLevel: (lvl) => scene.setLevel(lvl),
    currentSelection: () => scene.currentSelection(),
    cameraPosition: () => scene.cameraPosition(),
    visibleNodeIds: () => scene.visibleNodeIds(),
    timeline: {
      active: () => scene.timelineActive(),
      eventCount: () => scene.events().length,
      index: () => scene.timelineIndex(),
      goTo: (i) => {
        const count = scene.events().length;
        if (!count) return false;
        const idx = Math.max(0, Math.min(i, count - 1));
        if (!scene.timelineActive()) scene.toggleTimeline(true);
        scene.goToEvent(idx);
        return true;
      },
      play: () => scene.tlPlay(),
      pause: () => scene.tlPause(),
      // During a replay the visible set is what the scrubbed time has created so far.
      visibleNodeIds: () => scene.visibleNodeIds(),
      toggle: (on) => scene.toggleTimeline(on),
    },
    focus: (id) => scene.focus(id),
    setMotion: (mode) => scene.setMotion(mode),
    facets: {
      set: (patch) => scene.setFacets(patch),
      get: () => scene.getFacets(),
      clear: () => scene.clearFacets(),
    },
    exportSceneJSON: () => scene.exportSceneJSON(),
    inspector: {
      tree: () => scene.inspectorTree(),
    },
    setSkin: (id) => scene.setSkin(id),
    skin: () => scene.skinId(),
  };
}

export function installExplorerAPI(scene: ExplorerHandle): void {
  window.C3_EXPLORER = buildExplorerAPI(scene);
}
