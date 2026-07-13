import type { C3ArchitectureExplorerPayload } from "./types";

export type C3Payload = C3ArchitectureExplorerPayload;
export type C3Node = C3Payload["nodes"][number];
export type C3Edge = C3Payload["edges"][number];
export type C3Event = C3Payload["events"][number];

export type Level = "context" | "container" | "component";

export function lifecycleOf(n: { lifecycle?: string; staged?: boolean }): string {
  return n.lifecycle || (n.staged ? "staged" : "open");
}
