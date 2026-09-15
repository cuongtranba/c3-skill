import type { Avenue, C3Edge, C3Node, C3Payload, Dock, Route, Street, Waypoint } from "../types";

/* Dev fixture for `npm run dev` when window.C3_DATA is absent. Districts, node layouts and
 * roads are hand-planned to the spec's city-layout rules (§1–4); docks and route waypoints
 * are derived by the small planner below, which mirrors §5–6 (dock faces, street/avenue
 * graph, Dijkstra, lanes) so the fixture stays geometrically honest as edges change.
 * Nothing here ships: the product takes all of this from the Go payload. */

type Partial3 = Omit<C3Node, "level" | "lifecycle" | "staged" | "boundaries" | "importance" | "docks" | "district"> &
  Partial<Pick<C3Node, "level" | "lifecycle" | "staged" | "boundaries" | "importance" | "district">>;

const IMPORTANCE: Record<string, number> = {
  headquarters: 1.0,
  command: 1.0,
  comms: 0.8,
  bunker: 0.75,
  plant: 0.6,
  transmit: 0.6,
  gatehouse: 0.5,
  outpost: 0.3,
  record: 0.2,
  zone: 0,
  route: 0,
};

function N(o: Partial3): C3Node {
  return {
    level: "component",
    lifecycle: "frozen",
    staged: false,
    boundaries: [],
    district: "c3-1",
    importance: IMPORTANCE[o.archetype] ?? 0.6,
    docks: [],
    ...o,
  };
}

/* Sector 01 (container c3-1): five dependency rows, pitch 32 (max footprint d 18 + 14),
 * pad 10 → d = 180. Widest row (c3-112 22w, c3-114 14w, spaced 22*2.5 = 55) → w = 97. */
const SECTOR = { x: 0, z: 0, w: 97, d: 180, y: 0.6 };
const TOP = SECTOR.z - SECTOR.d / 2;
const BOTTOM = SECTOR.z + SECTOR.d / 2;
const LEFT = SECTOR.x - SECTOR.w / 2;
const RIGHT = SECTOR.x + SECTOR.w / 2;
const TRUNK_Z = TOP - 12;
const GOV_TRENCH_Z = BOTTOM + 12;
const ROW_Z = [-64, -32, 0, 32, 64];
const STREET_Z = [-80, -48, -16, 16, 48, 80];
const HQ = { x: 0, z: TOP - 60, w: 50, d: 36, y: 1.1 };
const GOV = { x: 0, z: BOTTOM + 50, w: 60, d: 24, y: 0 };

const nodes: C3Node[] = [
  N({ id: "c3-0", type: "system", title: "c3-design", goal: "The C3 product: a Go CLI and the Claude skill that ships it.", level: "context", archetype: "headquarters", district: "hq", tech: "Go · TypeScript", statusKey: "stable", eval: { verdict: "holds" }, code: { globs: ["cli/**", "skills/**"], files: 212, loc: 41000 }, layout: { x: HQ.x, y: HQ.y, z: HQ.z, w: 26, d: 20 } }),
  N({ id: "c3-1", type: "container", title: "cli", goal: "Go CLI: every c3x command, the store and the changeset engine.", parent: "c3-0", level: "container", archetype: "gatehouse", tech: "Go", statusKey: "stable", eval: { verdict: "holds" }, code: { globs: ["cli/**"], files: 180, loc: 36000 }, layout: { x: LEFT + 6, y: SECTOR.y, z: TOP + 5, w: 7, d: 7 } }),
  N({ id: "c3-109", type: "component", title: "dispatcher", goal: "Route every c3x invocation to its command and stamp the activity trail.", parent: "c3-1", archetype: "comms", category: "core", tech: "Go", statusKey: "stable", code: { globs: ["cli/main.go"], files: 2, loc: 900 }, eval: { verdict: "holds" }, layout: { x: 0, y: SECTOR.y, z: ROW_Z[0], w: 9, d: 9 } }),
  N({ id: "c3-112", type: "component", title: "change-cmds", goal: "Drive the change-unit saga: new, view, accept, apply, rebase.", parent: "c3-1", archetype: "command", category: "feature", tech: "Go", statusKey: "changing", lifecycle: "staged", staged: true, stagedBy: ["adr-20260915-c3v-visualize"], transition: { from: "frozen", to: "changing", by: "adr-20260915-c3v-visualize" }, boundaries: ["boundary-store-write"], code: { globs: ["cli/cmd/change*.go"], files: 9, loc: 3100 }, eval: { verdict: "holds" }, layout: { x: -27.5, y: SECTOR.y, z: ROW_Z[1], w: 22, d: 17 } }),
  N({ id: "c3-114", type: "component", title: "explore-cmd", goal: "Project the live model into a self-contained 3D explorer.", parent: "c3-1", archetype: "transmit", category: "feature", tech: "Go · TypeScript", statusKey: "stable", code: { globs: ["cli/cmd/explore*.go", "explorer-app/src/**"], files: 31, loc: 5400 }, eval: { verdict: "holds" }, layout: { x: 27.5, y: SECTOR.y, z: ROW_Z[1], w: 14, d: 10 } }),
  N({ id: "c3-104", type: "component", title: "changeset", goal: "Parse, validate and atomically apply patch folders.", parent: "c3-1", archetype: "plant", category: "core", tech: "Go", statusKey: "drift", boundaries: ["boundary-store-write"], code: { globs: ["cli/internal/changeset/**"], files: 14, loc: 2600 }, eval: { verdict: "drift" }, layout: { x: 0, y: SECTOR.y, z: ROW_Z[2], w: 10, d: 10 } }),
  N({ id: "c3-102", type: "component", title: "store", goal: "Own the disposable SQLite cache of entities, relationships and nodes.", parent: "c3-1", archetype: "bunker", category: "infra", tech: "Go", statusKey: "stable", boundaries: ["boundary-store-write"], code: { globs: ["cli/internal/store/**"], files: 11, loc: 2200 }, eval: { verdict: "holds" }, layout: { x: 0, y: SECTOR.y, z: ROW_Z[3], w: 18, d: 18 } }),
  N({ id: "c3-103", type: "component", title: "schema", goal: "Canvas definitions and REJECT-IF validation.", parent: "c3-1", archetype: "plant", category: "core", tech: "Go", statusKey: "review", code: { globs: ["cli/internal/schema/**"], files: 8, loc: 1100 }, eval: { verdict: "needs-judgement" }, layout: { x: 0, y: SECTOR.y, z: ROW_Z[4], w: 10, d: 10 } }),
  N({ id: "rule-wrap-error-cause", type: "rule", title: "wrap error cause", goal: "Every failure crossing a boundary wraps its cause with %w.", archetype: "outpost", district: "governance", statusKey: "governance", layout: { x: -7.5, y: GOV.y, z: GOV.z, w: 5, d: 5 } }),
  N({ id: "ref-frontmatter-docs", type: "ref", title: "frontmatter docs", goal: "Why facts carry YAML frontmatter.", archetype: "outpost", district: "governance", statusKey: "governance", layout: { x: 7.5, y: GOV.y, z: GOV.z, w: 5, d: 5 } }),
  // Zone: bbox of c3-112, c3-104, c3-102 footprints padded by 4.
  N({ id: "boundary-store-write", type: "boundary", title: "store write path", goal: "Only these components may write to the store.", archetype: "zone", boundaryKind: "security", district: "", statusKey: "unchecked", layout: { x: -14.75, y: SECTOR.y, z: 0.25, w: 55.5, d: 89.5 } }),
  // Route: a 2×2 spot 3 units west of the first step's source dock (filled in after docks are planned).
  N({ id: "flow-change-apply", type: "flow", title: "change apply", goal: "How a change-unit reaches the store.", archetype: "route", district: "", statusKey: "unchecked", layout: { x: 0, y: SECTOR.y, z: 0, w: 2, d: 2 } }),
];

const E = (from: string, to: string, kind: string, extra: Partial<C3Edge> = {}): C3Edge => ({ id: `${from}→${to}#${kind}`, from, to, kind, ...extra });

const edges: C3Edge[] = [
  E("c3-0", "c3-1", "contains"),
  ...["c3-109", "c3-112", "c3-114", "c3-104", "c3-102", "c3-103"].map((id) => E("c3-1", id, "contains")),
  E("c3-109", "c3-112", "depends_on", { label: "dispatch · in-proc" }),
  E("c3-109", "c3-114", "depends_on", { label: "dispatch · in-proc" }),
  E("c3-112", "c3-104", "depends_on", { label: "apply unit" }),
  E("c3-112", "c3-102", "depends_on", { label: "read entities" }),
  E("c3-104", "c3-103", "depends_on", { label: "validate canvas" }),
  E("c3-104", "c3-102", "depends_on", { label: "write nodes" }),
  E("c3-102", "c3-103", "depends_on", { label: "canvas lookup" }),
  E("c3-114", "c3-102", "depends_on", { label: "read entities" }),
  E("c3-112", "rule-wrap-error-cause", "uses"),
  E("c3-104", "rule-wrap-error-cause", "uses"),
  E("c3-114", "rule-wrap-error-cause", "uses"),
  E("c3-103", "ref-frontmatter-docs", "uses"),
  E("boundary-store-write", "c3-112", "encloses"),
  E("boundary-store-write", "c3-104", "encloses"),
  E("boundary-store-write", "c3-102", "encloses"),
  E("c3-109", "c3-112", "flow_step", { flow: "flow-change-apply", seq: 1, label: "dispatch" }),
  E("c3-112", "c3-104", "flow_step", { flow: "flow-change-apply", seq: 2, label: "apply unit" }),
  E("c3-104", "c3-102", "flow_step", { flow: "flow-change-apply", seq: 3, label: "write nodes" }),
];

const streets: Street[] = [
  ...STREET_Z.map((z, i) => ({ id: `c3-1:street:${i}`, district: "c3-1", z, x0: LEFT - 3, x1: RIGHT + 3, width: 3, major: false })),
  { id: "main-trunk", district: "", z: TRUNK_Z, x0: -140, x1: 140, width: 4.2, major: true },
  { id: "governance-trench", district: "", z: GOV_TRENCH_Z, x0: -140, x1: 140, width: 4.2, major: true },
];
const avenues: Avenue[] = [
  { id: "c3-1:avenue:w", district: "c3-1", x: LEFT - 3, z0: TRUNK_Z, z1: GOV_TRENCH_Z, width: 2.2 },
  { id: "c3-1:avenue:e", district: "c3-1", x: RIGHT + 3, z0: TRUNK_Z, z1: GOV_TRENCH_Z, width: 2.2 },
  { id: "hq:avenue:w", district: "hq", x: HQ.x - 28, z0: HQ.z + HQ.d / 2, z1: TRUNK_Z, width: 2.2 },
  { id: "hq:avenue:e", district: "hq", x: HQ.x + 28, z0: HQ.z + HQ.d / 2, z1: TRUNK_Z, width: 2.2 },
];

/* ─── planner: docks (§5) and routes (§6) ─────────────────────────────────────── */

const byId = new Map(nodes.map((n) => [n.id, n]));
const UNROUTED = new Set(["contains", "encloses", "flow_from", "flow_to"]);
const routable = edges.filter((e) => !UNROUTED.has(e.kind));
const dependsKey = (e: C3Edge): string => `${e.from}→${e.to}`;
const dependsRoutes = new Map(routable.filter((e) => e.kind === "depends_on").map((e) => [dependsKey(e), e]));
const sharedOf = (e: C3Edge): C3Edge | undefined => (e.kind === "flow_step" ? dependsRoutes.get(dependsKey(e)) : undefined);

const faceFor = (self: C3Node, other: C3Node): "n" | "s" => (other.layout.z >= self.layout.z ? "s" : "n");

interface Pending {
  edge: C3Edge;
  srcFace: "n" | "s";
  dstFace: "n" | "s";
}
const pending: Pending[] = routable
  .filter((e) => !sharedOf(e))
  .map((e) => {
    const a = byId.get(e.from) as C3Node;
    const b = byId.get(e.to) as C3Node;
    return { edge: e, srcFace: faceFor(a, b), dstFace: faceFor(b, a) };
  });

const docksByNodeFace = new Map<string, { edgeId: string }[]>();
for (const p of pending) {
  const push = (id: string, face: string, edgeId: string): void => {
    const k = `${id}|${face}`;
    if (!docksByNodeFace.has(k)) docksByNodeFace.set(k, []);
    (docksByNodeFace.get(k) as { edgeId: string }[]).push({ edgeId });
  };
  push(p.edge.from, p.srcFace, p.edge.id);
  push(p.edge.to, p.dstFace, p.edge.id);
}
const dockByEdgeEnd = new Map<string, Dock>();
for (const [k, list] of docksByNodeFace) {
  const [id, face] = k.split("|") as [string, "n" | "s"];
  const n = byId.get(id) as C3Node;
  list.sort((a, b) => a.edgeId.localeCompare(b.edgeId));
  list.forEach((d, i) => {
    const x = n.layout.x + (i - (list.length - 1) / 2) * 1.8;
    const z = n.layout.z + (face === "s" ? 1 : -1) * (n.layout.d / 2 + 1.4);
    const dock: Dock = { id: `${id}:${face}:${i}`, face, x, z, edgeId: d.edgeId };
    (n.docks as Dock[]).push(dock);
    dockByEdgeEnd.set(`${d.edgeId}|${id}`, dock);
  });
}

// Street/avenue graph.
type V = { key: string; x: number; z: number };
const vkey = (x: number, z: number): string => `${x.toFixed(2)},${z.toFixed(2)}`;
const verts = new Map<string, V>();
const adj = new Map<string, { to: string; w: number; seg: string }[]>();
const addV = (x: number, z: number): V => {
  const key = vkey(x, z);
  if (!verts.has(key)) {
    verts.set(key, { key, x, z });
    adj.set(key, []);
  }
  return verts.get(key) as V;
};
const onStreet = new Map<string, V[]>();
const onAvenue = new Map<string, V[]>();
for (const s of streets) for (const a of avenues) {
  if (a.x >= s.x0 && a.x <= s.x1 && s.z >= Math.min(a.z0, a.z1) && s.z <= Math.max(a.z0, a.z1)) {
    const v = addV(a.x, s.z);
    (onStreet.get(s.id) ?? onStreet.set(s.id, []).get(s.id))?.push(v);
    (onAvenue.get(a.id) ?? onAvenue.set(a.id, []).get(a.id))?.push(v);
  }
}
const streetForDock = (dock: Dock): Street => {
  const cands = streets.filter((s) => dock.x >= s.x0 && dock.x <= s.x1 && (dock.face === "s" ? s.z > dock.z : s.z < dock.z));
  cands.sort((a, b) => Math.abs(a.z - dock.z) - Math.abs(b.z - dock.z));
  if (!cands.length) throw new Error(`fixture: no street for dock ${dock.id}`);
  return cands[0];
};
const projections = new Map<string, { v: V; street: Street }>();
for (const p of pending) {
  for (const id of [p.edge.from, p.edge.to]) {
    const dock = dockByEdgeEnd.get(`${p.edge.id}|${id}`) as Dock;
    const s = streetForDock(dock);
    const v = addV(dock.x, s.z);
    (onStreet.get(s.id) ?? onStreet.set(s.id, []).get(s.id))?.push(v);
    projections.set(dock.id, { v, street: s });
  }
}
const link = (list: V[], seg: string, axis: "x" | "z"): void => {
  const sorted = [...new Set(list)].sort((a, b) => a[axis] - b[axis]);
  for (let i = 0; i + 1 < sorted.length; i++) {
    const a = sorted[i];
    const b = sorted[i + 1];
    const w = Math.abs(a[axis] - b[axis]);
    if (w < 1e-6) continue;
    adj.get(a.key)?.push({ to: b.key, w, seg });
    adj.get(b.key)?.push({ to: a.key, w, seg });
  }
};
for (const [id, list] of onStreet) link(list, id, "x");
for (const [id, list] of onAvenue) link(list, id, "z");

function dijkstra(from: string, to: string): { path: V[]; segs: string[] } {
  const dist = new Map<string, number>([[from, 0]]);
  const prev = new Map<string, { key: string; seg: string }>();
  const open = new Set<string>([from]);
  while (open.size) {
    let cur = "";
    let best = Infinity;
    for (const k of open) {
      const d = dist.get(k) ?? Infinity;
      if (d < best) {
        best = d;
        cur = k;
      }
    }
    open.delete(cur);
    if (cur === to) break;
    for (const e of adj.get(cur) ?? []) {
      const nd = best + e.w;
      if (nd < (dist.get(e.to) ?? Infinity)) {
        dist.set(e.to, nd);
        prev.set(e.to, { key: cur, seg: e.seg });
        open.add(e.to);
      }
    }
  }
  if (!dist.has(to)) throw new Error(`fixture: no route ${from} → ${to}`);
  const path: V[] = [];
  const segs: string[] = [];
  for (let k: string | undefined = to; k; k = prev.get(k)?.key) {
    path.unshift(verts.get(k) as V);
    const p = prev.get(k);
    if (p && (!segs.length || segs[0] !== p.seg)) segs.unshift(p.seg);
  }
  return { path, segs };
}

const collinear = (a: Waypoint, b: Waypoint, c: Waypoint): boolean => (Math.abs(a[0] - b[0]) < 1e-6 && Math.abs(b[0] - c[0]) < 1e-6) || (Math.abs(a[2] - b[2]) < 1e-6 && Math.abs(b[2] - c[2]) < 1e-6);

const isStaged = (id: string): boolean => !!byId.get(id)?.staged;
const flowSteps = [
  ["c3-109", "c3-112"],
  ["c3-112", "c3-104"],
  ["c3-104", "c3-102"],
];
const consecutive = (e: C3Edge): boolean => flowSteps.some(([a, b]) => a === e.from && b === e.to);

interface Planned {
  route: Route;
  segUse: { seg: string; horizontal: boolean }[];
}
const planned: Planned[] = pending.map((p) => {
  const e = p.edge;
  const dA = dockByEdgeEnd.get(`${e.id}|${e.from}`) as Dock;
  const dB = dockByEdgeEnd.get(`${e.id}|${e.to}`) as Dock;
  const pa = projections.get(dA.id) as { v: V; street: Street };
  const pb = projections.get(dB.id) as { v: V; street: Street };
  const { path, segs } = dijkstra(pa.v.key, pb.v.key);
  const nA = byId.get(e.from) as C3Node;
  const nB = byId.get(e.to) as C3Node;
  const pts: Waypoint[] = [[dA.x, nA.layout.y + 0.45, dA.z], ...path.map((v): Waypoint => [v.x, 0.16, v.z]), [dB.x, nB.layout.y + 0.45, dB.z]];
  const cleaned: Waypoint[] = [pts[0]];
  for (let i = 1; i < pts.length - 1; i++) if (!collinear(cleaned[cleaned.length - 1], pts[i], pts[i + 1])) cleaned.push(pts[i]);
  cleaned.push(pts[pts.length - 1]);
  const segUse = segs.map((seg) => ({ seg, horizontal: streets.some((s) => s.id === seg) }));
  return {
    route: {
      id: e.id,
      edgeId: e.id,
      kind: e.kind,
      active: e.kind === "flow_step" || (e.kind === "depends_on" && consecutive(e)) || isStaged(e.from),
      segments: segs,
      lane: 0,
      sharedWith: "",
      waypoints: cleaned,
    },
    segUse,
  };
});

// Lanes: routes sharing a segment get an index by route-id order; perpendicular offset on interior points.
const laneUsers = new Map<string, string[]>();
for (const p of planned) for (const s of p.segUse) (laneUsers.get(s.seg) ?? laneUsers.set(s.seg, []).get(s.seg))?.push(p.route.id);
for (const list of laneUsers.values()) list.sort();
const laneOf = (seg: string, rid: string): number => {
  const list = laneUsers.get(seg) ?? [rid];
  return (list.indexOf(rid) - (list.length - 1) / 2) * 0.7;
};
for (const p of planned) {
  const wps = p.route.waypoints;
  let laneMax = 0;
  for (let i = 1; i < wps.length - 1; i++) {
    const wp = wps[i];
    const street = streets.find((s) => Math.abs(s.z - wp[2]) < 1e-6 && p.segUse.some((u) => u.seg === s.id));
    const avenue = avenues.find((a) => Math.abs(a.x - wp[0]) < 1e-6 && p.segUse.some((u) => u.seg === a.id));
    if (street) {
      const off = laneOf(street.id, p.route.id);
      wp[2] += off;
      laneMax = Math.abs(off) > Math.abs(laneMax) ? off : laneMax;
    }
    if (avenue) {
      const off = laneOf(avenue.id, p.route.id);
      wp[0] += off;
      laneMax = Math.abs(off) > Math.abs(laneMax) ? off : laneMax;
    }
  }
  p.route.lane = laneMax;
}

const routes: Route[] = planned.map((p) => p.route);
for (const e of routable) {
  const shared = sharedOf(e);
  if (!shared) continue;
  const base = routes.find((r) => r.edgeId === shared.id) as Route;
  routes.push({ id: e.id, edgeId: e.id, kind: e.kind, active: true, segments: base.segments, lane: base.lane, sharedWith: base.id, waypoints: base.waypoints.map((w) => [...w] as Waypoint) });
}
routes.sort((a, b) => a.id.localeCompare(b.id));

// Flow signpost: 3 units west of the first step's source dock.
{
  const first = routes.find((r) => r.kind === "flow_step" && r.edgeId.startsWith("c3-109→c3-112")) as Route;
  const flow = byId.get("flow-change-apply") as C3Node;
  flow.layout = { x: first.waypoints[0][0] - 3, y: SECTOR.y, z: first.waypoints[0][2], w: 2, d: 2 };
}

export const FIXTURE: C3Payload = {
  schemaVersion: 2,
  project: "c3-design",
  generatedAt: "2026-09-15T10:00:00Z",
  nodes,
  edges,
  flows: [
    {
      id: "flow-change-apply",
      title: "change apply",
      steps: [
        { seq: 1, from: "c3-109", to: "c3-112", action: "dispatch" },
        { seq: 2, from: "c3-112", to: "c3-104", action: "apply unit" },
        { seq: 3, from: "c3-104", to: "c3-102", action: "write nodes" },
      ],
    },
  ],
  boundaries: [{ id: "boundary-store-write", kind: "security", parent: "", members: ["c3-112", "c3-104", "c3-102"] }],
  districts: [
    { id: "c3-1", title: "SECTOR 01 · CLI", kind: "sector", ...SECTOR, members: ["c3-1", "c3-109", "c3-112", "c3-114", "c3-104", "c3-102", "c3-103"] },
    { id: "hq", title: "HQ · C3-DESIGN", kind: "hq", ...HQ, members: ["c3-0"] },
    { id: "governance", title: "PERIMETER · GOVERNANCE", kind: "governance", ...GOV, members: ["rule-wrap-error-cause", "ref-frontmatter-docs"] },
  ],
  roads: { streets, avenues },
  routes,
  events: [
    { id: "genesis", date: "0000-00-00", title: "genesis", status: "done", creates: ["c3-0", "c3-1", "c3-109", "c3-112", "c3-104", "c3-103", "c3-102", "rule-wrap-error-cause", "ref-frontmatter-docs", "boundary-store-write"] },
    { id: "adr-20260710-add-visual-layer-explore", date: "2026-07-10", title: "add visual layer (explore)", status: "done", creates: ["c3-114"], modifies: ["c3-109"] },
    { id: "adr-20260915-c3v-visualize", date: "2026-09-15", title: "c3v visualize: infrastructure city", status: "open", creates: ["flow-change-apply"], modifies: ["c3-112", "c3-114"] },
  ],
};
