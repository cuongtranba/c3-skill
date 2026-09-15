import type * as THREE from "three";

/* A pure walk over the world group into a compact tree the inspector can render.
 * Objects marked noExport are skipped; runs of anonymous meshes collapse into a count
 * so a building shows as one row with "23 parts", not 23 rows. */
export interface SceneTreeNode {
  name: string;
  type: string;
  c3?: { id: string; type: string; archetype?: string; kind?: string; district?: string };
  visible: boolean;
  parts: number;
  children: SceneTreeNode[];
}

function isPart(o: THREE.Object3D): boolean {
  return !o.userData.c3 && !o.name && o.children.length === 0;
}

export function buildSceneTree(root: THREE.Object3D): SceneTreeNode {
  const walk = (o: THREE.Object3D): SceneTreeNode => {
    const node: SceneTreeNode = {
      name: o.name || (o.userData.c3?.id as string) || o.type,
      type: o.type,
      visible: o.visible,
      parts: 0,
      children: [],
    };
    if (o.userData.c3) node.c3 = o.userData.c3 as SceneTreeNode["c3"];
    for (const child of o.children) {
      if (child.userData.noExport) continue;
      if ((child as THREE.Light).isLight) continue;
      if (isPart(child)) {
        node.parts++;
        continue;
      }
      const sub = walk(child);
      // Anonymous groups with only parts fold into the parent count.
      if (!sub.c3 && !child.name && sub.children.length === 0) {
        node.parts += sub.parts;
        continue;
      }
      node.children.push(sub);
    }
    return node;
  };
  return walk(root);
}

export function countTree(t: SceneTreeNode): { nodes: number; parts: number } {
  let nodes = 1;
  let parts = t.parts;
  for (const c of t.children) {
    const r = countTree(c);
    nodes += r.nodes;
    parts += r.parts;
  }
  return { nodes, parts };
}
