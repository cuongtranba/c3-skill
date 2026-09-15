import { describe, expect, it } from "vitest";
import { FIXTURE } from "../dev/fixture";
import { applyFacets, codeGlobMatches, facetOptions, flowRouteIds, globMatch, isEmptyFacets } from "../scene/facets";

const ALL = FIXTURE.nodes.map((n) => n.id);

describe("facets", () => {
  it("no facets shows every node", () => {
    expect([...applyFacets(FIXTURE, {})].sort()).toEqual([...ALL].sort());
    expect(isEmptyFacets({})).toBe(true);
    expect(isEmptyFacets({ type: [], text: "" })).toBe(true);
    expect(isEmptyFacets({ type: ["rule"] })).toBe(false);
  });

  it("type filters to the listed fact types", () => {
    expect([...applyFacets(FIXTURE, { type: ["rule", "ref"] })].sort()).toEqual(["ref-frontmatter-docs", "rule-wrap-error-cause"]);
  });

  it("district keeps the sector's buildings and drops zone/route nodes (district is empty for them)", () => {
    const ids = applyFacets(FIXTURE, { district: ["c3-1"] });
    expect(ids.has("c3-112")).toBe(true);
    expect(ids.has("c3-1")).toBe(true);
    expect(ids.has("c3-0")).toBe(false);
    expect(ids.has("boundary-store-write")).toBe(false);
  });

  it("boundary id keeps members plus the zone itself; boundary kind resolves through the zone node", () => {
    expect([...applyFacets(FIXTURE, { boundary: ["boundary-store-write"] })].sort()).toEqual(["boundary-store-write", "c3-102", "c3-104", "c3-112"]);
    expect([...applyFacets(FIXTURE, { boundaryKind: ["security"] })].sort()).toEqual(["boundary-store-write", "c3-102", "c3-104", "c3-112"]);
    expect(applyFacets(FIXTURE, { boundaryKind: ["network"] }).size).toBe(0);
  });

  it("statusKey, lifecycle and archetype are exact-match lists", () => {
    expect([...applyFacets(FIXTURE, { statusKey: ["drift", "review"] })].sort()).toEqual(["c3-103", "c3-104"]);
    expect([...applyFacets(FIXTURE, { lifecycle: ["staged"] })]).toEqual(["c3-112"]);
    expect([...applyFacets(FIXTURE, { archetype: ["bunker", "comms"] })].sort()).toEqual(["c3-102", "c3-109"]);
  });

  it("flow keeps its step endpoints and the flow node, and names the routes to light", () => {
    expect([...applyFacets(FIXTURE, { flow: "flow-change-apply" })].sort()).toEqual(["c3-102", "c3-104", "c3-109", "c3-112", "flow-change-apply"]);
    const routes = flowRouteIds(FIXTURE, "flow-change-apply");
    expect(routes.has("c3-109→c3-112#flow_step")).toBe(true);
    // A flow_step sharing a depends_on route lights that route too.
    expect(routes.has("c3-109→c3-112#depends_on")).toBe(true);
    expect(routes.has("c3-114→c3-102#depends_on")).toBe(false);
  });

  it("text searches title, id, type, goal and tech, case-insensitively", () => {
    expect([...applyFacets(FIXTURE, { text: "SQLite" })]).toEqual(["c3-102"]);
    expect([...applyFacets(FIXTURE, { text: "c3-11" })].sort()).toEqual(["c3-112", "c3-114"]);
    expect(applyFacets(FIXTURE, { text: "typescript" }).has("c3-114")).toBe(true);
  });

  it("code glob matches bindings by glob or path in either direction", () => {
    expect(globMatch("cli/internal/**", "cli/internal/store/store.go")).toBe(true);
    expect(globMatch("cli/cmd/change*.go", "cli/cmd/change_apply.go")).toBe(true);
    expect(globMatch("cli/cmd/change*.go", "cli/cmd/other.go")).toBe(false);
    expect(globMatch("cli/*.go", "cli/cmd/x.go")).toBe(false);
    const store = FIXTURE.nodes.find((n) => n.id === "c3-102")!;
    expect(codeGlobMatches(store, "cli/internal/store/db.go")).toBe(true);
    expect(codeGlobMatches(store, "cli/internal/**")).toBe(true);
    expect(codeGlobMatches(store, "explorer-app/**")).toBe(false);
    expect([...applyFacets(FIXTURE, { codeGlob: "explorer-app/src/**" })]).toEqual(["c3-114"]);
  });

  it("facets are conjunctive", () => {
    expect([...applyFacets(FIXTURE, { district: ["c3-1"], statusKey: ["stable"], archetype: ["plant", "bunker"] })]).toEqual(["c3-102"]);
    expect(applyFacets(FIXTURE, { type: ["rule"], district: ["c3-1"] }).size).toBe(0);
  });

  it("facetOptions lists distinct values with counts", () => {
    const o = facetOptions(FIXTURE);
    expect(o.type.find((r) => r.value === "component")?.count).toBe(6);
    expect(o.district.find((r) => r.value === "c3-1")?.label).toBe("SECTOR 01 · CLI");
    expect(o.flow).toEqual([{ value: "flow-change-apply", label: "change apply", count: 3 }]);
    expect(o.boundaryKind).toEqual([{ value: "security", label: "security", count: 3 }]);
  });
});
