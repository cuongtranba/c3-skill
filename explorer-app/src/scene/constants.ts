export interface RingDef {
  key: string;
  label: string;
  sub: string;
  desc: string;
  tint: string;
  r: number;
}

// Ring keys are pinned by the Go payload schema (ringForType in explore.go);
// labels name the entity type each ring actually holds, subs map the C4 zoom
// metaphor (country → city → neighborhood, center-out).
export const RING_DEFS: RingDef[] = [
  {
    key: "governance",
    label: "ADRs",
    sub: "Decisions",
    desc: "Architecture decision records — the change-units that stage work and freeze it into the graph.",
    tint: "#5b4a8a",
    r: 58,
  },
  {
    key: "security",
    label: "Rules",
    sub: "Constraints",
    desc: "Coding standards and constraints that govern how components are built.",
    tint: "#2f7a6f",
    r: 50,
  },
  {
    key: "infra",
    label: "Refs",
    sub: "Standards",
    desc: "Shared reference docs and standards that components depend on.",
    tint: "#5b6a72",
    r: 42,
  },
  {
    key: "service",
    label: "Components",
    sub: "L3 · Neighborhood",
    desc: "Component view (C4 level 3): a deeper zoom into one container — its internal logical components and how they interact.",
    tint: "#3f7fc4",
    r: 32,
  },
  {
    key: "platform",
    label: "Containers",
    sub: "L2 · City",
    desc: "Container view (C4 level 2): the major deployable or executable units — apps, services, databases — and how they communicate.",
    tint: "#2a9184",
    r: 19,
  },
  {
    key: "core",
    label: "System",
    sub: "L1 · Country",
    desc: "System context (C4 level 1): where the system sits in the wider ecosystem — its users and the external systems it talks to.",
    tint: "#2fa89a",
    r: 9,
  },
];

export const LIFECYCLE_COLORS: Record<string, string> = {
  frozen: "#2fa89a",
  staged: "#d98a2b",
  open: "#3f7fc4",
  accepted: "#2f7a6f",
  done: "#6b7280",
  superseded: "#9a3a31",
};

export const EDGE_COLORS: Record<string, string> = {
  contains: "#7c8794",
  uses: "#2fa89a",
  affects: "#d98a2b",
};

export function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h;
}

export function easeInOutQuad(t: number): number {
  return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
}

export function ringByKey(k: string): RingDef {
  return RING_DEFS.find((r) => r.key === k) || RING_DEFS[2];
}
