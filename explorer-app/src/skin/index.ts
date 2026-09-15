import type * as THREE from "three";
import type { Skin } from "./types";
import { industrial } from "./industrial";
import { blueprint } from "./blueprint";
import { keepMaterials } from "./kit";

export type { Skin, SkinContext, SkinTokens, MaterialSet, ArchetypeBuilder, Built, EnvironmentHooks, Lighting, RoadStyle, LabelStyle, Palette, KindStyle, StatusDef, StatusStyle, Motion, Spinner, Extent } from "./types";
export { kindStyle, statusOf, boundaryKindColor, lifecycleColor, districtTint } from "./resolve";

/** Registry, in menu order. Adding a skin is adding it here (see explorer-app/SKINS.md). */
export const SKINS: readonly Skin[] = [industrial, blueprint];

export const DEFAULT_SKIN: Skin = industrial;

const STORAGE_KEY = "c3v.skin";

export function resolveSkin(id: string | null | undefined): Skin {
  return SKINS.find((s) => s.id === id) ?? DEFAULT_SKIN;
}

export function hasSkin(id: string): boolean {
  return SKINS.some((s) => s.id === id);
}

/** Selection order: `window.C3_SKIN` → `?skin=` → `localStorage("c3v.skin")` → default. Unknown ids fall through. */
export function initialSkinId(): string {
  const candidates: (string | null | undefined)[] = [];
  if (typeof window !== "undefined") {
    candidates.push(window.C3_SKIN);
    try {
      candidates.push(new URLSearchParams(window.location.search).get("skin"));
    } catch {
      /* no location */
    }
    try {
      candidates.push(window.localStorage?.getItem(STORAGE_KEY));
    } catch {
      /* storage blocked */
    }
  }
  for (const c of candidates) if (c && hasSkin(c)) return c;
  return DEFAULT_SKIN.id;
}

export function persistSkinId(id: string): void {
  try {
    window.localStorage?.setItem(STORAGE_KEY, id);
  } catch {
    /* storage blocked */
  }
}

/** Writes the skin's CSS custom properties onto `:root` so the chrome follows the city. */
export function applyChrome(skin: Skin, root: HTMLElement | null = typeof document !== "undefined" ? document.documentElement : null): void {
  if (!root) return;
  for (const [k, v] of Object.entries(skin.chrome)) root.style.setProperty(k, v);
  root.dataset.skin = skin.id;
}

/* Skin-owned materials outlive any one world (see kit.ts); they are registered on first use
 * and released only when the scene switches to another skin. Re-selecting a skin later is
 * safe: three.js re-uploads a disposed material on its next draw. */
const registered = new WeakSet<Skin>();

export function skinMaterials(skin: Skin): Set<THREE.Material> {
  const out = new Set<THREE.Material>(Object.values(skin.materials));
  for (const m of [skin.roads.shoulder, skin.roads.trench, skin.roads.rails]) if (m) out.add(m);
  for (const m of skin.extraMaterials ?? []) out.add(m);
  if (!registered.has(skin)) {
    keepMaterials(out);
    registered.add(skin);
  }
  return out;
}

export function releaseSkinMaterials(skin: Skin): void {
  for (const m of skinMaterials(skin)) m.dispose();
}
