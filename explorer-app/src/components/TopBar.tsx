import { useEffect, useState } from "react";
import type { ExplorerScene, Snapshot } from "../scene/ExplorerScene";
import type { Level } from "../data";

const LEVELS: { key: Level; label: string }[] = [
  { key: "context", label: "C1 · Context" },
  { key: "container", label: "C2 · Containers" },
  { key: "component", label: "C3 · Components" },
];

export function TopBar({
  scene,
  snap,
  project,
  live,
}: {
  scene: ExplorerScene;
  snap: Snapshot;
  project: string;
  live: { connected: boolean } | null;
}) {
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (snap.query === "") setSearch("");
  }, [snap.query]);

  return (
    <div className="c3-topbar">
      <div className="c3-pill c3-brand">
        <span className="c3-logo"></span>
        <span className="c3-brand-name">{project}</span>
        <span className="c3-brand-sub">Architecture · C4 explorer</span>
        {live && (
          <span
            className={"c3-live-dot" + (live.connected ? " on" : " off")}
            title={live.connected ? "Live — connected to c3x" : "Reconnecting…"}
          >
            {live.connected ? "LIVE" : "…"}
          </span>
        )}
      </div>
      <div className="c3-pill c3-controls">
        <div className="c3-levels">
          {LEVELS.map((l) => (
            <button
              key={l.key}
              className={snap.level === l.key ? "active" : ""}
              onClick={() => scene.setLevel(l.key)}
            >
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
          <button
            className={"c3-tl-toggle" + (snap.timeline.active ? " active" : "")}
            title="Replay the architecture timeline"
            onClick={() => scene.toggleTimeline()}
          >
            ⏱ Timeline
          </button>
        )}
      </div>
    </div>
  );
}
