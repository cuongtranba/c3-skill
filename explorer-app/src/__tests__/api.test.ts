import { describe, expect, it, vi } from "vitest";
import { buildExplorerAPI, type ExplorerHandle } from "../api/explorerAPI";

function stubHandle(overrides: Partial<ExplorerHandle> = {}): ExplorerHandle {
  return {
    ready: true,
    renderedNodeIds: () => ["c3-0", "c3-1"],
    allDataNodeIds: () => ["c3-0", "c3-1"],
    renderedEdgeCount: () => 1,
    renderStats: () => ({ calls: 0, triangles: 0, geometries: 0, textures: 0 }),
    dataEdgeCount: () => 1,
    nodesWithoutStatus: () => [],
    selectNodeById: () => true,
    setLevel: () => {},
    currentSelection: () => null,
    cameraPosition: () => ({ x: 6, y: 74, z: 88 }),
    visibleNodeIds: () => ["c3-0", "c3-1"],
    timelineActive: () => false,
    timelineIndex: () => 0,
    events: () => [{}, {}, {}],
    goToEvent: () => {},
    tlPlay: () => {},
    tlPause: () => {},
    toggleTimeline: () => {},
    focus: () => true,
    setMotion: () => {},
    setFacets: () => {},
    getFacets: () => ({}),
    clearFacets: () => {},
    exportSceneJSON: () => ({ metadata: { version: 4.6, type: "Object", generator: "test" }, object: {} }),
    inspectorTree: () => ({ name: "c3-world", type: "Group", visible: true, parts: 0, children: [] }),
    setSkin: () => true,
    skinId: () => "industrial",
    ...overrides,
  };
}

// The browser anti-goal checks (and cli/cmd/explore_test.go) call this exact surface; a
// rename here is a silent break of the verification pipeline, so the shape is pinned by test.
describe("C3_EXPLORER API surface", () => {
  it("keeps every pre-city member with the exact vanilla names", () => {
    const api = buildExplorerAPI(stubHandle());
    expect(api.ready).toBe(true);
    expect(api.renderedNodeIds()).toEqual(["c3-0", "c3-1"]);
    expect(api.allDataNodeIds()).toEqual(["c3-0", "c3-1"]);
    expect(api.renderedEdgeCount()).toBe(1);
    expect(api.dataEdgeCount()).toBe(1);
    expect(api.nodesWithoutStatus()).toEqual([]);
    expect(api.selectNodeById("c3-0")).toBe(true);
    expect(api.currentSelection()).toBeNull();
    expect(typeof api.setLevel).toBe("function");
    expect(api.cameraPosition()).toEqual({ x: 6, y: 74, z: 88 });
    expect(api.visibleNodeIds()).toEqual(["c3-0", "c3-1"]);
    expect(api.timeline.active()).toBe(false);
    expect(api.timeline.eventCount()).toBe(3);
    expect(api.timeline.index()).toBe(0);
    expect(typeof api.timeline.play).toBe("function");
    expect(typeof api.timeline.pause).toBe("function");
    expect(api.timeline.visibleNodeIds()).toEqual(["c3-0", "c3-1"]);
    expect(typeof api.timeline.toggle).toBe("function");
  });

  it("adds the payload-v2 members: focus, setMotion, facets.*, exportSceneJSON, inspector.tree", () => {
    const focus = vi.fn(() => true);
    const setMotion = vi.fn();
    const setFacets = vi.fn();
    const clearFacets = vi.fn();
    const api = buildExplorerAPI(stubHandle({ focus, setMotion, setFacets, clearFacets, getFacets: () => ({ type: ["component"] }) }));
    expect(api.focus("c3-1")).toBe(true);
    expect(focus).toHaveBeenCalledWith("c3-1");
    api.setMotion("all");
    expect(setMotion).toHaveBeenCalledWith("all");
    api.facets.set({ district: ["c3-1"] });
    expect(setFacets).toHaveBeenCalledWith({ district: ["c3-1"] });
    expect(api.facets.get()).toEqual({ type: ["component"] });
    api.facets.clear();
    expect(clearFacets).toHaveBeenCalled();
    expect(api.exportSceneJSON().metadata.type).toBe("Object");
    expect(api.inspector.tree().name).toBe("c3-world");
  });

  it("adds the skin members: setSkin(id) and skin()", () => {
    const setSkin = vi.fn(() => true);
    const api = buildExplorerAPI(stubHandle({ setSkin, skinId: () => "blueprint" }));
    expect(api.setSkin("blueprint")).toBe(true);
    expect(setSkin).toHaveBeenCalledWith("blueprint");
    expect(api.skin()).toBe("blueprint");
  });

  it("timeline.goTo activates the mode, clamps the index, and reports success", () => {
    const goToEvent = vi.fn();
    const toggleTimeline = vi.fn();
    const api = buildExplorerAPI(stubHandle({ goToEvent, toggleTimeline }));
    expect(api.timeline.goTo(99)).toBe(true);
    expect(toggleTimeline).toHaveBeenCalledWith(true);
    expect(goToEvent).toHaveBeenCalledWith(2);
  });

  it("timeline.goTo returns false when there are no events", () => {
    const api = buildExplorerAPI(stubHandle({ events: () => [] }));
    expect(api.timeline.goTo(0)).toBe(false);
  });

  it("timeline.visibleNodeIds reflects what the scrubbed time has created", () => {
    const api = buildExplorerAPI(stubHandle({ visibleNodeIds: () => ["c3-0"] }));
    expect(api.timeline.visibleNodeIds()).toEqual(["c3-0"]);
  });
});
