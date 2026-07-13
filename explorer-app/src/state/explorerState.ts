import { useSyncExternalStore } from "react";
import type { ExplorerScene, Snapshot } from "../scene/ExplorerScene";

const EMPTY: Snapshot = {
  ready: false,
  level: "container",
  focusContainer: null,
  query: "",
  dimmed: [],
  lifecycleCounts: {},
  selection: null,
  tooltip: null,
  timeline: { available: false, active: false, index: 0, playing: false, speed: 1, eventCount: 0 },
  lastUpdate: null,
};

const noopSubscribe = (): (() => void) => () => {};

export function useExplorerSnapshot(scene: ExplorerScene | null): Snapshot {
  return useSyncExternalStore(
    scene ? scene.subscribe : noopSubscribe,
    scene ? scene.getSnapshot : () => EMPTY,
  );
}
