import type { ExplorerScene } from "../scene/ExplorerScene";
import type { C3Payload } from "../data";

export interface ActionEvent {
  ts: string;
  cmd: string;
  args?: string[];
  mutating: boolean;
  success: boolean;
  error?: string;
}

export interface LiveCallbacks {
  onAction(e: ActionEvent): void;
  onInvalid(issues: string[]): void;
  onStatus(connected: boolean): void;
}

const ID_RE = /\b(c3-\d+|adr-[\w-]+|ref-[\w-]+|rule-[\w-]+)\b/g;

export function extractEntityIds(args: string[] | undefined): string[] {
  if (!args) return [];
  const ids = new Set<string>();
  args.forEach((a) => {
    for (const m of a.matchAll(ID_RE)) ids.add(m[1]);
  });
  return Array.from(ids);
}

export function startLiveClient(scene: ExplorerScene, cb: LiveCallbacks): () => void {
  const es = new EventSource("/events");

  es.addEventListener("payload", (e) => {
    cb.onInvalid([]);
    const next = JSON.parse((e as MessageEvent).data) as C3Payload;
    // Keep the global in step with the scene so external checks against
    // window.C3_DATA always see the live state.
    window.C3_DATA = next;
    scene.applyLiveData(next);
  });

  es.addEventListener("action", (e) => {
    const action = JSON.parse((e as MessageEvent).data) as ActionEvent;
    cb.onAction(action);
    const ids = extractEntityIds(action.args);
    if (ids.length) scene.pulseNodes(ids, action.mutating ? "amber" : "mint");
  });

  es.addEventListener("invalid", (e) => {
    const body = JSON.parse((e as MessageEvent).data) as { issues?: string[] };
    cb.onInvalid(body.issues || ["payload failed validation"]);
  });

  es.onopen = () => cb.onStatus(true);
  es.onerror = () => cb.onStatus(false);

  return () => es.close();
}
