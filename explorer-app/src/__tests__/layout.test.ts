import { describe, expect, it } from "vitest";
import { getVisibleGraph, layoutNodes } from "../scene/layout";
import type { C3Payload } from "../data";

const payload: C3Payload = {
  project: "t",
  generatedAt: "2026-01-01T00:00:00Z",
  nodes: [
    { id: "c3-0", type: "system", title: "sys", level: "context", ring: "core", lifecycle: "frozen" },
    { id: "c3-1", type: "container", title: "cont", parent: "c3-0", level: "container", ring: "platform", lifecycle: "frozen" },
    { id: "c3-101", type: "component", title: "comp", parent: "c3-1", level: "component", ring: "service", lifecycle: "frozen" },
    { id: "ref-x", type: "ref", title: "ref", level: "component", ring: "infra", lifecycle: "frozen" },
  ],
  edges: [
    { from: "c3-0", to: "c3-1", kind: "contains" },
    { from: "c3-1", to: "c3-101", kind: "contains" },
    { from: "c3-101", to: "ref-x", kind: "uses" },
  ],
  events: [
    { id: "genesis", date: "0000-00-00", title: "genesis", status: "done", creates: ["c3-0", "c3-1", "c3-101", "ref-x"] },
  ],
};

// AG-1 wall: at the default C2 level every node in the data must be visible.
describe("getVisibleGraph", () => {
  it("container level shows the full graph — no node or edge is lost", () => {
    const g = getVisibleGraph(payload, "container", null, false, null);
    expect(g.nodes.map((n) => n.id).sort()).toEqual(["c3-0", "c3-1", "c3-101", "ref-x"].sort());
    expect(g.edges.length).toBe(3);
  });

  it("context level keeps system + containers only", () => {
    const g = getVisibleGraph(payload, "context", null, false, null);
    expect(g.nodes.map((n) => n.id).sort()).toEqual(["c3-0", "c3-1"]);
  });

  it("component level with focus follows uses edges one hop out", () => {
    const g = getVisibleGraph(payload, "component", "c3-1", false, null);
    expect(g.nodes.map((n) => n.id).sort()).toEqual(["c3-101", "ref-x"]);
  });

  it("timeline visible-set filter wins over level filtering", () => {
    const g = getVisibleGraph(payload, "container", null, true, new Set(["c3-0", "c3-1"]));
    expect(g.nodes.map((n) => n.id).sort()).toEqual(["c3-0", "c3-1"]);
    expect(g.edges.length).toBe(1);
  });
});

describe("layoutNodes", () => {
  it("places every node", () => {
    const g = getVisibleGraph(payload, "container", null, false, null);
    layoutNodes(g.nodes, g.edges);
    g.nodes.forEach((n) => {
      expect(n._x).toBeTypeOf("number");
      expect(n._z).toBeTypeOf("number");
    });
  });
});
