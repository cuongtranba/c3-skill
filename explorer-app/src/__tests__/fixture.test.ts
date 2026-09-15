import { describe, expect, it } from "vitest";
import { FIXTURE } from "../dev/fixture";

const byId = new Map(FIXTURE.nodes.map((n) => [n.id, n]));
const edgeById = new Map(FIXTURE.edges.map((e) => [e.id, e]));
const near = (a: number, b: number, eps = 1e-6): boolean => Math.abs(a - b) < eps;

// The fixture stands in for the Go payload, so it has to obey the same validator rules.
describe("dev fixture obeys the payload-v2 geometry contract", () => {
  it("node ids are unique and every node has a footprint, an archetype and a district when it needs one", () => {
    expect(new Set(FIXTURE.nodes.map((n) => n.id)).size).toBe(FIXTURE.nodes.length);
    for (const n of FIXTURE.nodes) {
      expect(n.layout.w).toBeGreaterThan(0);
      expect(n.layout.d).toBeGreaterThan(0);
      if (n.archetype === "zone" || n.archetype === "route") expect(n.district).toBe("");
      else expect(FIXTURE.districts.some((d) => d.id === n.district)).toBe(true);
    }
  });

  it("edge endpoints, flow ids and boundary references resolve", () => {
    for (const e of FIXTURE.edges) {
      expect(byId.has(e.from)).toBe(true);
      expect(byId.has(e.to)).toBe(true);
      if (e.kind === "flow_step") expect(FIXTURE.flows.some((f) => f.id === e.flow)).toBe(true);
    }
    for (const f of FIXTURE.flows) {
      for (let i = 1; i < f.steps.length; i++) expect(f.steps[i].seq).toBeGreaterThan(f.steps[i - 1].seq);
    }
    for (const b of FIXTURE.boundaries) expect(byId.get(b.id)?.type).toBe("boundary");
    for (const n of FIXTURE.nodes) for (const b of n.boundaries ?? []) expect(byId.get(b)?.type).toBe("boundary");
  });

  it("every routed edge has exactly one route that starts at the source dock and ends at the target dock", () => {
    const unrouted = new Set(["contains", "encloses", "flow_from", "flow_to"]);
    const routable = FIXTURE.edges.filter((e) => !unrouted.has(e.kind));
    expect(FIXTURE.routes.length).toBe(routable.length);
    for (const e of routable) {
      const rs = FIXTURE.routes.filter((r) => r.edgeId === e.id);
      expect(rs.length).toBe(1);
      const r = rs[0];
      expect(r.waypoints.length).toBeGreaterThanOrEqual(2);
      const shared = r.sharedWith ? FIXTURE.routes.find((x) => x.id === r.sharedWith) : undefined;
      if (r.sharedWith) expect(shared).toBeDefined();
      const dockEdge = shared ? shared.edgeId : e.id;
      const src = byId.get(e.from)!;
      const dst = byId.get(e.to)!;
      const sd = src.docks!.find((d) => d.edgeId === dockEdge)!;
      const td = dst.docks!.find((d) => d.edgeId === dockEdge)!;
      expect(sd).toBeDefined();
      expect(td).toBeDefined();
      const [x0, y0, z0] = r.waypoints[0];
      const [x1, y1, z1] = r.waypoints[r.waypoints.length - 1];
      expect(near(x0, sd.x) && near(z0, sd.z)).toBe(true);
      expect(near(x1, td.x) && near(z1, td.z)).toBe(true);
      expect(near(y0, src.layout.y + 0.45)).toBe(true);
      expect(near(y1, dst.layout.y + 0.45)).toBe(true);
    }
  });

  it("interior waypoints lie on a street or avenue centreline offset by at most the lane spread", () => {
    const streets = FIXTURE.roads.streets;
    const avenues = FIXTURE.roads.avenues;
    for (const r of FIXTURE.routes) {
      for (const [x, y, z] of r.waypoints.slice(1, -1)) {
        expect(near(y, 0.16)).toBe(true);
        const onStreet = streets.some((s) => Math.abs(s.z - z) <= 2.5 && x >= s.x0 - 2.5 && x <= s.x1 + 2.5);
        const onAvenue = avenues.some((a) => Math.abs(a.x - x) <= 2.5 && z >= Math.min(a.z0, a.z1) - 2.5 && z <= Math.max(a.z0, a.z1) + 2.5);
        expect(onStreet || onAvenue).toBe(true);
      }
      for (const seg of r.segments) expect(streets.some((s) => s.id === seg) || avenues.some((a) => a.id === seg)).toBe(true);
    }
  });

  it("docks sit 1.4 outside the face, spread 1.8 apart, y = layout.y + 0.45, and reference existing edges", () => {
    for (const n of FIXTURE.nodes) {
      const byFace = new Map<string, number[]>();
      for (const d of n.docks ?? []) {
        expect(edgeById.has(d.edgeId)).toBe(true);
        const faceZ = n.layout.z + (d.face === "s" ? 1 : -1) * (n.layout.d / 2 + 1.4);
        expect(near(d.z, faceZ)).toBe(true);
        byFace.set(d.face, [...(byFace.get(d.face) ?? []), d.x]);
      }
      for (const xs of byFace.values()) {
        xs.sort((a, b) => a - b);
        for (let i = 1; i < xs.length; i++) expect(near(xs[i] - xs[i - 1], 1.8)).toBe(true);
        expect(near(xs.reduce((a, b) => a + b, 0) / xs.length, n.layout.x)).toBe(true);
      }
    }
  });

  it("flow_step routes share the depends_on route between the same endpoints and are active", () => {
    const steps = FIXTURE.routes.filter((r) => r.kind === "flow_step");
    expect(steps.length).toBe(3);
    for (const s of steps) {
      expect(s.active).toBe(true);
      expect(s.sharedWith).toMatch(/#depends_on$/);
      const base = FIXTURE.routes.find((r) => r.id === s.sharedWith)!;
      expect(base.waypoints).toEqual(s.waypoints);
      expect(base.active).toBe(true);
    }
    // The staged node's outgoing routes are active; an unrelated one is not.
    expect(FIXTURE.routes.find((r) => r.id === "c3-112→rule-wrap-error-cause#uses")?.active).toBe(true);
    expect(FIXTURE.routes.find((r) => r.id === "c3-114→c3-102#depends_on")?.active).toBe(false);
  });

  it("the zone is its members' bounding box padded by 4 and the flow signpost sits 3 west of the first step's source dock", () => {
    const zone = byId.get("boundary-store-write")!;
    const members = FIXTURE.boundaries[0].members.map((m) => byId.get(m)!);
    const minX = Math.min(...members.map((m) => m.layout.x - m.layout.w / 2)) - 4;
    const maxX = Math.max(...members.map((m) => m.layout.x + m.layout.w / 2)) + 4;
    const minZ = Math.min(...members.map((m) => m.layout.z - m.layout.d / 2)) - 4;
    const maxZ = Math.max(...members.map((m) => m.layout.z + m.layout.d / 2)) + 4;
    expect(near(zone.layout.x, (minX + maxX) / 2)).toBe(true);
    expect(near(zone.layout.w, maxX - minX)).toBe(true);
    expect(near(zone.layout.z, (minZ + maxZ) / 2)).toBe(true);
    expect(near(zone.layout.d, maxZ - minZ)).toBe(true);
    const flow = byId.get("flow-change-apply")!;
    const first = FIXTURE.routes.find((r) => r.id === "c3-109→c3-112#flow_step")!;
    expect(near(flow.layout.x, first.waypoints[0][0] - 3)).toBe(true);
    expect(near(flow.layout.z, first.waypoints[0][2])).toBe(true);
  });
});
