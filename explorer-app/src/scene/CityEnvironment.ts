import * as THREE from "three";
import type { C3Payload, District } from "../types";
import type { Extent, SkinContext } from "../skin/types";
import { disposeTree } from "../skin/kit";

export type { Extent } from "../skin/types";

/** World bounds from districts, node footprints and roads; the camera, shadows and ground follow it. */
export function payloadExtent(p: C3Payload): Extent {
  let minX = Infinity;
  let maxX = -Infinity;
  let minZ = Infinity;
  let maxZ = -Infinity;
  const take = (x0: number, x1: number, z0: number, z1: number): void => {
    minX = Math.min(minX, x0, x1);
    maxX = Math.max(maxX, x0, x1);
    minZ = Math.min(minZ, z0, z1);
    maxZ = Math.max(maxZ, z0, z1);
  };
  for (const d of p.districts) take(d.x - d.w / 2, d.x + d.w / 2, d.z - d.d / 2, d.z + d.d / 2);
  for (const n of p.nodes) take(n.layout.x - n.layout.w / 2, n.layout.x + n.layout.w / 2, n.layout.z - n.layout.d / 2, n.layout.z + n.layout.d / 2);
  for (const s of p.roads.streets) take(s.x0, s.x1, s.z, s.z);
  for (const a of p.roads.avenues) take(a.x, a.x, a.z0, a.z1);
  if (!isFinite(minX)) {
    minX = -60;
    maxX = 60;
    minZ = -40;
    maxZ = 40;
  }
  return { minX, maxX, minZ, maxZ, cx: (minX + maxX) / 2, cz: (minZ + maxZ) / 2, size: Math.max(maxX - minX, maxZ - minZ, 40) };
}

/* The environment owns the extent and the group structure the export and the inspector
 * tree rely on (`environment` → one group per district + `props`); what goes inside each
 * group is the skin's — ground, seams, platforms, masts, scenery. */
export class CityEnvironment {
  readonly group = new THREE.Group();
  readonly lights: THREE.Light[];
  readonly extent: Extent;

  constructor(payload: C3Payload, ctx: SkinContext) {
    this.lights = ctx.lights;
    this.group.name = "environment";
    this.group.userData.c3 = { id: "environment", type: "environment" };
    this.extent = payloadExtent(payload);
    ctx.skin.environment.ground(ctx, this.group, this.extent);
    for (const d of payload.districts) this.group.add(this.district(ctx, d));
    const props = new THREE.Group();
    props.name = "props";
    props.userData.c3 = { id: "props", type: "props" };
    ctx.skin.environment.props(ctx, props, payload);
    this.group.add(props);
  }

  private district(ctx: SkinContext, d: District): THREE.Group {
    const g = new THREE.Group();
    g.name = d.id;
    g.userData.c3 = { id: d.id, type: "district", kind: d.kind };
    ctx.skin.environment.district(ctx, g, d);
    return g;
  }

  dispose(): void {
    disposeTree(this.group);
  }
}
