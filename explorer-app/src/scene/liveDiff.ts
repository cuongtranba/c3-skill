import type { C3Payload } from "../data";

export interface LiveDiff {
  added: string[];
  removed: string[];
  changed: string[];
}

const CHANGE_FIELDS = ["lifecycle", "staged", "title", "parent"] as const;

export function diffPayload(prev: C3Payload, next: C3Payload): LiveDiff {
  const prevById = new Map(prev.nodes.map((n) => [n.id, n]));
  const nextIds = new Set(next.nodes.map((n) => n.id));

  const added: string[] = [];
  const changed: string[] = [];
  next.nodes.forEach((n) => {
    const old = prevById.get(n.id);
    if (!old) {
      added.push(n.id);
      return;
    }
    if (CHANGE_FIELDS.some((f) => old[f] !== n[f])) changed.push(n.id);
  });

  const removed = prev.nodes.filter((n) => !nextIds.has(n.id)).map((n) => n.id);
  return { added, removed, changed };
}
