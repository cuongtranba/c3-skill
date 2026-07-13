import { describe, expect, it, vi } from "vitest";
import { buildExplorerAPI, type ExplorerHandle } from "../api/explorerAPI";

function stubHandle(overrides: Partial<ExplorerHandle> = {}): ExplorerHandle {
  return {
    ready: true,
    renderedNodeIds: () => ["c3-0", "c3-1"],
    allDataNodeIds: () => ["c3-0", "c3-1"],
    renderedEdgeCount: () => 1,
    dataEdgeCount: () => 1,
    nodesWithoutStatus: () => [],
    selectNodeById: () => true,
    setLevel: () => {},
    currentSelection: () => null,
    timelineActive: () => false,
    timelineIndex: () => 0,
    events: () => [{}, {}, {}],
    goToEvent: () => {},
    tlPlay: () => {},
    tlPause: () => {},
    toggleTimeline: () => {},
    ...overrides,
  };
}

// The browser anti-goal checks call this exact surface; a rename here is a
// silent break of the verification pipeline, so the shape is pinned by test.
describe("C3_EXPLORER API surface", () => {
  it("exposes every verification member with the exact vanilla names", () => {
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
    expect(api.timeline.active()).toBe(false);
    expect(api.timeline.eventCount()).toBe(3);
    expect(api.timeline.index()).toBe(0);
    expect(typeof api.timeline.play).toBe("function");
    expect(typeof api.timeline.pause).toBe("function");
    expect(api.timeline.visibleNodeIds()).toEqual(["c3-0", "c3-1"]);
    expect(typeof api.timeline.toggle).toBe("function");
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
});
