import * as THREE from "three";
import { mergeGeometries } from "three/addons/utils/BufferGeometryUtils.js";
import type { Spinner } from "../skin/types";

/**
 * A building is authored as dozens of small boxes and cylinders — readable to
 * write, expensive to draw: a 150-building base is ~7 000 draw calls a frame.
 * Static parts that share a material instance are baked into a single mesh per
 * material. Everything driven per frame keeps its own object: spinner pivots
 * (fans, radars — their parts are baked relative to the pivot), blinking lamps,
 * the status lamp, and single-mesh emitters such as dock slits.
 *
 * Returns the number of meshes removed, for tests and diagnostics.
 */
export function consolidateStatic(root: THREE.Object3D, spinners: Spinner[]): number {
  // A spinner's parts are static relative to the spinner: bake each one first,
  // then exclude the whole subtree from the root pass so it keeps its pivot.
  let removed = 0;
  const excluded = new Set<THREE.Object3D>();
  for (const s of spinners) {
    if (s.obj !== root && isUnder(s.obj, root)) removed += mergeStatic(s.obj, new Set());
    s.obj.traverse((o) => excluded.add(o));
  }
  return removed + mergeStatic(root, excluded);
}

function isUnder(o: THREE.Object3D, root: THREE.Object3D): boolean {
  for (let p: THREE.Object3D | null = o.parent; p; p = p.parent) if (p === root) return true;
  return false;
}

// Merges every group of ≥ 2 static meshes that share one material instance —
// the skin's persistent materials and a node's own accent material alike; a
// merged mesh keeps the same material reference, so per-node dimming still works.
function mergeStatic(root: THREE.Object3D, excluded: Set<THREE.Object3D>): number {
  root.updateMatrixWorld(true);
  const toRoot = root.matrixWorld.clone().invert();

  const byMaterial = new Map<THREE.Material, THREE.Mesh[]>();
  root.traverse((o) => {
    const m = o as THREE.Mesh;
    if (!m.isMesh || (m as THREE.InstancedMesh).isInstancedMesh) return;
    if (excluded.has(m) || m.userData.blink || m.userData.statusLamp || m.userData.noExport) return;
    if (Array.isArray(m.material)) return;
    const g = m.geometry;
    if (!g.getAttribute("position") || !g.getAttribute("normal") || !g.index) return;
    const list = byMaterial.get(m.material) ?? [];
    list.push(m);
    byMaterial.set(m.material, list);
  });

  let removed = 0;
  for (const [material, meshes] of byMaterial) {
    if (meshes.length < 2) continue;
    const parts: THREE.BufferGeometry[] = [];
    for (const m of meshes) {
      const g = m.geometry.clone();
      // Merged geometry only needs what the material samples; dropping stray
      // attributes keeps every part layout-compatible.
      for (const name of Object.keys(g.attributes)) {
        if (name !== "position" && name !== "normal" && name !== "uv") g.deleteAttribute(name);
      }
      g.applyMatrix4(new THREE.Matrix4().multiplyMatrices(toRoot, m.matrixWorld));
      parts.push(g);
    }
    const merged = mergeGeometries(parts, false);
    for (const p of parts) p.dispose();
    if (!merged) continue;
    const mesh = new THREE.Mesh(merged, material);
    mesh.castShadow = true;
    mesh.receiveShadow = true;
    mesh.userData.merged = meshes.length; // stays an anonymous part in the scene tree
    root.add(mesh);
    for (const m of meshes) {
      m.removeFromParent();
      m.geometry.dispose();
      removed++;
    }
  }
  return removed;
}
