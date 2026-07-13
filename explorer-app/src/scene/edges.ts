import * as THREE from "three";
import { EDGE_COLORS, RING_DEFS } from "./constants";
import type { C3Edge } from "../data";
import type { BasicMesh, EdgeRec, LNode } from "./sceneTypes";

export interface EdgeBuild {
  pickTube: BasicMesh;
  rec: EdgeRec;
}

export function addEdgeMesh(
  e: C3Edge,
  nodeById: Record<string, LNode>,
  worldGroup: THREE.Group,
): EdgeBuild | null {
  const a = nodeById[e.from],
    b = nodeById[e.to];
  if (!a || !b) return null;

  const kind = e.kind || "uses";
  const col = new THREE.Color(EDGE_COLORS[kind] || EDGE_COLORS.uses);

  const ax = a._x ?? 0,
    az = a._z ?? 0,
    bx = b._x ?? 0,
    bz = b._z ?? 0;
  const p1 = new THREE.Vector3(ax, (a._hgt || 1.5) * 0.7, az);
  const p2 = new THREE.Vector3(bx, (b._hgt || 1.5) * 0.7, bz);
  const dist = p1.distanceTo(p2);

  // boundary-gate CatmullRom routing
  const pa = { r: Math.hypot(ax, az), t: Math.atan2(az, ax) };
  const pb = { r: Math.hypot(bx, bz), t: Math.atan2(bz, bx) };
  let dtt = pb.t - pa.t;
  while (dtt > Math.PI) dtt -= 2 * Math.PI;
  while (dtt < -Math.PI) dtt += 2 * Math.PI;
  const lo = Math.min(pa.r, pb.r),
    hi = Math.max(pa.r, pb.r);
  const crossed = RING_DEFS.filter((rr) => rr.r > lo + 1.4 && rr.r < hi - 1.4);
  const gatePts = crossed
    .map((rr) => {
      const f = (rr.r - pa.r) / (pb.r - pa.r || 0.001);
      const ang = pa.t + dtt * f;
      return { f, x: Math.cos(ang) * rr.r, z: Math.sin(ang) * rr.r, label: rr.label };
    })
    .sort((u, v) => u.f - v.f);

  const peak = Math.min(2.2 + crossed.length * 1.4 + dist * 0.045, 10);
  let curve: THREE.Curve<THREE.Vector3>;
  if (gatePts.length) {
    const pts = [p1];
    gatePts.forEach((gp) => pts.push(new THREE.Vector3(gp.x, 1.0 + peak * Math.sin(Math.PI * gp.f), gp.z)));
    pts.push(p2);
    curve = new THREE.CatmullRomCurve3(pts, false, "centripetal", 0.7);
  } else {
    const mid = p1.clone().add(p2).multiplyScalar(0.5);
    mid.y += Math.min(2.4 + dist * 0.18, 10);
    curve = new THREE.QuadraticBezierCurve3(p1, mid, p2);
  }

  const tubeOpacity = kind === "contains" ? 0.22 : 0.32;
  const tubeRadius = kind === "contains" ? 0.055 : 0.08;

  const tube = new THREE.Mesh(
    new THREE.TubeGeometry(curve, 32, tubeRadius, 6, false),
    new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: tubeOpacity }),
  ) as BasicMesh;
  worldGroup.add(tube);

  const arrowMeshes: BasicMesh[] = [];
  const addArrow = (t: number, flip: boolean) => {
    const pos = curve.getPoint(t);
    const tan = curve.getTangent(t);
    if (flip) tan.negate();
    const cone = new THREE.Mesh(
      new THREE.ConeGeometry(0.38, 1.05, 8),
      new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: 0.8 }),
    ) as BasicMesh;
    cone.position.copy(pos);
    cone.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), tan);
    worldGroup.add(cone);
    arrowMeshes.push(cone);
  };
  if (kind !== "contains") addArrow(0.88, false);

  const gates = gatePts.map((gp) => {
    const gm = new THREE.Mesh(
      new THREE.RingGeometry(0.3, 0.58, 20),
      new THREE.MeshBasicMaterial({
        color: col,
        transparent: true,
        opacity: 0.5,
        side: THREE.DoubleSide,
        depthWrite: false,
      }),
    ) as BasicMesh;
    gm.rotation.x = -Math.PI / 2;
    gm.position.set(gp.x, 0.07, gp.z);
    worldGroup.add(gm);
    return gm;
  });

  const particles: BasicMesh[] = [];
  if (kind === "uses" || kind === "affects") {
    const pc = 2;
    const pgeo = new THREE.SphereGeometry(0.22, 8, 8);
    for (let i = 0; i < pc; i++) {
      const pm = new THREE.Mesh(pgeo, new THREE.MeshBasicMaterial({ color: col })) as BasicMesh;
      pm.userData.t = i / pc;
      pm.userData.speed = 0.09 + Math.random() * 0.03;
      worldGroup.add(pm);
      particles.push(pm);
    }
  }

  const pickTube = new THREE.Mesh(
    new THREE.TubeGeometry(curve, 20, 0.55, 4, false),
    new THREE.MeshBasicMaterial({ visible: false }),
  ) as BasicMesh;
  const rec: EdgeRec = {
    e,
    curve,
    tube,
    arrowMeshes,
    particles,
    gates,
    kind,
    from: e.from,
    to: e.to,
    baseOpacity: tubeOpacity,
    crossedLabels: gatePts.map((g) => g.label),
  };
  pickTube.userData.rec = rec;
  worldGroup.add(pickTube);
  return { pickTube, rec };
}
