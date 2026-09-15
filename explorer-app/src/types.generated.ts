/* Generated from schema/explorer-payload.schema.json — do not edit. Resync: c3x visualize --schema > schema/explorer-payload.schema.json && npm run generate-types */

/**
 * The window.C3_DATA contract (payload v2: nodes, edges, flows, boundaries and the derived city layout) validated before the three.js HTML is generated.
 */
export interface C3ArchitectureExplorerPayload {
  boundaries: {
    id: string;
    /**
     * the Perimeter Kind enum value; empty for N.A
     */
    kind: string;
    members: string[];
    parent: string;
    [k: string]: unknown;
  }[];
  districts: {
    d: number;
    id: string;
    kind: "governance" | "hq" | "sector";
    members: string[];
    title: string;
    w: number;
    x: number;
    y: number;
    z: number;
    [k: string]: unknown;
  }[];
  edges: {
    /**
     * flow_step edges only
     */
    flow?: string;
    from: string;
    id: string;
    /**
     * canvas-driven
     */
    kind: string;
    label?: string;
    /**
     * flow_step edges only
     */
    seq?: number;
    to: string;
    [k: string]: unknown;
  }[];
  /**
   * The timeline: one event per change-unit (ADR), date-ordered; every fact is created by exactly one event so replaying all events reproduces the live graph.
   *
   * @minItems 1
   */
  events: [
    {
      creates?: string[];
      date: string;
      id: string;
      modifies?: string[];
      status: "accepted" | "done" | "open" | "superseded";
      title: string;
      [k: string]: unknown;
    },
    ...{
      creates?: string[];
      date: string;
      id: string;
      modifies?: string[];
      status: "accepted" | "done" | "open" | "superseded";
      title: string;
      [k: string]: unknown;
    }[]
  ];
  flows: {
    id: string;
    steps: {
      action: string;
      from: string;
      seq: number;
      to: string;
      [k: string]: unknown;
    }[];
    title: string;
    [k: string]: unknown;
  }[];
  generatedAt: string;
  /**
   * @minItems 1
   */
  nodes: [
    {
      archetype:
        | "bunker"
        | "command"
        | "comms"
        | "gatehouse"
        | "headquarters"
        | "outpost"
        | "plant"
        | "record"
        | "route"
        | "transmit"
        | "zone";
      /**
       * enclosing boundary ids, outermost first
       */
      boundaries: string[];
      /**
       * boundary nodes only: the Perimeter Kind
       */
      boundaryKind: string;
      /**
       * component category; may be empty
       */
      category: string;
      code: {
        files: number;
        globs: string[];
        loc: number;
        [k: string]: unknown;
      } | null;
      /**
       * district id; empty for zone/route archetypes
       */
      district: string;
      docks: {
        edgeId: string;
        face: "n" | "s";
        id: string;
        x: number;
        z: number;
        [k: string]: unknown;
      }[];
      eval: {
        verdict: "drift" | "holds" | "needs-judgement" | "unchecked";
        [k: string]: unknown;
      } | null;
      goal?: string;
      id: string;
      importance: number;
      layout: {
        d: number;
        w: number;
        x: number;
        y: number;
        z: number;
        [k: string]: unknown;
      };
      /**
       * container free-text boundary: field
       */
      legacyBoundary: string;
      level: "component" | "container" | "context";
      lifecycle: "accepted" | "done" | "frozen" | "open" | "staged" | "superseded";
      parent?: string;
      staged: boolean;
      stagedBy?: string[];
      statusKey:
        | "accepted"
        | "changing"
        | "done"
        | "drift"
        | "governance"
        | "open"
        | "review"
        | "stable"
        | "superseded"
        | "unchecked";
      /**
       * top two technologies by matched file count, joined with ' · '; empty when unknown
       */
      tech: string;
      title: string;
      transition?: {
        by: string;
        from: string;
        to: string;
        [k: string]: unknown;
      } | null;
      /**
       * canvas-driven
       */
      type: string;
      [k: string]: unknown;
    },
    ...{
      archetype:
        | "bunker"
        | "command"
        | "comms"
        | "gatehouse"
        | "headquarters"
        | "outpost"
        | "plant"
        | "record"
        | "route"
        | "transmit"
        | "zone";
      /**
       * enclosing boundary ids, outermost first
       */
      boundaries: string[];
      /**
       * boundary nodes only: the Perimeter Kind
       */
      boundaryKind: string;
      /**
       * component category; may be empty
       */
      category: string;
      code: {
        files: number;
        globs: string[];
        loc: number;
        [k: string]: unknown;
      } | null;
      /**
       * district id; empty for zone/route archetypes
       */
      district: string;
      docks: {
        edgeId: string;
        face: "n" | "s";
        id: string;
        x: number;
        z: number;
        [k: string]: unknown;
      }[];
      eval: {
        verdict: "drift" | "holds" | "needs-judgement" | "unchecked";
        [k: string]: unknown;
      } | null;
      goal?: string;
      id: string;
      importance: number;
      layout: {
        d: number;
        w: number;
        x: number;
        y: number;
        z: number;
        [k: string]: unknown;
      };
      /**
       * container free-text boundary: field
       */
      legacyBoundary: string;
      level: "component" | "container" | "context";
      lifecycle: "accepted" | "done" | "frozen" | "open" | "staged" | "superseded";
      parent?: string;
      staged: boolean;
      stagedBy?: string[];
      statusKey:
        | "accepted"
        | "changing"
        | "done"
        | "drift"
        | "governance"
        | "open"
        | "review"
        | "stable"
        | "superseded"
        | "unchecked";
      /**
       * top two technologies by matched file count, joined with ' · '; empty when unknown
       */
      tech: string;
      title: string;
      transition?: {
        by: string;
        from: string;
        to: string;
        [k: string]: unknown;
      } | null;
      /**
       * canvas-driven
       */
      type: string;
      [k: string]: unknown;
    }[]
  ];
  project: string;
  roads: {
    avenues: {
      district: string;
      id: string;
      width: number;
      x: number;
      z0: number;
      z1: number;
      [k: string]: unknown;
    }[];
    streets: {
      district: string;
      id: string;
      major: boolean;
      width: number;
      x0: number;
      x1: number;
      z: number;
      [k: string]: unknown;
    }[];
    [k: string]: unknown;
  };
  /**
   * One per non-contains edge: dock to dock along streets and avenues.
   */
  routes: {
    active: boolean;
    edgeId: string;
    id: string;
    /**
     * canvas-driven
     */
    kind: string;
    lane: number;
    segments: string[];
    sharedWith: string;
    /**
     * @minItems 2
     */
    waypoints: [[number, number, number], [number, number, number], ...[number, number, number][]];
    [k: string]: unknown;
  }[];
  schemaVersion: 2;
}
