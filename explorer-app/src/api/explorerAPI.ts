/* window.C3_EXPLORER — verification surface. The automated anti-goal checks
 * (unrendered nodes, undrawn edges, broken interactions, missing status) drive
 * this exact shape; never rename its members. */

export interface ExplorerHandle {
  ready: boolean;
  renderedNodeIds(): string[];
  allDataNodeIds(): string[];
  renderedEdgeCount(): number;
  dataEdgeCount(): number;
  nodesWithoutStatus(): string[];
  selectNodeById(id: string): boolean;
  setLevel(lvl: "context" | "container" | "component"): void;
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
}

export interface ExplorerAPI {
  ready: boolean;
  renderedNodeIds(): string[];
  allDataNodeIds(): string[];
  renderedEdgeCount(): number;
  dataEdgeCount(): number;
  nodesWithoutStatus(): string[];
  selectNodeById(id: string): boolean;
  setLevel(lvl: "context" | "container" | "component"): void;
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
}

export function buildExplorerAPI(scene: ExplorerHandle): ExplorerAPI {
  return {
    get ready() {
      return scene.ready;
    },
    renderedNodeIds: () => scene.renderedNodeIds(),
    allDataNodeIds: () => scene.allDataNodeIds(),
    renderedEdgeCount: () => scene.renderedEdgeCount(),
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
      visibleNodeIds: () => scene.renderedNodeIds(),
      toggle: (on) => scene.toggleTimeline(on),
    },
  };
}

export function installExplorerAPI(scene: ExplorerHandle): void {
  window.C3_EXPLORER = buildExplorerAPI(scene);
}
