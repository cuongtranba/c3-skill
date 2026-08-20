/* Generated from schema/explorer-payload.schema.json — do not edit. Resync: c3x explore --schema > schema/explorer-payload.schema.json && npm run generate-types */

/**
 * The window.C3_DATA contract validated before the three.js HTML is generated.
 */
export interface C3ArchitectureExplorerPayload {
  edges: {
    from: string;
    kind: "affects" | "contains" | "uses";
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
  generatedAt: string;
  /**
   * @minItems 1
   */
  nodes: [
    {
      goal?: string;
      id: string;
      level: "component" | "container" | "context";
      lifecycle: "accepted" | "done" | "frozen" | "open" | "staged" | "superseded";
      parent?: string;
      ring: "core" | "governance" | "infra" | "platform" | "security" | "service";
      staged?: boolean;
      stagedBy?: string[];
      title: string;
      transition?: {
        by: string;
        from: string;
        to: string;
        [k: string]: unknown;
      } | null;
      type: "adr" | "component" | "container" | "pm-requirement" | "ref" | "rule" | "system" | "user-story";
      [k: string]: unknown;
    },
    ...{
      goal?: string;
      id: string;
      level: "component" | "container" | "context";
      lifecycle: "accepted" | "done" | "frozen" | "open" | "staged" | "superseded";
      parent?: string;
      ring: "core" | "governance" | "infra" | "platform" | "security" | "service";
      staged?: boolean;
      stagedBy?: string[];
      title: string;
      transition?: {
        by: string;
        from: string;
        to: string;
        [k: string]: unknown;
      } | null;
      type: "adr" | "component" | "container" | "pm-requirement" | "ref" | "rule" | "system" | "user-story";
      [k: string]: unknown;
    }[]
  ];
  project: string;
}
