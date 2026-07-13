export interface RingDef {
  key: string;
  label: string;
  tint: string;
  r: number;
}

export const RING_DEFS: RingDef[] = [
  { key: "governance", label: "Governance", tint: "#5b4a8a", r: 58 },
  { key: "security", label: "Security", tint: "#2f7a6f", r: 50 },
  { key: "infra", label: "Infra", tint: "#5b6a72", r: 42 },
  { key: "service", label: "Service", tint: "#3f7fc4", r: 32 },
  { key: "platform", label: "Platform", tint: "#2a9184", r: 19 },
  { key: "core", label: "Core", tint: "#2fa89a", r: 9 },
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
