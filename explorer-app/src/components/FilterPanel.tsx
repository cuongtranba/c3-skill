import { useMemo } from "react";
import type { CityScene, Snapshot } from "../scene/CityScene";
import { facetOptions, isEmptyFacets } from "../scene/facets";
import { ARCH_LABEL } from "../scene/constants";
import { boundaryKindColor } from "../skin";
import type { C3Payload } from "../data";

type ListKey = "type" | "district" | "boundary" | "boundaryKind" | "statusKey" | "lifecycle" | "archetype";

const GROUPS: { key: ListKey; title: string }[] = [
  { key: "statusKey", title: "Status" },
  { key: "district", title: "District" },
  { key: "type", title: "Type" },
  { key: "archetype", title: "Archetype" },
  { key: "boundary", title: "Zone" },
  { key: "boundaryKind", title: "Zone kind" },
  { key: "lifecycle", title: "Lifecycle" },
];

/* Facets: conjunctive, hide-only. Every row is a toggle; a lit swatch means the value is
 * currently required. Text and code-glob are free inputs. */
export function FilterPanel({ scene, snap, data }: { scene: CityScene; snap: Snapshot; data: C3Payload }) {
  const options = useMemo(() => facetOptions(data), [data]);
  const f = snap.facets;
  const empty = isEmptyFacets(f);
  const skin = scene.getSkin();
  const { status, palette } = skin.tokens;

  const swatch = (key: ListKey, value: string): string | undefined => {
    if (key === "statusKey") return status[value]?.color;
    if (key === "boundaryKind") return boundaryKindColor(skin, value);
    return undefined;
  };
  const label = (key: ListKey, value: string, fallback: string): string => {
    if (key === "archetype") return ARCH_LABEL[value] ?? value;
    if (key === "statusKey") return status[value]?.text.toLowerCase() ?? value;
    if (key === "district") return fallback.toLowerCase();
    return fallback;
  };

  return (
    <div className="c3-panel c3-card c3-filters">
      <h3>
        Filters
        {!empty && (
          <button className="c3-link" onClick={() => scene.clearFacets()}>
            clear
          </button>
        )}
      </h3>
      {options.flow.length > 0 && (
        <>
          <div className="c3-q">Flow</div>
          {options.flow.map((o) => (
            <div key={o.value} className={"c3-row c3-toggle" + (f.flow === o.value ? " on" : "")} onClick={() => scene.setFacets({ flow: f.flow === o.value ? "" : o.value })}>
              <span className="c3-bar" style={{ background: palette.violet }}></span>
              {o.label}
              <span className="c3-count">{o.count} steps</span>
            </div>
          ))}
        </>
      )}
      {GROUPS.map((g) => {
        const rows = options[g.key];
        if (rows.length < 1 || (rows.length === 1 && g.key !== "statusKey")) return null;
        const active = new Set(f[g.key] ?? []);
        return (
          <div key={g.key}>
            <div className="c3-q">{g.title}</div>
            {rows.map((o) => {
              const sw = swatch(g.key, o.value);
              return (
                <div key={o.value} className={"c3-row c3-toggle" + (active.has(o.value) ? " on" : "")} onClick={() => scene.toggleFacetValue(g.key, o.value)}>
                  <span className="c3-dot" style={{ background: sw ?? (active.has(o.value) ? palette.text : palette.line) }}></span>
                  <span className="c3-row-label">{label(g.key, o.value, o.label)}</span>
                  <span className="c3-count">{o.count}</span>
                </div>
              );
            })}
          </div>
        );
      })}
      <div className="c3-q">Code glob</div>
      <input
        className="c3-input"
        type="text"
        placeholder="cli/internal/**"
        spellCheck={false}
        value={f.codeGlob ?? ""}
        onChange={(e) => scene.setFacets({ codeGlob: e.target.value })}
      />
    </div>
  );
}
