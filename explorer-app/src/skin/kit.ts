import * as THREE from "three";
import type { SkinContext } from "./types";

/* Geometry kit shared by every skin's builders and environment hooks. Nothing here has a
 * look of its own: primitives take a material, the composite parts (vents, antennas,
 * dishes, fans) take the build context and draw from its material slots. */

export type Mesh = THREE.Mesh<THREE.BufferGeometry, THREE.Material>;

/** Unlit colour for emitters (windows, lamps, strips). Per-instance so a node can dim on its own. */
export function emissive(hex: string, k = 1): THREE.MeshBasicMaterial {
  return new THREE.MeshBasicMaterial({ color: new THREE.Color(hex).multiplyScalar(k), toneMapped: false });
}

export function box(w: number, h: number, d: number, mat: THREE.Material, x = 0, y = 0, z = 0): Mesh {
  const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat);
  m.position.set(x, y + h / 2, z);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

export function cyl(r: number, h: number, mat: THREE.Material, x = 0, y = 0, z = 0, seg = 32, r2?: number): Mesh {
  const m = new THREE.Mesh(new THREE.CylinderGeometry(r, r2 ?? r, h, seg), mat);
  m.position.set(x, y + h / 2, z);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

/** A thin slab positioned by its centre (lights, rails, windows). */
export function strip(w: number, h: number, d: number, x: number, y: number, z: number, mat: THREE.Material): Mesh {
  const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat);
  m.position.set(x, y, z);
  return m;
}

export function vents(ctx: SkinContext, g: THREE.Group, x: number, y: number, z: number, w: number, n: number, axis: "x" | "z" = "x", mat: THREE.Material = ctx.m.darkMetal): void {
  for (let i = 0; i < n; i++) {
    g.add(axis === "x" ? strip(w, 0.12, 0.35, x, y + i * 0.45, z, mat) : strip(0.35, 0.12, w, x, y + i * 0.45, z, mat));
  }
}

export function antenna(ctx: SkinContext, g: THREE.Group, x: number, y: number, z: number, h: number, lampMat: THREE.Material | null): void {
  g.add(cyl(0.07, h, ctx.m.steel, x, y, z, 6));
  if (lampMat) {
    const l = new THREE.Mesh(new THREE.SphereGeometry(0.16, 8, 6), lampMat);
    l.position.set(x, y + h + 0.1, z);
    l.userData.blink = true;
    g.add(l);
  }
}

export function dish(ctx: SkinContext, g: THREE.Group, x: number, y: number, z: number, r: number, rotY = 0, spin = 0): Mesh {
  const d = new THREE.Mesh(new THREE.SphereGeometry(r, 24, 12, 0, Math.PI * 2, 0, Math.PI / 2.8), ctx.m.metal);
  d.position.set(x, y, z);
  d.rotation.set(Math.PI + 0.7, rotY, 0);
  d.castShadow = true;
  g.add(d);
  g.add(cyl(0.1, y, ctx.m.steel, x, 0, z, 8));
  if (spin) ctx.spinners.push({ obj: d, axis: "y", speed: spin });
  return d;
}

export function fan(ctx: SkinContext, g: THREE.Group, x: number, y: number, z: number, r: number): THREE.Group {
  const grp = new THREE.Group();
  grp.position.set(x, y, z);
  const ring = new THREE.Mesh(new THREE.TorusGeometry(r, 0.12, 8, 24), ctx.m.steel);
  ring.rotation.x = Math.PI / 2;
  grp.add(ring);
  const blades = new THREE.Group();
  for (let i = 0; i < 4; i++) {
    const b = new THREE.Mesh(new THREE.BoxGeometry(r * 1.7, 0.05, 0.28), ctx.m.metal);
    b.rotation.y = (i * Math.PI) / 4;
    blades.add(b);
  }
  grp.add(blades);
  g.add(grp);
  ctx.spinners.push({ obj: blades, axis: "y", speed: 3.2 });
  return grp;
}

/** Ink outline around a mesh: EdgesGeometry lines that follow the parent mesh's transform. */
export function inkEdges(mesh: THREE.Mesh, mat: THREE.LineBasicMaterial, threshold = 25): THREE.LineSegments {
  const edges = new THREE.LineSegments(new THREE.EdgesGeometry(mesh.geometry, threshold), mat);
  edges.userData.noExport = true;
  edges.raycast = () => {};
  return edges;
}

/* Materials a skin owns outlive any one world: a rebuild (live frame, skin switch) disposes
 * the per-instance emitters and geometries under the world but leaves these alone. The
 * skin registry releases them only when the skin is switched away. */
const PERSISTENT = new WeakSet<THREE.Material>();

export function keepMaterials(mats: Iterable<THREE.Material>): void {
  for (const m of mats) PERSISTENT.add(m);
}

export function isPersistentMaterial(m: THREE.Material): boolean {
  return PERSISTENT.has(m);
}

/** Disposes geometries and per-instance materials under `root`; skin-owned materials survive. */
export function disposeTree(root: THREE.Object3D): void {
  root.traverse((o) => {
    const m = o as THREE.Mesh;
    if (!(m as THREE.Mesh).isMesh && !(o as THREE.Sprite).isSprite && !(o as THREE.Line).isLine) return;
    m.geometry?.dispose();
    const mats = Array.isArray(m.material) ? m.material : [m.material];
    for (const mat of mats) {
      if (!mat || PERSISTENT.has(mat)) continue;
      const tex = (mat as THREE.Material & { map?: THREE.Texture | null }).map;
      tex?.dispose();
      mat.dispose();
    }
  });
}
