import { describe, expect, it } from "vitest";
import { diffPayload } from "../scene/liveDiff";
import type { C3Payload } from "../data";

function payload(nodes: Partial<C3Payload["nodes"][number]>[]): C3Payload {
  return {
    project: "t",
    generatedAt: "2026-01-01T00:00:00Z",
    nodes: nodes.map((n) => ({
      id: "x",
      type: "component",
      title: "x",
      level: "component",
      ring: "service",
      lifecycle: "frozen",
      ...n,
    })) as C3Payload["nodes"],
    edges: [],
    events: [{ id: "genesis", date: "0000-00-00", title: "g", status: "done" }] as C3Payload["events"],
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
    const next = payload([{ id: "a", staged: true, lifecycle: "staged" }]);
    expect(diffPayload(prev, next).changed).toEqual(["a"]);
  });

  it("identical payloads produce an empty diff", () => {
    const prev = payload([{ id: "a" }, { id: "b", parent: "a" }]);
    const next = payload([{ id: "a" }, { id: "b", parent: "a" }]);
    expect(diffPayload(prev, next)).toEqual({ added: [], removed: [], changed: [] });
  });
});
