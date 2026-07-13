import { LIFECYCLE_COLORS } from "../scene/constants";
import type { C3Payload } from "../data";
import type { ExplorerScene, Snapshot } from "../scene/ExplorerScene";

const SPEEDS = [0.5, 1, 2, 4, 8];

export function TimelineBar({ scene, snap, data }: { scene: ExplorerScene; snap: Snapshot; data: C3Payload }) {
  if (!snap.timeline.active) return null;
  const { index, playing, speed, eventCount } = snap.timeline;
  const ev = (data.events || [])[index];

  return (
    <div className="c3-timeline">
      <button
        className="c3-tl-play"
        aria-label={playing ? "Pause" : "Play"}
        onClick={() => (playing ? scene.tlPause() : scene.tlPlay())}
      >
        {playing ? "⏸" : "▶"}
      </button>
      <input
        type="range"
        id="c3-tl-scrub"
        min={0}
        max={Math.max(eventCount - 1, 0)}
        step={1}
        value={index}
        aria-label="Timeline position"
        onInput={(e) => {
          scene.tlPause();
          scene.goToEvent(parseInt((e.target as HTMLInputElement).value, 10) || 0);
        }}
      />
      <select
        id="c3-tl-speed"
        aria-label="Playback speed"
        value={speed}
        onChange={(e) => scene.setTlSpeed(parseFloat(e.target.value) || 1)}
      >
        {SPEEDS.map((s) => (
          <option key={s} value={s}>
            {s}×
          </option>
        ))}
      </select>
      <div className="c3-tl-card">
        {ev && (
          <>
            <div className="c3-tl-date">
              {ev.date || ""} · {index + 1}/{eventCount}
            </div>
            <div className="c3-tl-title">{ev.title || ev.id}</div>
            <div className="c3-tl-delta">
              <span className="c3-tl-status" style={{ background: LIFECYCLE_COLORS[ev.status] || "#6b7280" }}>
                {ev.status || ""}
              </span>{" "}
              +{(ev.creates || []).length} created · ~{(ev.modifies || []).length} touched
            </div>
          </>
        )}
      </div>
    </div>
  );
}
