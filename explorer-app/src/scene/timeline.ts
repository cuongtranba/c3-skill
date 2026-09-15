import type { C3Event, C3Node } from "../types";

/* Timeline replay over `events`: a node exists at index `idx` once some event at or before
 * idx created it. ADR nodes are their own event, so they appear when their event does;
 * an ADR with no event (or created by an earlier one) is treated as always present. */
export function timelineVisibleSet(events: C3Event[], nodes: C3Node[], idx: number): Set<string> {
  const adrEventIndex = new Map<string, number>();
  events.forEach((ev, i) => adrEventIndex.set(ev.id, i));

  const visible = new Set<string>();
  for (let i = 0; i <= idx && i < events.length; i++) {
    for (const id of events[i].creates ?? []) visible.add(id);
  }
  for (const n of nodes) {
    if (n.type !== "adr" || visible.has(n.id)) continue;
    const evIdx = adrEventIndex.get(n.id);
    if (evIdx === undefined || evIdx <= idx) visible.add(n.id);
  }
  return visible;
}

/** Everything the payload knows about that no event ever creates — always shown, so nothing is lost to replay. */
export function neverCreated(events: C3Event[], nodes: C3Node[]): Set<string> {
  const created = new Set<string>();
  for (const ev of events) for (const id of ev.creates ?? []) created.add(id);
  const out = new Set<string>();
  for (const n of nodes) if (!created.has(n.id) && n.type !== "adr") out.add(n.id);
  return out;
}
