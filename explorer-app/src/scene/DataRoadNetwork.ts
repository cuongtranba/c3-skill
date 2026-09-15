import * as THREE from "three";
import type { C3Edge, Roads, Route } from "../types";
import type { RoadStyle, SkinContext } from "../skin/types";
import { kindStyle } from "../skin/resolve";
import { disposeTree, emissive, strip } from "../skin/kit";

/* Roads are ribbons the skin dresses: an optional shoulder, a sunken channel, edge rails
 * and marker lamps on major trunks. Every ribbon comes from payload.roads; every route
 * channel rides payload.routes[].waypoints (lane offsets already applied by Go). */

export interface RouteObject {
  route: Route;
  edge: C3Edge | undefined;
  path: THREE.CurvePath<THREE.Vector3>;
  chan: THREE.Mesh<THREE.TubeGeometry, THREE.MeshStandardMaterial>;
  len: number;
  intensity: number;
  targetIntensity: number;
  hidden: boolean;
}

function trench(style: RoadStyle, group: THREE.Group, cx: number, cz: number, len: number, width: number, horizontal: boolean, major: boolean): void {
  const dims = (l: number, w: number): [number, number] => (horizontal ? [l, w] : [w, l]);
  if (style.shoulder) {
    const [rw, rd] = dims(len, width + (major ? 2.4 : 1.0));
    const rib = new THREE.Mesh(new THREE.BoxGeometry(rw, 0.06, rd), style.shoulder);
    rib.position.set(cx, 0.03, cz);
    rib.receiveShadow = true;
    group.add(rib);
  }
  if (style.trench) {
    const [tw, td] = dims(len, width);
    const tr = new THREE.Mesh(new THREE.BoxGeometry(tw, 0.05, td), style.trench);
    tr.position.set(cx, 0.045, cz);
    group.add(tr);
  }
  if (style.rails) {
    for (const s of [-1, 1]) {
      const off = s * (width / 2 + 0.07);
      const [ww, wd] = dims(len, 0.14);
      group.add(strip(ww, 0.08, wd, cx + (horizontal ? 0 : off), 0.1, cz + (horizontal ? off : 0), style.rails));
    }
  }
  if (major && style.markerLamp) {
    const lampMat = emissive(style.markerLamp.color, style.markerLamp.intensity);
    const start = (horizontal ? cx : cz) - len / 2 + 4;
    for (let p = start; p < start + len - 4; p += 12) {
      for (const s of [-1, 1]) {
        const off = s * (width / 2 + 0.7);
        group.add(horizontal ? strip(0.4, 0.03, 0.2, p, 0.09, cz + off, lampMat) : strip(0.2, 0.03, 0.4, cx + off, 0.09, p, lampMat));
      }
    }
  }
}

/** Rounds every interior corner of a polyline so channels read as bent cable, not wire. */
export function roundedPath(pts: THREE.Vector3[], r = 2.2): THREE.CurvePath<THREE.Vector3> {
  const path = new THREE.CurvePath<THREE.Vector3>();
  let prev = pts[0];
  for (let i = 1; i < pts.length - 1; i++) {
    const p = pts[i];
    const next = pts[i + 1];
    const din = p.clone().sub(prev);
    const dout = next.clone().sub(p);
    const rin = Math.min(r, din.length() / 2);
    const rout = Math.min(r, dout.length() / 2);
    if (din.lengthSq() < 1e-8 || dout.lengthSq() < 1e-8) continue;
    const a = p.clone().sub(din.normalize().multiplyScalar(rin));
    const b = p.clone().add(dout.normalize().multiplyScalar(rout));
    if (a.distanceToSquared(prev) > 1e-8) path.add(new THREE.LineCurve3(prev, a));
    path.add(new THREE.QuadraticBezierCurve3(a, p, b));
    prev = b;
  }
  const last = pts[pts.length - 1];
  if (last.distanceToSquared(prev) > 1e-8 || path.curves.length === 0) path.add(new THREE.LineCurve3(prev, last));
  return path;
}

export class DataRoadNetwork {
  readonly group = new THREE.Group();
  readonly routes: RouteObject[] = [];
  readonly byEdgeId = new Map<string, RouteObject>();
  readonly pickMeshes: THREE.Object3D[] = [];

  constructor(roads: Roads, routes: Route[], edgesById: Map<string, C3Edge>, ctx: SkinContext) {
    const style = ctx.skin.roads;
    this.group.name = "roads";
    this.group.userData.c3 = { id: "roads", type: "roads" };
    for (const s of roads.streets) trench(style, this.group, (s.x0 + s.x1) / 2, s.z, Math.abs(s.x1 - s.x0), s.width, true, s.major);
    for (const a of roads.avenues) trench(style, this.group, a.x, (a.z0 + a.z1) / 2, Math.abs(a.z1 - a.z0), a.width, false, false);

    for (const r of routes) {
      if (r.waypoints.length < 2) continue;
      const pts = r.waypoints.map(([x, y, z]) => new THREE.Vector3(x, y, z));
      const path = roundedPath(pts);
      const len = path.getLength();
      const col = new THREE.Color(kindStyle(ctx.skin, r.kind).color);
      const segments = Math.max(24, Math.min(200, Math.round(len * 1.2)));
      const chan = new THREE.Mesh(
        new THREE.TubeGeometry(path, segments, style.channel.radius, 8, false),
        new THREE.MeshStandardMaterial({ color: col.clone().multiplyScalar(style.channel.bodyScale), emissive: col, emissiveIntensity: style.channel.emissive, roughness: 0.5 }),
      );
      chan.userData.routeId = r.id;
      chan.userData.noExport = true;
      this.group.add(chan);
      this.pickMeshes.push(chan);
      // The exportable twin: one Line per route over the payload waypoints, as the Go export emits.
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(pts), new THREE.LineBasicMaterial({ color: col }));
      line.name = r.id;
      line.visible = false;
      line.userData.c3 = { id: r.id, type: "route", kind: r.kind, edgeId: r.edgeId };
      this.group.add(line);
      const obj: RouteObject = { route: r, edge: edgesById.get(r.edgeId), path, chan, len, intensity: 1, targetIntensity: 1, hidden: false };
      this.routes.push(obj);
      this.byEdgeId.set(r.edgeId, obj);
    }
  }

  dispose(): void {
    disposeTree(this.group);
  }
}
