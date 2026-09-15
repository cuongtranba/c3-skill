import type { Snapshot } from "../scene/CityScene";

export function Tooltip({ snap }: { snap: Snapshot }) {
  if (!snap.tooltip) return null;
  return (
    <div className="c3-tooltip" style={{ left: snap.tooltip.x, top: snap.tooltip.y }}>
      <b>{snap.tooltip.text}</b>
      {snap.tooltip.sub && <span> · {snap.tooltip.sub}</span>}
    </div>
  );
}
