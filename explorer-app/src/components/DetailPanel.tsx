import { EDGE_COLORS, LIFECYCLE_COLORS } from "../scene/constants";
import type { EdgeRef, ExplorerScene, Snapshot } from "../scene/ExplorerScene";

function EdgeList({ refs, label }: { refs: EdgeRef[]; label: string }) {
  if (!refs.length) return null;
  return (
    <div className="c3-edge-list">
      <div className="h">{label}</div>
      {refs.map((r) => (
        <div key={r.id} className="row">
          {r.title}
        </div>
      ))}
    </div>
  );
}

export function DetailPanel({ scene, snap }: { scene: ExplorerScene; snap: Snapshot }) {
  const sel = snap.selection;
  if (!sel) return null;

  if (sel.kind === "edge") {
    return (
      <div className="c3-detail">
        <div className="c3-detail-head">
          <span className="c3-badge" style={{ color: EDGE_COLORS[sel.edgeKind] || "#7c8794" }}>
            EDGE · {sel.edgeKind.toUpperCase()}
          </span>
          <button className="c3-close" aria-label="Close" onClick={() => scene.clearSelection()}>
            ✕
          </button>
        </div>
        <h2 className="c3-detail-title">
          {sel.fromTitle} → {sel.toTitle}
        </h2>
        <div className="c3-detail-body">
          <div style={{ fontSize: "13.5px", color: "var(--ink)" }}>
            Kind: <b>{sel.edgeKind}</b>
          </div>
          {sel.crossedLabels.length ? (
            <div style={{ marginTop: 8, fontSize: "12.5px" }}>
              Crosses boundaries: {sel.crossedLabels.join(", ")}
            </div>
          ) : (
            <div style={{ marginTop: 8, fontSize: "12.5px" }}>Stays within one boundary layer.</div>
          )}
        </div>
      </div>
    );
  }

  const lcColor = LIFECYCLE_COLORS[sel.lifecycle] || LIFECYCLE_COLORS.open;
  return (
    <div className="c3-detail">
      <div className="c3-detail-head">
        <span className="c3-badge" style={{ background: lcColor + "22", color: lcColor }}>
          {sel.type.toUpperCase()}
        </span>
        <span className="c3-lifecycle" style={{ color: lcColor }}>
          {sel.lifecycle.toUpperCase()}
        </span>
        <button className="c3-close" aria-label="Close" onClick={() => scene.clearSelection()}>
          ✕
        </button>
      </div>
      <h2 className="c3-detail-title">{sel.title}</h2>
      <div className="c3-detail-body">
        {sel.goal && <div className="c3-detail-goal">{sel.goal}</div>}
        <div className="c3-detail-id">
          {sel.id}
          {sel.parent ? ` ∈ ${sel.parent}` : ""}
        </div>
        {sel.staged && sel.transition && (
          <div className="c3-staged-box">
            <div className="h">STAGED TRANSITION</div>
            <div className="t">
              {sel.transition.from} → {sel.transition.to}
            </div>
            <div className="b">by {sel.transition.by}</div>
            {sel.stagedBy && sel.stagedBy.length > 0 && (
              <div className="b" style={{ marginTop: 4 }}>
                staged by: {sel.stagedBy.join(", ")}
              </div>
            )}
          </div>
        )}
        <EdgeList refs={sel.contains} label="Contains" />
        <EdgeList refs={sel.uses} label="Uses" />
        <EdgeList refs={sel.usedBy} label="Used by" />
      </div>
    </div>
  );
}
