import { useEffect, useState } from "react";
import type { CityScene, Snapshot } from "../scene/CityScene";
import type { MotionMode } from "../scene/TrafficSystem";
import type { Level } from "../data";
import { SKINS } from "../skin";

const MOTION: { key: MotionMode; label: string }[] = [
  { key: "active", label: "active flow" },
  { key: "all", label: "all roads" },
  { key: "none", label: "still" },
];

const LEVELS: { key: Level; label: string; title: string }[] = [
  { key: "context", label: "C1", title: "Context: system and containers" },
  { key: "container", label: "C2", title: "Containers and change-units" },
  { key: "component", label: "C3", title: "Components" },
  { key: "all", label: "All", title: "The whole base" },
];

export function TopBar({ scene, snap, project, live }: { scene: CityScene; snap: Snapshot; project: string; live: { connected: boolean } | null }) {
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (snap.query === "") setSearch("");
  }, [snap.query]);

  return (
    <div className="c3-topbar">
      <div className="c3-panel c3-brand">
        <span className="c3-logo"></span>
        <div>
          <div className="c3-brand-name">c3v · base overview</div>
          <div className="c3-brand-sub">
            {project} / {snap.visibleCount}
            {snap.visibleCount !== snap.nodeCount ? ` of ${snap.nodeCount}` : ""} nodes
          </div>
        </div>
        {live && (
          <span className={"c3-live-dot" + (live.connected ? " on" : " off")} title={live.connected ? "Live — connected to c3x" : "Reconnecting…"}>
            {live.connected ? "LIVE" : "…"}
          </span>
        )}
      </div>
      <div className="c3-panel c3-controls">
        <span className="c3-lbl">Traffic</span>
        <div className="c3-seg" id="motion">
          {MOTION.map((m) => (
            <button key={m.key} className={snap.motion === m.key ? "active" : ""} onClick={() => scene.setMotion(m.key)}>
              {m.label}
            </button>
          ))}
        </div>
        {scene.getSkin().lighting.bloom && (
          <>
            <span className="c3-lbl">Bloom</span>
            <div className="c3-seg" id="bloom">
              <button className={snap.bloom ? "active" : ""} onClick={() => scene.setBloom(true)}>
                on
              </button>
              <button className={!snap.bloom ? "active" : ""} onClick={() => scene.setBloom(false)}>
                off
              </button>
            </div>
          </>
        )}
        <span className="c3-lbl">Skin</span>
        <div className="c3-seg" id="skin">
          {SKINS.map((s) => (
            <button key={s.id} className={snap.skin === s.id ? "active" : ""} title={`${s.label} skin`} onClick={() => scene.setSkin(s.id)}>
              {s.label}
            </button>
          ))}
        </div>
        <span className="c3-lbl">Level</span>
        <div className="c3-seg c3-levels">
          {LEVELS.map((l) => (
            <button key={l.key} title={l.title} className={snap.level === l.key ? "active" : ""} onClick={() => scene.setLevel(l.key)}>
              {l.label}
            </button>
          ))}
        </div>
        <div className="c3-search-wrap">
          <span className="c3-search-icon">⌕</span>
          <input
            id="c3-search"
            type="text"
            placeholder="Search…"
            autoComplete="off"
            spellCheck={false}
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              scene.setQuery(e.target.value);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") scene.selectFirstMatch();
            }}
          />
        </div>
        {snap.timeline.available && (
          <button className={"c3-tl-toggle" + (snap.timeline.active ? " active" : "")} title="Replay the architecture timeline" onClick={() => scene.toggleTimeline()}>
            ⏱ Timeline
          </button>
        )}
        <button id="reset" onClick={() => scene.resetView()}>
          Reset view
        </button>
      </div>
    </div>
  );
}
