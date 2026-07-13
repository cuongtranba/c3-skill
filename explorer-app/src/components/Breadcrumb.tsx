import type { C3Payload } from "../data";
import type { Snapshot } from "../scene/ExplorerScene";

const LEVEL_LABELS: Record<string, string> = {
  context: "C1 · Context",
  container: "C2 · Containers",
  component: "C3 · Components",
};

export function Breadcrumb({ snap, data }: { snap: Snapshot; data: C3Payload }) {
  let t = (data.project || "c3") + " / " + (LEVEL_LABELS[snap.level] || snap.level);
  if (snap.level === "component" && snap.focusContainer) {
    const cn = (data.nodes || []).find((n) => n.id === snap.focusContainer);
    t += " / " + (cn ? cn.title || cn.id : snap.focusContainer);
  }
  if (snap.selection && snap.selection.kind === "node") {
    t += "  ›  " + snap.selection.title;
  }
  return <div className="c3-crumb">{t}</div>;
}
