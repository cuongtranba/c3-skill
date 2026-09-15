import type { C3Node, C3Payload } from "../types";

export interface LiveDiff {
  added: string[];
  removed: string[];
  changed: string[];
}

const SCALAR_FIELDS = ["lifecycle", "staged", "title", "parent", "statusKey", "archetype", "district", "tech", "boundaryKind"] as const;

function evalVerdict(n: C3Node): string {
  return n.eval?.verdict ?? "";
}

function layoutKey(n: C3Node): string {
  const l = n.layout;
  return l ? `${l.x}|${l.y}|${l.z}|${l.w}|${l.d}` : "";
}

function stagedBy(n: C3Node): string {
  return (n.stagedBy ?? []).join(",");
}

function boundaries(n: C3Node): string {
  return (n.boundaries ?? []).join(",");
}

/** Which nodes a live payload frame adds, removes or changes in a way the city shows. */
export function diffPayload(prev: C3Payload, next: C3Payload): LiveDiff {
  const prevById = new Map(prev.nodes.map((n) => [n.id, n]));
  const nextIds = new Set(next.nodes.map((n) => n.id));

  const added: string[] = [];
  const changed: string[] = [];
  for (const n of next.nodes) {
    const old = prevById.get(n.id);
    if (!old) {
      added.push(n.id);
      continue;
    }
    const scalar = SCALAR_FIELDS.some((f) => (old[f] ?? "") !== (n[f] ?? ""));
    if (scalar || evalVerdict(old) !== evalVerdict(n) || layoutKey(old) !== layoutKey(n) || stagedBy(old) !== stagedBy(n) || boundaries(old) !== boundaries(n)) {
      changed.push(n.id);
    }
  }

  const removed = prev.nodes.filter((n) => !nextIds.has(n.id)).map((n) => n.id);
  return { added, removed, changed };
}
