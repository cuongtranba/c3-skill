import { describe, expect, it } from "vitest";
import { FIXTURE } from "../dev/fixture";
import { neverCreated, timelineVisibleSet } from "../scene/timeline";
import type { C3Node } from "../types";

const adr = (id: string): C3Node => ({
  id,
  type: "adr",
  title: id,
  level: "component",
  lifecycle: "open",
  statusKey: "open",
  archetype: "record",
  district: "governance",
  importance: 0.2,
  layout: { x: 0, y: 0, z: 0, w: 3, d: 3 },
});

describe("timelineVisibleSet", () => {
  it("at genesis only genesis-created nodes exist; later nodes are hidden", () => {
    const v = timelineVisibleSet(FIXTURE.events, FIXTURE.nodes, 0);
    expect(v.has("c3-109")).toBe(true);
    expect(v.has("c3-114")).toBe(false);
    expect(v.has("flow-change-apply")).toBe(false);
  });

  it("accumulates creations up to the scrubbed index", () => {
    expect(timelineVisibleSet(FIXTURE.events, FIXTURE.nodes, 1).has("c3-114")).toBe(true);
    const last = timelineVisibleSet(FIXTURE.events, FIXTURE.nodes, FIXTURE.events.length - 1);
    for (const n of FIXTURE.nodes) expect(last.has(n.id)).toBe(true);
  });

  it("clamps an index past the end and treats a negative one as nothing yet", () => {
    expect(timelineVisibleSet(FIXTURE.events, FIXTURE.nodes, 99).size).toBe(FIXTURE.nodes.length);
    expect(timelineVisibleSet(FIXTURE.events, FIXTURE.nodes, -1).size).toBe(0);
  });

  it("an ADR node appears with its own event; an ADR without an event is always present", () => {
    const nodes = [...FIXTURE.nodes, adr("adr-20260915-c3v-visualize"), adr("adr-orphan")];
    const v0 = timelineVisibleSet(FIXTURE.events, nodes, 0);
    expect(v0.has("adr-20260915-c3v-visualize")).toBe(false);
    expect(v0.has("adr-orphan")).toBe(true);
    const v2 = timelineVisibleSet(FIXTURE.events, nodes, 2);
    expect(v2.has("adr-20260915-c3v-visualize")).toBe(true);
  });

  it("neverCreated lists non-ADR nodes no event creates, so replay never loses them", () => {
    expect(neverCreated(FIXTURE.events, FIXTURE.nodes).size).toBe(0);
    const extra = { ...FIXTURE.nodes[0], id: "c3-999" };
    expect([...neverCreated(FIXTURE.events, [...FIXTURE.nodes, extra])]).toEqual(["c3-999"]);
  });
});
