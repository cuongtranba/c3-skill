import * as THREE from "three";
import type { RoadStyle, SkinContext } from "../skin/types";
import { kindStyle } from "../skin/resolve";
import { disposeTree, emissive } from "../skin/kit";
import type { RouteObject } from "./DataRoadNetwork";

export type MotionMode = "active" | "all" | "none";

interface Packet {
  mesh: THREE.Mesh<THREE.CapsuleGeometry, THREE.MeshBasicMaterial>;
  t: number;
}

interface Lane {
  route: RouteObject;
  packets: Packet[];
}

const UP = new THREE.Vector3(0, 1, 0);

/* Packets are the only thing that travels. They ride the active flow by default, every
 * road on request, and nothing under reduced motion or "still". Channel glow tracks the
 * same mode so a lit route always means traffic. */
export class TrafficSystem {
  readonly group = new THREE.Group();
  mode: MotionMode = "active";
  private lanes: Lane[] = [];
  private readonly channel: RoadStyle["channel"];

  constructor(routes: RouteObject[], ctx: SkinContext) {
    const { packet, channel } = ctx.skin.roads;
    this.channel = channel;
    this.group.name = "traffic";
    this.group.userData.noExport = true;
    for (const route of routes) {
      const n = route.route.active ? 3 : 2;
      const packets: Packet[] = [];
      const mat = emissive(kindStyle(ctx.skin, route.route.kind).color, packet.glow);
      for (let i = 0; i < n; i++) {
        const mesh = new THREE.Mesh(new THREE.CapsuleGeometry(packet.radius, packet.length, 4, 10), mat);
        mesh.visible = false;
        this.group.add(mesh);
        packets.push({ mesh, t: i / n });
      }
      this.lanes.push({ route, packets });
    }
  }

  update(dt: number, reduced: boolean, hovered: RouteObject | null, highlighted: Set<string> | null): void {
    const { emissive: idle, activeBoost } = this.channel;
    for (const { route: o, packets } of this.lanes) {
      o.intensity += (o.targetIntensity - o.intensity) * 0.1;
      const lit = highlighted ? highlighted.has(o.route.id) : o.route.active;
      const activeMode = this.mode === "all" || (this.mode === "active" && lit);
      const dimAll = this.mode === "active" && !lit ? 0.35 : 1;
      const hv = hovered === o ? 2.0 : 1;
      const vis = o.hidden ? 0 : o.intensity;
      o.chan.visible = !o.hidden;
      o.chan.material.emissiveIntensity = idle * vis * dimAll * hv * (activeMode ? activeBoost : 1);
      const show = !reduced && !o.hidden && activeMode && this.mode !== "none" && o.intensity > 0.4;
      for (const p of packets) {
        p.mesh.visible = show;
        if (!show) continue;
        p.t = (p.t + (dt * 12) / o.len) % 1;
        p.mesh.position.copy(o.path.getPointAt(p.t));
        p.mesh.quaternion.setFromUnitVectors(UP, o.path.getTangentAt(p.t));
      }
    }
  }

  dispose(): void {
    disposeTree(this.group);
    this.lanes = [];
  }
}
