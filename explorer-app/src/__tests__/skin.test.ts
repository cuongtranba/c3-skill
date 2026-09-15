import { afterEach, beforeEach, describe, expect, it } from "vitest";
import * as THREE from "three";
import { FIXTURE } from "../dev/fixture";
import { CityWorld } from "../scene/CityWorld";
import { InfrastructureNode } from "../scene/InfrastructureNode";
import { DEFAULT_SKIN, SKINS, applyChrome, boundaryKindColor, initialSkinId, kindStyle, lifecycleColor, persistSkinId, resolveSkin, skinMaterials, statusOf, type Skin, type SkinContext } from "../skin";

/** Every archetype the payload-v2 spec can emit. */
const ARCHETYPES = ["headquarters", "gatehouse", "comms", "command", "plant", "bunker", "transmit", "outpost", "record", "zone", "route"];

const ctxFor = (skin: Skin): SkinContext => ({ skin, m: skin.materials, palette: skin.tokens.palette, spinners: [], lights: [], labels: false });

const materialsIn = (root: THREE.Object3D): Set<THREE.Material> => {
  const out = new Set<THREE.Material>();
  root.traverse((o) => {
    const m = (o as THREE.Mesh).material as THREE.Material | THREE.Material[] | undefined;
    if (!m) return;
    for (const x of Array.isArray(m) ? m : [m]) out.add(x);
  });
  return out;
};

describe("skin registry", () => {
  it("lists industrial first as the default and resolves every id", () => {
    expect(SKINS.length).toBeGreaterThanOrEqual(2);
    expect(DEFAULT_SKIN.id).toBe("industrial");
    expect(SKINS[0]).toBe(DEFAULT_SKIN);
    for (const s of SKINS) expect(resolveSkin(s.id)).toBe(s);
    expect(new Set(SKINS.map((s) => s.id)).size).toBe(SKINS.length);
  });

  it("falls back to the default for unknown, empty and missing ids", () => {
    expect(resolveSkin("neon")).toBe(DEFAULT_SKIN);
    expect(resolveSkin("")).toBe(DEFAULT_SKIN);
    expect(resolveSkin(undefined)).toBe(DEFAULT_SKIN);
    expect(resolveSkin(null)).toBe(DEFAULT_SKIN);
  });
});

describe("skin selection order", () => {
  // This jsdom has no Web Storage; a Map-backed stand-in covers the persisted-choice stage.
  const store = new Map<string, string>();
  const memStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => store.clear(),
  };
  const reset = (): void => {
    delete window.C3_SKIN;
    window.history.replaceState({}, "", "/");
    store.clear();
  };
  beforeEach(() => {
    Object.defineProperty(window, "localStorage", { value: memStorage, configurable: true });
    reset();
  });
  afterEach(() => {
    reset();
    Object.defineProperty(window, "localStorage", { value: undefined, configurable: true });
  });

  it("defaults to industrial with nothing set", () => {
    expect(initialSkinId()).toBe("industrial");
  });

  it("reads localStorage when nothing else is set", () => {
    memStorage.setItem("c3v.skin", "blueprint");
    expect(initialSkinId()).toBe("blueprint");
  });

  it("prefers ?skin= over localStorage", () => {
    memStorage.setItem("c3v.skin", "blueprint");
    window.history.replaceState({}, "", "/?skin=industrial");
    expect(initialSkinId()).toBe("industrial");
  });

  it("prefers window.C3_SKIN over everything", () => {
    memStorage.setItem("c3v.skin", "industrial");
    window.history.replaceState({}, "", "/?skin=industrial");
    window.C3_SKIN = "blueprint";
    expect(initialSkinId()).toBe("blueprint");
  });

  it("skips unknown ids at every stage", () => {
    window.C3_SKIN = "nope";
    window.history.replaceState({}, "", "/?skin=also-nope");
    memStorage.setItem("c3v.skin", "blueprint");
    expect(initialSkinId()).toBe("blueprint");
    memStorage.setItem("c3v.skin", "still-nope");
    expect(initialSkinId()).toBe("industrial");
  });

  it("persistSkinId round-trips through storage", () => {
    persistSkinId("blueprint");
    expect(memStorage.getItem("c3v.skin")).toBe("blueprint");
    expect(initialSkinId()).toBe("blueprint");
  });

  it("applyChrome writes every CSS variable and the skin id onto the root element", () => {
    const root = document.createElement("div");
    const bp = resolveSkin("blueprint");
    applyChrome(bp, root);
    for (const [k, v] of Object.entries(bp.chrome)) expect(root.style.getPropertyValue(k)).toBe(v);
    expect(root.dataset.skin).toBe("blueprint");
  });
});

describe("every skin covers the vocabulary", () => {
  const fixtureKinds = new Set(FIXTURE.edges.map((e) => e.kind));
  const fixtureStatus = new Set(FIXTURE.nodes.map((n) => n.statusKey));
  const fixtureBoundaryKinds = new Set(FIXTURE.boundaries.map((b) => b.kind));
  const fixtureLifecycles = new Set<string>([...FIXTURE.nodes.map((n) => n.lifecycle), ...FIXTURE.events.map((e) => e.status)]);
  const fixtureDistrictKinds = new Set(FIXTURE.districts.map((d) => d.kind));

  for (const skin of SKINS) {
    it(`${skin.id}: a builder for every spec archetype`, () => {
      for (const a of ARCHETYPES) expect(typeof skin.archetypes[a], a).toBe("function");
    });

    it(`${skin.id}: a status, kind, boundary-kind, lifecycle and district colour for everything in the fixture`, () => {
      const hex = /^#[0-9a-fA-F]{6}$/;
      expect(skin.tokens.status.unchecked).toBeDefined();
      for (const k of fixtureStatus) {
        const st = statusOf(skin, { statusKey: k });
        expect(st.color, k).toMatch(hex);
        expect(st.text.length).toBeGreaterThan(0);
        expect(skin.tokens.status[k], `explicit status ${k}`).toBeDefined();
      }
      for (const k of fixtureKinds) {
        expect(kindStyle(skin, k).color, k).toMatch(hex);
        expect(skin.tokens.kinds[k], `explicit kind ${k}`).toBeDefined();
      }
      for (const k of fixtureBoundaryKinds) {
        expect(boundaryKindColor(skin, k), k).toMatch(hex);
        expect(skin.tokens.boundaryKinds[k], `explicit boundary kind ${k}`).toBeDefined();
      }
      for (const k of fixtureLifecycles) expect(lifecycleColor(skin, k), k).toMatch(hex);
      for (const k of fixtureDistrictKinds) expect(skin.tokens.districtTints[k], k).toMatch(hex);
      expect(skin.tokens.background).toMatch(hex);
      expect(skin.tokens.selection.color).toMatch(hex);
    });

    it(`${skin.id}: unknown kinds and boundary kinds resolve to a fallback, not undefined`, () => {
      expect(kindStyle(skin, "made_up").color).toBe(skin.tokens.palette.subtle);
      expect(kindStyle(skin, "made_up").label).toBe("made up");
      expect(boundaryKindColor(skin, "made-up")).toBe(skin.tokens.boundaryFallback);
      expect(statusOf(skin, { statusKey: "made-up" as never }).text).toBe(skin.tokens.status.unchecked.text);
    });

    it(`${skin.id}: chrome declares the variables styles.css relies on`, () => {
      for (const v of ["--bg", "--panel", "--panel-2", "--panel-glass", "--line", "--line-2", "--line-soft", "--text", "--muted", "--subtle", "--blue", "--blue-soft", "--cyan", "--cyan-soft", "--amber", "--red", "--sil", "--on-accent"]) {
        expect(skin.chrome[v], v).toBeTruthy();
      }
    });
  }
});

describe("blueprint skin builds the fixture", () => {
  const blueprint = resolveSkin("blueprint");
  const edgesById = new Map(FIXTURE.edges.map((e) => [e.id, e]));

  it("every fixture node becomes a group with at least one mesh and a status lamp", () => {
    const ctx = ctxFor(blueprint);
    for (const n of FIXTURE.nodes) {
      const b = new InfrastructureNode(n, edgesById, ctx, { labels: false });
      let meshes = 0;
      let lamps = 0;
      b.group.traverse((o) => {
        if ((o as THREE.Mesh).isMesh) meshes++;
        if (o.userData.statusLamp) lamps++;
      });
      expect(meshes, n.id).toBeGreaterThanOrEqual(1);
      expect(lamps, n.id).toBe(1);
      expect(b.pickMeshes.length, n.id).toBeGreaterThan(0);
      b.dispose();
    }
  });

  it("inks every building with edge lines and draws no props", () => {
    const world = new CityWorld(FIXTURE, { labels: false, skin: blueprint });
    let lines = 0;
    world.group.getObjectByName("nodes")!.traverse((o) => {
      if ((o as THREE.LineSegments).isLineSegments) lines++;
    });
    expect(lines).toBeGreaterThan(FIXTURE.nodes.length);
    expect(world.group.getObjectByName("props")!.children.length).toBe(0);
    expect(world.lights.length).toBe(0);
    expect([...world.nodes.keys()].sort()).toEqual(FIXTURE.nodes.map((n) => n.id).sort());
    expect(world.roads.routes.length).toBe(FIXTURE.routes.length);
    world.dispose();
  });
});

describe("skins never share material instances", () => {
  it("a world built with skin B contains no material owned by skin A, in both directions", () => {
    for (const a of SKINS) {
      for (const b of SKINS) {
        if (a === b) continue;
        const world = new CityWorld(FIXTURE, { labels: false, skin: b });
        const used = materialsIn(world.group);
        const foreign = skinMaterials(a);
        for (const m of used) expect(foreign.has(m), `${a.id} material in ${b.id} world`).toBe(false);
        // …and the world does use its own skin's slots.
        let own = 0;
        for (const m of used) if (skinMaterials(b).has(m)) own++;
        expect(own).toBeGreaterThan(0);
        world.dispose();
      }
    }
  });

  it("disposing a world leaves the skin's own materials intact", () => {
    const skin = resolveSkin("industrial");
    let disposed = 0;
    const off: (() => void)[] = [];
    for (const m of skinMaterials(skin)) {
      const fn = (): void => {
        disposed++;
      };
      m.addEventListener("dispose", fn);
      off.push(() => m.removeEventListener("dispose", fn));
    }
    const world = new CityWorld(FIXTURE, { labels: false, skin });
    world.dispose();
    off.forEach((f) => f());
    expect(disposed).toBe(0);
  });
});
