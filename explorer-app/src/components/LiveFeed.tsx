import type { ActionEvent } from "../live/liveClient";
import type { LastUpdate } from "../scene/ExplorerScene";

function timeOf(ts: string): string {
  const d = new Date(ts);
  return isNaN(d.getTime()) ? "" : d.toLocaleTimeString([], { hour12: false });
}

export function LiveFeed({ items, lastUpdate }: { items: ActionEvent[]; lastUpdate: LastUpdate | null }) {
  return (
    <div className="c3-card c3-feed">
      <div className="c3-card-title">
        Live actions
        {lastUpdate && (
          <span className="c3-feed-delta">
            {lastUpdate.added ? ` +${lastUpdate.added}` : ""}
            {lastUpdate.changed ? ` ~${lastUpdate.changed}` : ""}
            {lastUpdate.removed ? ` −${lastUpdate.removed}` : ""}
          </span>
        )}
      </div>
      {items.length === 0 && <div className="c3-feed-empty">Waiting for c3x commands…</div>}
      {items.map((it, i) => (
        <div key={items.length - i} className={"c3-feed-row" + (it.success ? "" : " failed")}>
          <span className="c3-feed-time">{timeOf(it.ts)}</span>
          <span className={"c3-feed-dot" + (it.mutating ? " mut" : "")}></span>
          <span className="c3-feed-cmd" title={[it.cmd, ...(it.args || [])].join(" ")}>
            {it.cmd} {(it.args || []).join(" ")}
          </span>
        </div>
      ))}
    </div>
  );
}
