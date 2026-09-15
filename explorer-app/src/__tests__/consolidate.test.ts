import * as THREE from "three";
import { describe, expect, it } from "vitest";
import { consolidateStatic } from "../scene/consolidate";
import { FIXTURE } from "../dev/fixture";
import { CityWorld } from "../scene/CityWorld";
import { resolveSkin } from "../skin";

describe("consolidateStatic", () => {
  it("bakes static meshes per shared material and leaves animated / per-node parts alone", () => {
    const shared = new THREE.MeshStandardMaterial();
    const perNode = new THREE.MeshBasicMaterial();
    const root = new THREE.Group();
    for (let i = 0; i < 10; i++) {
      const m = new THREE.Mesh(new THREE.BoxGeometry(1, 1, 1), shared);
      m.position.set(i * 2, 0, 0);
      root.add(m);
    }
    const accent = new THREE.Mesh(new THREE.BoxGeometry(1, 1, 1), perNode); // a lone per-node emitter stays
    root.add(accent);
    const blades = new THREE.Group();
    blades.add(new THREE.Mesh(new THREE.BoxGeometry(2, 0.1, 0.2), shared));
    root.add(blades);
    const lamp = new THREE.Mesh(new THREE.SphereGeometry(0.2), shared);
    lamp.userData.blink = true;
    root.add(lamp);

    const removed = consolidateStatic(root, [{ obj: blades, axis: "y", speed: 1 }]);

    expect(removed).toBe(10);
    const meshes: THREE.Mesh[] = [];
    root.traverse((o) => { if ((o as THREE.Mesh).isMesh) meshes.push(o as THREE.Mesh); });
    // 1 merged shell + accent + blade + lamp
    expect(meshes).toHaveLength(4);
    const merged = meshes.find((m) => m.userData.merged === 10)!;
    expect(merged.material).toBe(shared);
    expect(merged.geometry.getAttribute("position").count).toBe(10 * 24);
    // world-space placement is preserved: the merged shell spans the row of boxes
    merged.geometry.computeBoundingBox();
    expect(merged.geometry.boundingBox!.min.x).toBeCloseTo(-0.5);
    expect(merged.geometry.boundingBox!.max.x).toBeCloseTo(18.5);
    expect(root.children).toContain(accent);
    expect(root.children).toContain(blades);
    expect(root.children).toContain(lamp);
  });

  it("cuts a fixture city's mesh count by an order of magnitude without losing nodes", () => {
    const world = new CityWorld(FIXTURE, { skin: resolveSkin("industrial"), labels: false });
    // Count what actually draws: meshes not under a hidden group (selection FX is hidden until selected).
    let drawn = 0;
    const walk = (o: THREE.Object3D) => {
      if (!o.visible) return;
      if ((o as THREE.Mesh).isMesh) drawn++;
      for (const c of o.children) walk(c);
    };
    let merged = 0;
    for (const n of world.nodes.values()) {
      walk(n.group);
      merged += n.mergedParts;
    }
    expect(world.nodes.size).toBe(FIXTURE.nodes.length);
    // The authored parts still exist as picking/export-visible meshes only where they
    // move or carry a per-node material; everything else collapsed into shells.
    expect((drawn + merged) / drawn).toBeGreaterThan(3);
    expect(drawn / FIXTURE.nodes.length).toBeLessThanOrEqual(16);
  });
});
