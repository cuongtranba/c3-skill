import { RING_DEFS, hash } from "./constants";
import type { C3Edge, C3Payload, Level } from "../data";
import type { LNode } from "./sceneTypes";

export interface VisibleGraph {
  nodes: LNode[];
  edges: C3Edge[];
}

export function getVisibleGraph(
  data: C3Payload | undefined,
  level: Level,
  focusContainer: string | null,
  tlActive: boolean,
  tlVisibleSet: Set<string> | null,
): VisibleGraph {
  if (!data) return { nodes: [], edges: [] };

  const allNodes = (data.nodes || []) as LNode[];
  const allEdges = data.edges || [];

  let visNodes: LNode[];
  const extraEdges: C3Edge[] = [];

  // Timeline mode and the "all" level render the full graph — this is the
  // level the AG-1 no-lost-nodes check converges on.
  if (tlActive || level === "all") {
    visNodes = allNodes.slice();
  } else if (level === "context") {
    visNodes = allNodes.filter(
      (n) => n.level === "context" || n.type === "system" || n.type === "container",
    );
    const systemIds = new Set(visNodes.filter((n) => n.type === "system").map((n) => n.id));
    const ctxIds = new Set(visNodes.map((n) => n.id));
    allNodes
      .filter((n) => n.type === "container" && n.parent && systemIds.has(n.parent))
      .forEach((n) => ctxIds.add(n.id));
    visNodes = allNodes.filter((n) => ctxIds.has(n.id));
  } else if (level === "container") {
    // C2 = the C4 city view: system + containers + change-units. Component
    // wiring is not lost — every component-level edge is re-drawn between the
    // components' parent containers (aggregated, deduped).
    visNodes = allNodes.filter((n) => n.type === "system" || n.type === "container" || n.type === "adr");
    const visIds = new Set(visNodes.map((n) => n.id));
    const byId = new Map(allNodes.map((n) => [n.id, n]));
    const mapEnd = (id: string): string | null => {
      if (visIds.has(id)) return id;
      const parent = byId.get(id)?.parent;
      return parent && visIds.has(parent) ? parent : null;
    };
    const seen = new Set(allEdges.map((e) => e.kind + "|" + e.from + "|" + e.to));
    allEdges.forEach((e) => {
      if (e.kind === "contains") return;
      const a = mapEnd(e.from);
      const b = mapEnd(e.to);
      if (!a || !b || a === b) return;
      if (a === e.from && b === e.to) return;
      const key = e.kind + "|" + a + "|" + b;
      if (seen.has(key)) return;
      seen.add(key);
      extraEdges.push({ from: a, to: b, kind: e.kind });
    });
  } else {
    let candidates = allNodes.filter(
      (n) => n.type === "component" || n.type === "ref" || n.type === "rule" || n.level === "component",
    );
    if (focusContainer) {
      const directIds = new Set(
        candidates.filter((n) => n.parent === focusContainer).map((n) => n.id),
      );
      allEdges
        .filter((e) => e.kind === "uses" && directIds.has(e.from))
        .forEach((e) => directIds.add(e.to));
      candidates = allNodes.filter((n) => directIds.has(n.id));
    }
    visNodes = candidates;
  }

  if (tlVisibleSet !== null) {
    visNodes = visNodes.filter((n) => tlVisibleSet.has(n.id));
  }

  const visIds = new Set(visNodes.map((n) => n.id));
  const visEdges = allEdges.filter((e) => visIds.has(e.from) && visIds.has(e.to));
  return { nodes: visNodes, edges: visEdges.concat(extraEdges) };
}

export function layoutNodes(nodes: LNode[], edges: C3Edge[]): void {
  const byRing: Record<string, LNode[]> = {};
  nodes.forEach((n) => {
    const rk = n.ring || "infra";
    if (!byRing[rk]) byRing[rk] = [];
    byRing[rk].push(n);
  });

  const placed: Record<string, LNode> = {};

  function neighborAngle(n: LNode): number | null {
    let sx = 0,
      sy = 0,
      c = 0;
    edges.forEach((e) => {
      let other: LNode | undefined;
      if (e.from === n.id) other = placed[e.to];
      else if (e.to === n.id) other = placed[e.from];
      if (other && other._x !== undefined && other._z !== undefined) {
        const a = Math.atan2(other._z, other._x);
        sx += Math.cos(a);
        sy += Math.sin(a);
        c++;
      }
    });
    return c ? Math.atan2(sy, sx) : null;
  }

  function getParentAngle(n: LNode): number | null {
    if (!n.parent || !placed[n.parent]) return null;
    const p = placed[n.parent];
    if (p._x === undefined || p._z === undefined) return null;
    return Math.atan2(p._z, p._x);
  }

  RING_DEFS.forEach((ring) => {
    const arr = byRing[ring.key];
    if (!arr || !arr.length) return;

    if (ring.key === "core" && arr.length === 1) {
      arr[0]._x = 0;
      arr[0]._z = 0;
      arr[0]._y = 0;
      placed[arr[0].id] = arr[0];
      return;
    }

    arr.forEach((n) => {
      const pa = getParentAngle(n);
      const na = neighborAngle(n);
      n._want = pa !== null ? pa : na !== null ? na : ((hash(n.id) % 1000) / 1000) * Math.PI * 2;
    });
    arr.sort((a, b) => (a._want ?? 0) - (b._want ?? 0));

    const slots = arr.map((_, i) => (i / arr.length) * Math.PI * 2);
    let sx = 0,
      sy = 0;
    arr.forEach((n, i) => {
      const d = (n._want ?? 0) - slots[i];
      sx += Math.cos(d);
      sy += Math.sin(d);
    });
    const off = Math.atan2(sy, sx);

    arr.forEach((n, i) => {
      const h = hash(n.id);
      const angle = slots[i] + off + ((h % 100) / 100 - 0.5) * 0.18;
      const rad = ring.r + (((h >> 3) % 100) / 100 - 0.5) * 3.5;
      n._x = Math.cos(angle) * rad;
      n._z = Math.sin(angle) * rad;
      n._y = 0;
      placed[n.id] = n;
    });
  });

  nodes.forEach((n) => {
    if (n._x === undefined) {
      const h = hash(n.id);
      const angle = ((h % 1000) / 1000) * Math.PI * 2;
      n._x = Math.cos(angle) * 42;
      n._z = Math.sin(angle) * 42;
      n._y = 0;
    }
  });
}
