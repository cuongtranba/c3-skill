import { EDGE_COLORS, LIFECYCLE_COLORS } from "../scene/constants";
import type { ExplorerScene, Snapshot } from "../scene/ExplorerScene";

const EDGE_ROWS: { kind: string; label: string }[] = [
  { kind: "contains", label: "contains" },
  { kind: "uses", label: "uses" },
  { kind: "affects", label: "affects (change-unit)" },
];

export function Legend({ scene, snap }: { scene: ExplorerScene; snap: Snapshot }) {
  const dimmed = new Set(snap.dimmed);
  return (
    <div className="c3-legend">
      <div className="c3-card">
        <div className="c3-card-title">Lifecycle · tap to filter</div>
        <div>
          {Object.keys(LIFECYCLE_COLORS)
            .filter((lc) => snap.lifecycleCounts[lc])
            .map((lc) => (
              <div
                key={lc}
                className={"c3-legend-row" + (dimmed.has(lc) ? " off" : "")}
                onClick={() => scene.toggleLifecycle(lc)}
              >
                <span className="c3-swatch dot" style={{ background: LIFECYCLE_COLORS[lc] }}></span>
                {lc}
                <span className="c3-legend-count">{snap.lifecycleCounts[lc]}</span>
              </div>
            ))}
        </div>
      </div>
      <div className="c3-card">
        <div className="c3-card-title">Edges</div>
        <div>
          {EDGE_ROWS.map((r) => (
            <div key={r.kind} className="c3-legend-row">
              <span className="c3-edge-swatch" style={{ background: EDGE_COLORS[r.kind] }}></span>
              {r.label}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
