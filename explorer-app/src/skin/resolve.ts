import type { C3Node } from "../types";
import type { KindStyle, Skin, StatusStyle } from "./types";

/* Token lookups with the fallbacks the payload's open vocabularies need. Every consumer
 * (scene, chrome, tests) goes through these, never through a skin's tables directly. */

/** Edge kinds are open strings; an unknown kind takes the neutral governance look and its own name. */
export function kindStyle(skin: Skin, kind: string): KindStyle {
  return skin.tokens.kinds[kind] ?? { color: skin.tokens.palette.subtle, label: kind.replace(/_/g, " ") };
}

/** Status lamp semantics come from the payload's statusKey; the renderer never re-derives it. */
export function statusOf(skin: Skin, n: Pick<C3Node, "statusKey">): StatusStyle {
  const s = skin.tokens.status[n.statusKey] ?? skin.tokens.status.unchecked;
  return { key: n.statusKey, ...s };
}

export function boundaryKindColor(skin: Skin, kind: string): string {
  return skin.tokens.boundaryKinds[kind] ?? skin.tokens.boundaryFallback;
}

export function lifecycleColor(skin: Skin, lifecycle: string): string {
  return skin.tokens.lifecycle[lifecycle] ?? skin.tokens.lifecycleFallback;
}

export function districtTint(skin: Skin, kind: string): string {
  return skin.tokens.districtTints[kind] ?? skin.tokens.districtTints.sector ?? skin.tokens.palette.line;
}
