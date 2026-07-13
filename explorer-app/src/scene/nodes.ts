import * as THREE from "three";
import { LIFECYCLE_COLORS, RING_DEFS, ringByKey } from "./constants";
import { lifecycleOf } from "../data";
import { makeLabel } from "./label";
import type { LNode, NodeMesh } from "./sceneTypes";

export function addNodeMesh(n: LNode, worldGroup: THREE.Group): NodeMesh {
  const grp = new THREE.Group();
  grp.position.set(n._x ?? 0, 0, n._z ?? 0);

  const isBig = n.type === "system" || n.type === "container";
  const rad = isBig ? 3.2 : 2.4;
  const hgt = isBig ? 2.4 : 1.6;

  let geo: THREE.BufferGeometry;
  if (n.type === "system") geo = new THREE.CylinderGeometry(rad, rad * 0.92, hgt, 32);
  else if (n.type === "adr") geo = new THREE.BoxGeometry(rad * 1.3, hgt, rad * 1.3);
  else if (n.type === "rule") geo = new THREE.BoxGeometry(rad * 1.2, hgt, rad * 1.2);
  else if (n.type === "ref") geo = new THREE.CylinderGeometry(rad, rad, hgt, 8);
  else geo = new THREE.CylinderGeometry(rad, rad * 0.94, hgt, 32);

  const ringTint = ringByKey(n.ring || "infra").tint;
  const lc = lifecycleOf(n);
  const lcColor = LIFECYCLE_COLORS[lc] || LIFECYCLE_COLORS.open;

  const mat = new THREE.MeshStandardMaterial({
    color: new THREE.Color(ringTint),
    roughness: 0.62,
    metalness: 0.04,
    emissive: new THREE.Color(ringTint),
    emissiveIntensity: 0.0,
  });
  const mesh = new THREE.Mesh(geo, mat) as NodeMesh;
  mesh.position.y = hgt / 2;
  grp.add(mesh);

  // lifecycle halo ring (status glyph — AG-4)
  const halo = new THREE.Mesh(
    new THREE.TorusGeometry(rad * 1.05, 0.18, 8, 48),
    new THREE.MeshBasicMaterial({ color: new THREE.Color(lcColor), transparent: true, opacity: 0.85 }),
  );
  halo.rotation.x = Math.PI / 2;
  halo.position.y = hgt + 0.05;
  halo.userData.isHalo = true;
  halo.userData.baseOpacity = 0.85;
  grp.add(halo);

  if (n.staged || lc === "staged") {
    const stageCone = new THREE.Mesh(
      new THREE.ConeGeometry(0.38, 0.9, 6),
      new THREE.MeshBasicMaterial({ color: new THREE.Color("#d98a2b") }),
    );
    stageCone.position.y = hgt + 1.6;
    stageCone.userData.isStagedGlyph = true;
    grp.add(stageCone);
  }

  const sh = new THREE.Mesh(
    new THREE.CircleGeometry(rad * 1.5, 24),
    new THREE.MeshBasicMaterial({ color: 0x14161a, transparent: true, opacity: 0.07, depthWrite: false }),
  );
  sh.rotation.x = -Math.PI / 2;
  sh.position.y = 0.04;
  sh.userData.baseOpacity = 0.07;
  grp.add(sh);

  const lbl = makeLabel(n.title || n.id, { big: isBig });
  lbl.position.y = hgt + (isBig ? 3.0 : 2.2);
  grp.add(lbl);

  mesh.userData.node = n;
  mesh.userData.hasStatusGlyph = true; // every node gets a halo — AG-4 satisfied
  grp.userData = { node: n, mesh, mat, halo, hgt };
  n._grp = grp;
  n._mesh = mesh;
  n._mat = mat;
  n._hgt = hgt;

  worldGroup.add(grp);
  return mesh;
}

export function buildRingGuides(nodes: LNode[], worldGroup: THREE.Group): void {
  const usedRings = new Set<string>(nodes.map((n) => n.ring || "infra"));

  RING_DEFS.forEach((ring, i) => {
    if (!usedRings.has(ring.key)) return;
    const prev = RING_DEFS[i - 1];
    const next = RING_DEFS[i + 1];
    const outer = prev ? (prev.r + ring.r) / 2 : ring.r + 8;
    const inner = next ? (ring.r + next.r) / 2 : 0.001;

    const band = new THREE.Mesh(
      new THREE.RingGeometry(inner, outer, 96),
      new THREE.MeshBasicMaterial({
        color: ring.tint,
        transparent: true,
        opacity: 0.045,
        side: THREE.DoubleSide,
        depthWrite: false,
      }),
    );
    band.rotation.x = -Math.PI / 2;
    band.position.y = 0.01;
    worldGroup.add(band);

    const orb = new THREE.Mesh(
      new THREE.RingGeometry(ring.r - 0.1, ring.r + 0.1, 96),
      new THREE.MeshBasicMaterial({
        color: ring.tint,
        transparent: true,
        opacity: 0.18,
        side: THREE.DoubleSide,
        depthWrite: false,
      }),
    );
    orb.rotation.x = -Math.PI / 2;
    orb.position.y = 0.02;
    worldGroup.add(orb);

    const lbl = makeLabel(ring.label.toUpperCase(), { ring: true, color: ring.tint });
    lbl.position.set(Math.cos(2.5) * ring.r, 0.4, Math.sin(2.5) * ring.r);
    worldGroup.add(lbl);
  });
}
