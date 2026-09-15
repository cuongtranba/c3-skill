import { useSyncExternalStore } from "react";
import type { CityScene, Snapshot } from "../scene/CityScene";

const EMPTY: Snapshot = {
  ready: false,
  level: "all",
  query: "",
  facets: {},
  motion: "active",
  bloom: true,
  skin: "industrial",
  reducedMotion: false,
  selection: null,
  tooltip: null,
  timeline: { available: false, active: false, index: 0, playing: false, speed: 1, eventCount: 0 },
  lastUpdate: null,
  visibleCount: 0,
  nodeCount: 0,
};

const noopSubscribe = (): (() => void) => () => {};

export function useExplorerSnapshot(scene: CityScene | null): Snapshot {
  return useSyncExternalStore(scene ? scene.subscribe : noopSubscribe, scene ? scene.getSnapshot : () => EMPTY);
}
