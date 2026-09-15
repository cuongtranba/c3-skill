import { describe, expect, it } from "vitest";
import { diffPayload } from "../scene/liveDiff";
import type { C3Node, C3Payload } from "../types";

function payload(nodes: Partial<C3Node>[]): C3Payload {
  return {
    schemaVersion: 2,
    project: "t",
    generatedAt: "2026-01-01T00:00:00Z",
    nodes: nodes.map((n) => ({
      id: "x",
      type: "component",
      title: "x",
      level: "component",
      lifecycle: "frozen",
      statusKey: "stable",
      archetype: "plant",
      district: "c3-1",
      importance: 0.6,
      layout: { x: 0, y: 0.6, z: 0, w: 10, d: 10 },
      ...n,
    })) as C3Node[],
    edges: [],
    flows: [],
    boundaries: [],
    districts: [],
    roads: { streets: [], avenues: [] },
    routes: [],
    events: [{ id: "genesis", date: "0000-00-00", title: "g", status: "done" }],
  };
}

describe("diffPayload", () => {
  it("detects added nodes", () => {
    const prev = payload([{ id: "a" }]);
    const next = payload([{ id: "a" }, { id: "b" }]);
    expect(diffPayload(prev, next)).toEqual({ added: ["b"], removed: [], changed: [] });
  });

  it("detects removed nodes", () => {
    const prev = payload([{ id: "a" }, { id: "b" }]);
    const next = payload([{ id: "a" }]);
    expect(diffPayload(prev, next).removed).toEqual(["b"]);
  });

  it("detects lifecycle change", () => {
    const prev = payload([{ id: "a", lifecycle: "open" }]);
    const next = payload([{ id: "a", lifecycle: "done" }]);
    expect(diffPayload(prev, next).changed).toEqual(["a"]);
  });

  it("detects staged flip", () => {
    const prev = payload([{ id: "a" }]);
    const next = payload([{ id: "a", staged: true, lifecycle: "staged", statusKey: "changing" }]);
    expect(diffPayload(prev, next).changed).toEqual(["a"]);
  });

  it("detects the city fields: statusKey, archetype, district, eval verdict, layout", () => {
    const base = payload([{ id: "a" }]);
    expect(diffPayload(base, payload([{ id: "a", statusKey: "drift" }])).changed).toEqual(["a"]);
    expect(diffPayload(base, payload([{ id: "a", archetype: "bunker" }])).changed).toEqual(["a"]);
    expect(diffPayload(base, payload([{ id: "a", district: "c3-2" }])).changed).toEqual(["a"]);
    expect(diffPayload(base, payload([{ id: "a", eval: { verdict: "drift" } }])).changed).toEqual(["a"]);
    expect(diffPayload(base, payload([{ id: "a", layout: { x: 5, y: 0.6, z: 0, w: 10, d: 10 } }])).changed).toEqual(["a"]);
    expect(diffPayload(base, payload([{ id: "a", boundaries: ["boundary-x"] }])).changed).toEqual(["a"]);
  });

  it("identical payloads produce an empty diff", () => {
    const prev = payload([{ id: "a" }, { id: "b", parent: "a", stagedBy: ["adr-1"] }]);
    const next = payload([{ id: "a" }, { id: "b", parent: "a", stagedBy: ["adr-1"] }]);
    expect(diffPayload(prev, next)).toEqual({ added: [], removed: [], changed: [] });
  });

  it("treats an omitted optional field and an empty one as equal", () => {
    const prev = payload([{ id: "a", tech: "" }]);
    const next = payload([{ id: "a" }]);
    expect(diffPayload(prev, next).changed).toEqual([]);
  });
});
