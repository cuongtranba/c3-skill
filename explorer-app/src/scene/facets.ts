import type { C3Payload, C3Node } from "../types";

/* Facets are a pure conjunctive filter over payload nodes. They decide what is hidden;
 * they never move anything. Every field is optional and an empty list means "no constraint". */
export interface Facets {
  type?: string[];
  district?: string[];
  boundary?: string[];
  boundaryKind?: string[];
  statusKey?: string[];
  lifecycle?: string[];
  archetype?: string[];
  /** Flow id: keeps the nodes its steps touch and highlights its routes. */
  flow?: string;
  text?: string;
  codeGlob?: string;
}

export const FACET_KEYS: (keyof Facets)[] = ["type", "district", "boundary", "boundaryKind", "statusKey", "lifecycle", "archetype", "flow", "text", "codeGlob"];

export function isEmptyFacets(f: Facets): boolean {
  return FACET_KEYS.every((k) => {
    const v = f[k];
    return v === undefined || v === "" || (Array.isArray(v) && v.length === 0);
  });
}

/** Minimal glob: `**` spans directories, `*` stays inside one segment, `?` is one char. */
export function globToRegExp(glob: string): RegExp {
  let re = "^";
  for (let i = 0; i < glob.length; i++) {
    const c = glob[i];
    if (c === "*") {
      if (glob[i + 1] === "*") {
        i++;
        if (glob[i + 1] === "/") {
          i++;
          re += "(?:.*/)?";
        } else re += ".*";
      } else re += "[^/]*";
    } else if (c === "?") re += "[^/]";
    else if (/[.+^${}()|[\]\\]/.test(c)) re += "\\" + c;
    else re += c;
  }
  return new RegExp(re + "$");
}

export function globMatch(glob: string, path: string): boolean {
  return globToRegExp(glob).test(path);
}

/** A node's binding matches when one of its globs covers the query path, the query glob covers one of its globs, or they share a literal prefix. */
export function codeGlobMatches(node: C3Node, query: string): boolean {
  const globs = node.code?.globs ?? [];
  if (!globs.length) return false;
  const q = query.trim();
  if (!q) return true;
  const literal = q.replace(/[*?].*$/, "");
  return globs.some((g) => globMatch(g, q) || globMatch(q, g) || (literal.length > 0 && (g.startsWith(literal) || literal.startsWith(g.replace(/[*?].*$/, "")))));
}

interface Index {
  kindOf: Map<string, string>;
  flowMembers: Map<string, Set<string>>;
}

function index(p: C3Payload): Index {
  const kindOf = new Map<string, string>();
  for (const b of p.boundaries) kindOf.set(b.id, b.kind);
  for (const n of p.nodes) if (n.archetype === "zone" && n.boundaryKind) kindOf.set(n.id, n.boundaryKind);
  const flowMembers = new Map<string, Set<string>>();
  for (const f of p.flows) {
    const s = new Set<string>([f.id]);
    for (const st of f.steps) {
      s.add(st.from);
      s.add(st.to);
    }
    flowMembers.set(f.id, s);
  }
  return { kindOf, flowMembers };
}

function textOf(n: C3Node): string {
  return [n.title, n.id, n.type, n.goal ?? "", n.tech ?? "", n.category ?? "", n.archetype, n.statusKey].join(" ").toLowerCase();
}

function has(list: string[] | undefined, v: string): boolean {
  return !list || list.length === 0 || list.includes(v);
}

export function nodeMatches(n: C3Node, f: Facets, ix: Index): boolean {
  if (!has(f.type, n.type)) return false;
  if (!has(f.district, n.district)) return false;
  if (!has(f.statusKey, n.statusKey)) return false;
  if (!has(f.lifecycle, n.lifecycle)) return false;
  if (!has(f.archetype, n.archetype)) return false;
  if (f.boundary && f.boundary.length) {
    const mine = new Set(n.boundaries ?? []);
    if (n.archetype === "zone") mine.add(n.id);
    if (!f.boundary.some((b) => mine.has(b))) return false;
  }
  if (f.boundaryKind && f.boundaryKind.length) {
    const kinds = new Set((n.boundaries ?? []).map((b) => ix.kindOf.get(b) ?? ""));
    if (n.archetype === "zone") kinds.add(n.boundaryKind ?? "");
    if (!f.boundaryKind.some((k) => kinds.has(k))) return false;
  }
  if (f.flow) {
    const members = ix.flowMembers.get(f.flow);
    if (!members || !members.has(n.id)) return false;
  }
  if (f.text && f.text.trim()) {
    if (!textOf(n).includes(f.text.trim().toLowerCase())) return false;
  }
  if (f.codeGlob && f.codeGlob.trim()) {
    if (!codeGlobMatches(n, f.codeGlob)) return false;
  }
  return true;
}

/** Ids of the nodes that survive every facet. */
export function applyFacets(p: C3Payload, f: Facets): Set<string> {
  const ix = index(p);
  const out = new Set<string>();
  if (isEmptyFacets(f)) {
    for (const n of p.nodes) out.add(n.id);
    return out;
  }
  for (const n of p.nodes) if (nodeMatches(n, f, ix)) out.add(n.id);
  return out;
}

/** Route ids to light up for a flow facet: its flow_step routes plus the depends_on routes they share. */
export function flowRouteIds(p: C3Payload, flowId: string): Set<string> {
  const out = new Set<string>();
  const stepEdges = new Set(p.edges.filter((e) => e.kind === "flow_step" && e.flow === flowId).map((e) => e.id));
  for (const r of p.routes) {
    if (stepEdges.has(r.edgeId)) {
      out.add(r.id);
      if (r.sharedWith) out.add(r.sharedWith);
    }
  }
  return out;
}

/** Distinct values per facet dimension, for the filter panel. */
export function facetOptions(p: C3Payload): Record<"type" | "district" | "boundary" | "boundaryKind" | "statusKey" | "lifecycle" | "archetype" | "flow", { value: string; label: string; count: number }[]> {
  const count = (pick: (n: C3Node) => string[]): Map<string, number> => {
    const m = new Map<string, number>();
    for (const n of p.nodes) for (const v of pick(n)) if (v) m.set(v, (m.get(v) ?? 0) + 1);
    return m;
  };
  const rows = (m: Map<string, number>, label: (v: string) => string = (v) => v): { value: string; label: string; count: number }[] =>
    [...m.entries()].sort((a, b) => a[0].localeCompare(b[0])).map(([value, c]) => ({ value, label: label(value), count: c }));
  const districtTitle = new Map(p.districts.map((d) => [d.id, d.title]));
  const boundaryTitle = new Map(p.nodes.filter((n) => n.archetype === "zone").map((n) => [n.id, n.title]));
  const ix = index(p);
  return {
    type: rows(count((n) => [n.type])),
    district: rows(count((n) => [n.district]), (v) => districtTitle.get(v) ?? v),
    boundary: rows(count((n) => n.boundaries ?? []), (v) => boundaryTitle.get(v) ?? v),
    boundaryKind: rows(count((n) => (n.boundaries ?? []).map((b) => ix.kindOf.get(b) ?? ""))),
    statusKey: rows(count((n) => [n.statusKey])),
    lifecycle: rows(count((n) => [n.lifecycle])),
    archetype: rows(count((n) => [n.archetype])),
    flow: p.flows.map((f) => ({ value: f.id, label: f.title, count: f.steps.length })),
  };
}
