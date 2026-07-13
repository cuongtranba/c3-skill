import type { Snapshot } from "../scene/ExplorerScene";

export function Tooltip({ snap }: { snap: Snapshot }) {
  if (!snap.tooltip) return null;
  return (
    <div className="c3-tooltip" style={{ left: snap.tooltip.x, top: snap.tooltip.y }}>
      {snap.tooltip.text}
    </div>
  );
}
