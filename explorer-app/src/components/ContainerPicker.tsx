import type { C3Payload } from "../data";
import type { ExplorerScene, Snapshot } from "../scene/ExplorerScene";

export function ContainerPicker({ scene, snap, data }: { scene: ExplorerScene; snap: Snapshot; data: C3Payload }) {
  if (snap.level !== "component") return null;
  const containers = (data.nodes || []).filter((n) => n.type === "container");
  return (
    <div className="c3-containers show">
      {containers.map((c) => (
        <button
          key={c.id}
          className={snap.focusContainer === c.id ? "active" : ""}
          onClick={() => scene.setFocusContainer(c.id)}
        >
          {c.title || c.id}
        </button>
      ))}
    </div>
  );
}
