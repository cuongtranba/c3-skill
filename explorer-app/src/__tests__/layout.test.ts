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

// AG-1 wall: at the default "all" level every node in the data must be visible.
describe("getVisibleGraph", () => {
  it("all level shows the full graph — no node or edge is lost", () => {
    const g = getVisibleGraph(payload, "all", null, false, null);
    expect(g.nodes.map((n) => n.id).sort()).toEqual(["c3-0", "c3-1", "c3-101", "ref-x"].sort());
    expect(g.edges.length).toBe(3);
  });

  it("container level keeps system/containers/adrs and aggregates component wiring to parents", () => {
    const withRef2 = structuredClone(payload);
    withRef2.nodes.push({
      id: "c3-102", type: "component", title: "comp2", parent: "c3-2",
      level: "component", ring: "service", lifecycle: "frozen",
    });
    withRef2.nodes.push({
      id: "c3-2", type: "container", title: "cont2", parent: "c3-0",
      level: "container", ring: "platform", lifecycle: "frozen",
    });
    withRef2.edges.push({ from: "c3-101", to: "c3-102", kind: "uses" });
    const g = getVisibleGraph(withRef2, "container", null, false, null);
    expect(g.nodes.map((n) => n.id).sort()).toEqual(["c3-0", "c3-1", "c3-2"]);
    // c3-101 uses c3-102 becomes c3-1 uses c3-2; component->ref edge drops (ref has no parent).
    expect(g.edges).toContainEqual({ from: "c3-1", to: "c3-2", kind: "uses" });
    expect(g.edges.filter((e) => e.kind === "uses").length).toBe(1);
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
    const g = getVisibleGraph(payload, "all", null, false, null);
    layoutNodes(g.nodes, g.edges);
    g.nodes.forEach((n) => {
      expect(n._x).toBeTypeOf("number");
      expect(n._z).toBeTypeOf("number");
    });
  });
});
