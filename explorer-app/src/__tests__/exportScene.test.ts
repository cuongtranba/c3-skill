import { describe, expect, it } from "vitest";
import * as THREE from "three";
import { FIXTURE } from "../dev/fixture";
import { CityWorld } from "../scene/CityWorld";
import { collectC3Ids, sceneToJSON, sceneToObject } from "../scene/exportScene";
import { buildSceneTree, countTree } from "../scene/sceneTree";

// jsdom has no WebGL and no 2D canvas; the world is pure geometry so it builds without either.
const world = new CityWorld(FIXTURE, { labels: false });

describe("exportScene", () => {
  it("round-trips through ObjectLoader with every payload node id in userData.c3", () => {
    const json = sceneToJSON(world.group);
    expect(json.metadata.type).toBe("Object");
    const text = JSON.stringify(json);
    const loaded = new THREE.ObjectLoader().parse(JSON.parse(text));
    const ids = collectC3Ids(loaded);
    for (const n of FIXTURE.nodes) expect(ids.has(n.id)).toBe(true);
    for (const d of FIXTURE.districts) expect(ids.has(d.id)).toBe(true);
    for (const r of FIXTURE.routes) expect(ids.has(r.id)).toBe(true);
    const nodeIds = [...collectC3Ids(loaded)].filter((id) => FIXTURE.nodes.some((n) => n.id === id));
    expect(nodeIds.sort()).toEqual(FIXTURE.nodes.map((n) => n.id).sort());
  });

  it("strips noExport objects (labels, selection FX, traffic, lights, helpers) and keeps positions", () => {
    const exported = sceneToObject(world.group);
    let lights = 0;
    let noExport = 0;
    let sprites = 0;
    exported.traverse((o) => {
      if ((o as THREE.Light).isLight) lights++;
      if (o.userData.noExport) noExport++;
      if ((o as THREE.Sprite).isSprite) sprites++;
    });
    expect(lights).toBe(0);
    expect(noExport).toBe(0);
    expect(sprites).toBe(0);
    expect(exported.getObjectByName("traffic")).toBeUndefined();
    const hq = exported.getObjectByName("c3-0")!;
    const hqNode = FIXTURE.nodes.find((n) => n.id === "c3-0")!;
    expect(hq.position.x).toBeCloseTo(hqNode.layout.x);
    expect(hq.position.z).toBeCloseTo(hqNode.layout.z);
    // The live world still has its FX.
    let liveNoExport = 0;
    world.group.traverse((o) => {
      if (o.userData.noExport) liveNoExport++;
    });
    expect(liveNoExport).toBeGreaterThan(0);
  });

  it("builds one route channel per payload route and a dock terminal per payload dock", () => {
    expect(world.roads.routes.length).toBe(FIXTURE.routes.length);
    const dockCount = FIXTURE.nodes.reduce((s, n) => s + (n.docks?.length ?? 0), 0);
    let terminals = 0;
    world.group.traverse((o) => {
      if (o.userData.dock) terminals++;
    });
    expect(terminals).toBe(dockCount);
  });
});

describe("sceneTree", () => {
  it("walks the world into districts, nodes and roads with collapsed parts", () => {
    const tree = buildSceneTree(world.group);
    expect(tree.name).toBe("c3-world");
    const names = tree.children.map((c) => c.name);
    expect(names).toEqual(["environment", "nodes", "roads"]);
    const nodes = tree.children.find((c) => c.name === "nodes")!;
    expect(nodes.children.map((c) => c.c3?.id).sort()).toEqual(FIXTURE.nodes.map((n) => n.id).sort());
    const hq = nodes.children.find((c) => c.c3?.id === "c3-0")!;
    expect(hq.c3?.archetype).toBe("headquarters");
    expect(hq.parts).toBeGreaterThan(10);
    expect(hq.children.length).toBe(0);
    const env = tree.children.find((c) => c.name === "environment")!;
    expect(env.children.map((c) => c.c3?.id)).toEqual(expect.arrayContaining(["c3-1", "hq", "governance", "props"]));
    const roads = tree.children.find((c) => c.name === "roads")!;
    // one row per route plus the baked "ribbons" (trenches, shoulders, rails, marker lamps)
    expect(roads.children.length).toBe(FIXTURE.routes.length + 1);
    expect(roads.children.some((c) => c.name === "ribbons")).toBe(true);
    const { nodes: total, parts } = countTree(tree);
    expect(total).toBeGreaterThan(FIXTURE.nodes.length);
    expect(parts).toBeGreaterThan(100);
  });
});
