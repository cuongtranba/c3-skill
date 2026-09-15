import * as THREE from "three";
import { consolidateStatic } from "./consolidate";
import type { C3Edge, C3Payload } from "../types";
import type { Skin, SkinContext, Spinner } from "../skin/types";
import { DEFAULT_SKIN, skinMaterials } from "../skin";
import { CityEnvironment } from "./CityEnvironment";
import { DataRoadNetwork } from "./DataRoadNetwork";
import { InfrastructureNode } from "./InfrastructureNode";
import { TrafficSystem } from "./TrafficSystem";

export interface WorldOptions {
  /** Canvas label sprites need a 2D context; tests build without them. */
  labels: boolean;
  skin?: Skin;
}

/* The whole built city for one payload under one skin: environment, buildings, roads,
 * traffic. Pure geometry — no renderer, no camera — so it can be assembled in a test and
 * exported. Positions, districts, roads, docks and routes all come from the payload;
 * nothing here lays anything out. */
export class CityWorld {
  readonly group = new THREE.Group();
  readonly payload: C3Payload;
  readonly skin: Skin;
  readonly env: CityEnvironment;
  readonly nodes = new Map<string, InfrastructureNode>();
  readonly roads: DataRoadNetwork;
  readonly traffic: TrafficSystem;
  readonly spinners: Spinner[] = [];
  readonly lights: THREE.Light[] = [];
  readonly pickMeshes: THREE.Object3D[] = [];
  readonly edgesById = new Map<string, C3Edge>();
  /** Undirected adjacency over every non-contains edge, for neighbourhood dimming. */
  readonly adjacency = new Map<string, Set<string>>();

  constructor(payload: C3Payload, opts: WorldOptions = { labels: true }) {
    this.payload = payload;
    this.skin = opts.skin ?? DEFAULT_SKIN;
    skinMaterials(this.skin);
    this.group.name = "c3-world";
    this.group.userData.c3 = { id: "world", type: "world", project: payload.project };
    for (const e of payload.edges) this.edgesById.set(e.id, e);
    for (const n of payload.nodes) this.adjacency.set(n.id, new Set());
    for (const e of payload.edges) {
      if (e.kind === "contains") continue;
      this.adjacency.get(e.from)?.add(e.to);
      this.adjacency.get(e.to)?.add(e.from);
    }

    const ctx: SkinContext = { skin: this.skin, m: this.skin.materials, palette: this.skin.tokens.palette, spinners: this.spinners, lights: this.lights, labels: opts.labels };

    this.env = new CityEnvironment(payload, ctx);
    // Platforms, lips, hazard marks, masts and props are static dressing; bake them per material.
    consolidateStatic(this.env.group, this.spinners);
    this.group.add(this.env.group);

    const buildings = new THREE.Group();
    buildings.name = "nodes";
    buildings.userData.c3 = { id: "nodes", type: "nodes" };
    for (const n of payload.nodes) {
      const b = new InfrastructureNode(n, this.edgesById, ctx, { labels: opts.labels });
      this.nodes.set(n.id, b);
      buildings.add(b.group);
      this.pickMeshes.push(...b.pickMeshes);
    }
    this.group.add(buildings);

    this.roads = new DataRoadNetwork(payload.roads, payload.routes, this.edgesById, ctx);
    this.group.add(this.roads.group);
    this.pickMeshes.push(...this.roads.pickMeshes);

    this.traffic = new TrafficSystem(this.roads.routes, ctx);
    this.group.add(this.traffic.group);
  }

  /** BFS hop distances from `id` over the adjacency, capped at 3. */
  distances(id: string): Map<string, number> {
    const d = new Map<string, number>([[id, 0]]);
    const q = [id];
    while (q.length) {
      const x = q.shift() as string;
      const dx = d.get(x) as number;
      if (dx >= 3) continue;
      for (const y of this.adjacency.get(x) ?? []) {
        if (!d.has(y)) {
          d.set(y, dx + 1);
          q.push(y);
        }
      }
    }
    return d;
  }

  /** Releases geometries and per-node materials; the skin's own materials survive for the next world. */
  dispose(): void {
    this.traffic.dispose();
    this.roads.dispose();
    for (const n of this.nodes.values()) n.dispose();
    this.env.dispose();
    for (const l of this.lights) l.dispose();
    this.group.clear();
  }
}
