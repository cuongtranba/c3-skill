import { useEffect, useRef } from "react";
import { LIFECYCLE_COLORS } from "../scene/constants";
import type { C3Payload } from "../data";
import type { ExplorerScene, Snapshot } from "../scene/ExplorerScene";

export function KaraokeList({ scene, snap, data }: { scene: ExplorerScene; snap: Snapshot; data: C3Payload }) {
  const listRef = useRef<HTMLDivElement>(null);
  const { active, index, speed } = snap.timeline;

  useEffect(() => {
    if (!active || !listRef.current) return;
    const cur = listRef.current.children[index];
    if (cur) cur.scrollIntoView({ block: "center", behavior: speed >= 4 ? "auto" : "smooth" });
  }, [active, index, speed]);

  if (!active) return null;
  const events = data.events || [];

  return (
    <div className="c3-tl-list" ref={listRef}>
      {events.map((ev, i) => {
        const color = LIFECYCLE_COLORS[ev.status] || "#6b7280";
        const date = !ev.date || ev.date === "0000-00-00" ? "genesis" : ev.date;
        const c = (ev.creates || []).length,
          m = (ev.modifies || []).length;
        const delta = (c ? "+" + c : "") + (c && m ? " " : "") + (m ? "~" + m : "");
        const title = ev.title || ev.id;
        return (
          <div
            key={ev.id}
            className={"c3-tl-row" + (i === index ? " current" : i < index ? " past" : "")}
            title={title}
            onClick={() => {
              scene.tlPause();
              scene.goToEvent(i);
            }}
          >
            <span className="c3-tl-row-date">{date}</span>
            <span className="c3-tl-row-dot" style={{ background: color }}></span>
            <span className="c3-tl-row-title">{title}</span>
            <span className="c3-tl-row-delta">{delta}</span>
          </div>
        );
      })}
    </div>
  );
}
