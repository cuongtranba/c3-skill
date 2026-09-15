import type { C3Payload } from "./types";

export type { C3Payload, C3Node, C3Edge, C3Event, C3Flow, C3Boundary, District, Route, Street, Avenue, Dock } from "./types";
export type { StatusStyle } from "./skin/types";
export { statusOf } from "./skin/resolve";

/** Kept for the legacy setLevel surface; the city never re-lays out, levels only hide. */
export type Level = "context" | "container" | "component" | "all";

export function lifecycleOf(n: { lifecycle?: string; staged?: boolean }): string {
  return n.lifecycle || (n.staged ? "staged" : "open");
}

/** Fills the collections a partial or older payload may omit so every consumer can index safely. */
export function normalizePayload(raw: C3Payload): C3Payload {
  return {
    ...raw,
    nodes: raw.nodes ?? [],
    edges: raw.edges ?? [],
    flows: raw.flows ?? [],
    boundaries: raw.boundaries ?? [],
    districts: raw.districts ?? [],
    roads: { streets: raw.roads?.streets ?? [], avenues: raw.roads?.avenues ?? [] },
    routes: raw.routes ?? [],
    events: raw.events ?? [],
  };
}
