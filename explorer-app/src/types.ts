// Payload v2 as the renderer consumes it (docs/specs/2026-09-15-c3v-visualize-payload-v2.md).
// The shape is owned by the Go CLI: `c3x visualize --schema` → schema/explorer-payload.schema.json →
// `npm run generate-types` → types.generated.ts. The assertion at the bottom fails to compile when the
// generated (schema) type stops being assignable to these hand-written, deliberately more tolerant types.
import type { C3ArchitectureExplorerPayload } from "./types.generated";

export type Lifecycle = "frozen" | "staged" | "open" | "accepted" | "done" | "superseded";
export type StatusKey =
  | "stable" | "changing" | "drift" | "review" | "unchecked"
  | "open" | "accepted" | "done" | "superseded" | "governance";
export type Archetype =
  | "headquarters" | "gatehouse" | "comms" | "command" | "plant" | "bunker"
  | "transmit" | "outpost" | "record" | "zone" | "route";
export type Verdict = "holds" | "drift" | "needs-judgement" | "unchecked";
export type Waypoint = [number, number, number];

export interface Transition { from: string; to: string; by: string }
export interface CodeBinding { globs: string[]; files: number; loc: number }
export interface Layout { x: number; y: number; z: number; w: number; d: number }
export interface Dock { id: string; face: "n" | "s" | "e" | "w"; x: number; z: number; edgeId: string }

export interface C3Node {
  id: string;
  type: string;
  title: string;
  goal?: string;
  parent?: string;
  level: "context" | "container" | "component";
  lifecycle: Lifecycle;
  staged?: boolean;
  stagedBy?: string[];
  transition?: Transition | null;
  category?: string;
  tech?: string;
  boundaries?: string[];
  boundaryKind?: string;
  legacyBoundary?: string;
  code?: CodeBinding | null;
  eval?: { verdict: Verdict } | null;
  statusKey: StatusKey;
  archetype: Archetype;
  district: string;
  importance: number;
  layout: Layout;
  docks?: Dock[];
}

export interface C3Edge {
  id: string;
  from: string;
  to: string;
  kind: string;
  label?: string;
  flow?: string;
  seq?: number;
}

export interface C3Event {
  id: string;
  date: string;
  title: string;
  status: string;
  creates?: string[];
  modifies?: string[];
}

export interface C3Flow { id: string; title: string; steps: { seq: number; from: string; to: string; action?: string }[] }
export interface C3Boundary { id: string; kind: string; parent: string; members: string[] }
export interface District { id: string; title: string; kind: "sector" | "hq" | "governance"; x: number; z: number; w: number; d: number; y: number; members: string[] }
export interface Street { id: string; district: string; z: number; x0: number; x1: number; width: number; major: boolean }
export interface Avenue { id: string; district: string; x: number; z0: number; z1: number; width: number }
export interface Roads { streets: Street[]; avenues: Avenue[] }
export interface Route { id: string; edgeId: string; kind: string; active: boolean; segments: string[]; lane: number; sharedWith?: string; waypoints: Waypoint[] }

export interface C3Payload {
  schemaVersion: number;
  project: string;
  generatedAt: string;
  nodes: C3Node[];
  edges: C3Edge[];
  flows: C3Flow[];
  boundaries: C3Boundary[];
  districts: District[];
  roads: Roads;
  routes: Route[];
  events: C3Event[];
  agentRoutes?: Record<string, unknown>;
}

// Drift guard: whatever the Go schema emits must fit the renderer's model.
const _schemaFitsRenderer: C3Payload = {} as C3ArchitectureExplorerPayload;
void _schemaFitsRenderer;
