import * as THREE from "three";

/* Exports the world group as THREE ObjectLoader JSON. Anything marked `userData.noExport`
 * (labels, selection FX, traffic, lights, helpers) is stripped so the file carries the
 * built environment only: districts, buildings, roads and route channels. */

function cloneExportable(src: THREE.Object3D): THREE.Object3D | null {
  if (src.userData.noExport) return null;
  if ((src as THREE.Light).isLight) return null;
  const copy = src.clone(false);
  copy.userData = { ...src.userData };
  delete copy.userData.noExport;
  delete copy.userData.baseScale;
  for (const child of src.children) {
    const c = cloneExportable(child);
    if (c) copy.add(c);
  }
  return copy;
}

export function sceneToObject(world: THREE.Object3D): THREE.Object3D {
  const root = cloneExportable(world) ?? new THREE.Group();
  root.updateMatrixWorld(true);
  return root;
}

export interface SceneJSON {
  metadata: { version: number; type: string; generator: string };
  object: Record<string, unknown>;
  [k: string]: unknown;
}

export function sceneToJSON(world: THREE.Object3D): SceneJSON {
  const json = sceneToObject(world).toJSON() as unknown as SceneJSON;
  json.metadata = { ...json.metadata, generator: "c3v visualize (renderer)" };
  return json;
}

/** Ids recorded as `userData.c3.id` under a loaded tree — the round-trip check compares them with the payload. */
export function collectC3Ids(root: THREE.Object3D, type?: string): Set<string> {
  const out = new Set<string>();
  root.traverse((o) => {
    const c3 = o.userData?.c3 as { id?: string; type?: string } | undefined;
    if (c3?.id && (!type || c3.type === type)) out.add(c3.id);
  });
  return out;
}
