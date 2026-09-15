import { useMemo, useState, type ReactElement } from "react";
import type { CityScene, EdgeRef, NodeSelection, RouteSelection, Snapshot } from "../scene/CityScene";
import type { SceneTreeNode } from "../scene/sceneTree";
import { ARCH_LABEL } from "../scene/constants";
import { type Skin, boundaryKindColor, kindStyle } from "../skin";
import type { C3Payload } from "../data";

type Tab = "detail" | "scene" | "legend";

export function Inspector({ scene, snap, data }: { scene: CityScene; snap: Snapshot; data: C3Payload }) {
  const [tab, setTab] = useState<Tab>("detail");
  return (
    <div className="c3-side">
      <div className="c3-panel c3-card c3-inspector">
        <div className="c3-tabs">
          {(["detail", "scene", "legend"] as Tab[]).map((t) => (
            <button key={t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>
              {t === "detail" ? "Inspector" : t === "scene" ? "Scene" : "Legend"}
            </button>
          ))}
        </div>
        {tab === "detail" && <Detail scene={scene} sel={snap.selection} />}
        {tab === "scene" && <SceneTree scene={scene} snap={snap} />}
        {tab === "legend" && <Legend data={data} skin={scene.getSkin()} />}
      </div>
    </div>
  );
}

function Row({ e, id, onClick }: { e: EdgeRef; id: string; onClick: (id: string) => void }) {
  return (
    <div className="c3-row c3-link-row" onClick={() => onClick(id)}>
      <span className="c3-dot" style={{ background: e.color }}></span>
      <span className="c3-row-label">{e.title}</span>
      <span className="c3-sub-inline">
        · {e.kindLabel}
        {e.label ? " · " + e.label : ""}
      </span>
    </div>
  );
}

function Detail({ scene, sel }: { scene: CityScene; sel: NodeSelection | RouteSelection | null }) {
  if (!sel) return <div className="c3-empty">Click a building. Double-click to travel to it.</div>;
  if (sel.kind === "route") return <RouteDetail scene={scene} r={sel} />;
  const n = sel;
  const skin = scene.getSkin();
  const go = (id: string): void => {
    scene.focus(id);
  };
  return (
    <div className="c3-detail">
      <div className="c3-title">{n.title}</div>
      <div className="c3-sub">
        {n.id} · {n.type}
        {n.tech ? " · " + n.tech : ""} · {n.archetypeLabel}
        <br />
        <span className="c3-pill">
          <span className="c3-dot" style={{ background: n.statusColor }}></span>
          {n.statusText}
        </span>
        {n.staged && n.transition && (
          <span className="c3-pill c3-pill-staged">
            {n.transition.from} → {n.transition.to}
          </span>
        )}
      </div>
      <dl>
        {n.goal && (
          <>
            <dt>Goal</dt>
            <dd>{n.goal}</dd>
          </>
        )}
        {n.district && (
          <>
            <dt>District</dt>
            <dd>{n.districtTitle.toLowerCase()}</dd>
          </>
        )}
        {n.parent && (
          <>
            <dt>Parent</dt>
            <dd>
              <button className="c3-link" onClick={() => go(n.parent as string)}>
                {n.parentTitle} <code>{n.parent}</code>
              </button>
            </dd>
          </>
        )}
        {n.category && (
          <>
            <dt>Category</dt>
            <dd>{n.category}</dd>
          </>
        )}
        {n.boundaryKind && (
          <>
            <dt>Kind</dt>
            <dd>
              <span className="c3-dot c3-dot-inline" style={{ background: boundaryKindColor(skin, n.boundaryKind) }}></span>
              {n.boundaryKind}
            </dd>
          </>
        )}
        {n.legacyBoundary && (
          <>
            <dt>Boundary</dt>
            <dd>{n.legacyBoundary}</dd>
          </>
        )}
        {n.zones.length > 0 && (
          <>
            <dt>Zones</dt>
            <dd>
              {n.zones.map((z) => (
                <div key={z.id}>
                  <button className="c3-link" onClick={() => go(z.id)}>
                    <span className="c3-dot c3-dot-inline" style={{ background: boundaryKindColor(skin, z.kind) }}></span>
                    {z.kind} · {z.title}
                  </button>
                </div>
              ))}
            </dd>
          </>
        )}
        {n.members && n.members.length > 0 && (
          <>
            <dt>Members</dt>
            <dd>
              {n.members.map((m) => (
                <div key={m}>
                  <button className="c3-link" onClick={() => go(m)}>
                    {m}
                  </button>
                </div>
              ))}
            </dd>
          </>
        )}
        {n.flowSteps && n.flowSteps.length > 0 && (
          <>
            <dt>Steps</dt>
            <dd>
              {n.flowSteps.map((s) => (
                <div key={s.seq} className="c3-step">
                  <code>{s.seq}</code>{" "}
                  <button className="c3-link" onClick={() => go(s.from)}>
                    {s.from}
                  </button>{" "}
                  →{" "}
                  <button className="c3-link" onClick={() => go(s.to)}>
                    {s.to}
                  </button>
                  {s.action ? <span className="c3-sub-inline"> · {s.action}</span> : null}
                </div>
              ))}
            </dd>
          </>
        )}
        {n.governs.length > 0 && (
          <>
            <dt>Governs</dt>
            <dd>
              {n.governs.map((g) => (
                <div key={g.edgeId}>
                  <button className="c3-link" onClick={() => go(g.id)}>
                    {g.title}
                  </button>
                </div>
              ))}
            </dd>
          </>
        )}
        {n.code && (
          <>
            <dt>Code</dt>
            <dd>
              <code>{n.code.globs.join("\n")}</code>
              <br />
              {n.code.files} files · {n.code.loc.toLocaleString()} loc
            </dd>
          </>
        )}
        {n.eval && (
          <>
            <dt>Eval</dt>
            <dd>{n.eval.verdict}</dd>
          </>
        )}
        {n.stagedBy && n.stagedBy.length > 0 && (
          <>
            <dt>Staged by</dt>
            <dd>
              {n.stagedBy.map((s) => (
                <div key={s}>
                  <code>{s}</code>
                </div>
              ))}
            </dd>
          </>
        )}
        <dt>Lifecycle</dt>
        <dd>{n.lifecycle}</dd>
      </dl>
      {n.inbound.length > 0 && (
        <>
          <div className="c3-q">Inbound ({n.inbound.length})</div>
          {n.inbound.map((e) => (
            <Row key={e.edgeId} e={e} id={e.id} onClick={go} />
          ))}
        </>
      )}
      {n.outbound.length > 0 && (
        <>
          <div className="c3-q">Outbound ({n.outbound.length})</div>
          {n.outbound.map((e) => (
            <Row key={e.edgeId} e={e} id={e.id} onClick={go} />
          ))}
        </>
      )}
      {n.contains.length > 0 && (
        <>
          <div className="c3-q">Contains ({n.contains.length})</div>
          {n.contains.map((e) => (
            <Row key={e.edgeId} e={e} id={e.id} onClick={go} />
          ))}
        </>
      )}
    </div>
  );
}

function RouteDetail({ scene, r }: { scene: CityScene; r: RouteSelection }) {
  return (
    <div className="c3-detail">
      <div className="c3-title">
        {r.kindLabel}
        {r.label ? " · " + r.label : ""}
      </div>
      <div className="c3-sub">
        {r.id}
        <br />
        <span className="c3-pill">
          <span className="c3-dot" style={{ background: r.color }}></span>
          {r.active ? "ACTIVE" : "IDLE"}
        </span>
      </div>
      <dl>
        <dt>From</dt>
        <dd>
          <button className="c3-link" onClick={() => scene.focus(r.fromId)}>
            {r.fromTitle} <code>{r.fromId}</code>
          </button>
        </dd>
        <dt>To</dt>
        <dd>
          <button className="c3-link" onClick={() => scene.focus(r.toId)}>
            {r.toTitle} <code>{r.toId}</code>
          </button>
        </dd>
        {r.flow && (
          <>
            <dt>Flow</dt>
            <dd>
              <button className="c3-link" onClick={() => scene.focus(r.flow as string)}>
                {r.flow}
              </button>
              {r.seq !== undefined ? ` · step ${r.seq}` : ""}
            </dd>
          </>
        )}
        {r.segments.length > 0 && (
          <>
            <dt>Roads</dt>
            <dd>
              <code>{r.segments.join("\n")}</code>
            </dd>
          </>
        )}
        {r.sharedWith && (
          <>
            <dt>Shares</dt>
            <dd>
              <code>{r.sharedWith}</code>
            </dd>
          </>
        )}
      </dl>
    </div>
  );
}

function SceneTree({ scene, snap }: { scene: CityScene; snap: Snapshot }) {
  const tree = useMemo(() => scene.inspectorTree(), [scene, snap.lastUpdate, snap.visibleCount]);
  const [open, setOpen] = useState<Set<string>>(() => new Set([tree.name]));
  const toggle = (k: string): void => {
    const next = new Set(open);
    if (next.has(k)) next.delete(k);
    else next.add(k);
    setOpen(next);
  };
  const render = (t: SceneTreeNode, path: string, depth: number): ReactElement => {
    const k = path + "/" + t.name;
    const isOpen = open.has(k) || depth === 0;
    const selectable = t.c3?.type && !["world", "environment", "district", "nodes", "roads", "props", "route"].includes(t.c3.type);
    return (
      <div key={k} className="c3-tree-node" style={{ paddingLeft: depth ? 10 : 0 }}>
        <div className={"c3-tree-row" + (t.visible ? "" : " hidden")}>
          <span className={"c3-tree-caret" + (t.children.length ? "" : " leaf")} onClick={() => t.children.length && toggle(k)}>
            {t.children.length ? (isOpen ? "▾" : "▸") : "·"}
          </span>
          <span
            className={"c3-tree-name" + (selectable ? " c3-link" : "")}
            onClick={() => {
              if (selectable && t.c3) scene.focus(t.c3.id);
            }}
          >
            {t.name}
          </span>
          <span className="c3-tree-meta">
            {t.c3?.archetype ? ARCH_LABEL[t.c3.archetype] ?? t.c3.archetype : t.c3?.kind ?? t.type.toLowerCase()}
            {t.parts ? ` · ${t.parts} parts` : ""}
          </span>
        </div>
        {isOpen && t.children.map((c) => render(c, k, depth + 1))}
      </div>
    );
  };
  return (
    <div className="c3-tree">
      <div className="c3-empty" style={{ marginBottom: 6 }}>
        World group as exported by <code>C3_EXPLORER.exportSceneJSON()</code>.
      </div>
      {render(tree, "", 0)}
    </div>
  );
}

const ARCH_ROWS: { key: string; h: number }[] = [
  { key: "headquarters", h: 18 },
  { key: "gatehouse", h: 7 },
  { key: "comms", h: 16 },
  { key: "command", h: 12 },
  { key: "plant", h: 10 },
  { key: "bunker", h: 7 },
  { key: "transmit", h: 8 },
  { key: "outpost", h: 5 },
  { key: "record", h: 3 },
  { key: "zone", h: 2 },
  { key: "route", h: 4 },
];

function Legend({ data, skin }: { data: C3Payload; skin: Skin }) {
  const kinds = new Set(data.edges.map((e) => e.kind).filter((k) => k !== "contains"));
  const statuses = new Set(data.nodes.map((n) => n.statusKey));
  const archetypes = new Set(data.nodes.map((n) => n.archetype));
  return (
    <div className="c3-legend">
      <div className="c3-q">Buildings (archetype is derived)</div>
      {ARCH_ROWS.filter((a) => archetypes.has(a.key as never)).map((a) => (
        <div key={a.key} className="c3-row">
          <span className="c3-sil" style={{ height: a.h }}></span>
          {ARCH_LABEL[a.key]}
        </div>
      ))}
      <div className="c3-q">Roads</div>
      {[...kinds].map((k) => (
        <div key={k} className="c3-row">
          <span className="c3-bar" style={{ background: kindStyle(skin, k).color }}></span>
          {kindStyle(skin, k).label}
        </div>
      ))}
      <div className="c3-row">
        <span className="c3-bar" style={{ background: skin.tokens.palette.line }}></span>
        primary trunk trench · street conduit · dock terminal
      </div>
      <div className="c3-q">Status light</div>
      {Object.entries(skin.tokens.status)
        .filter(([k]) => statuses.has(k as never))
        .map(([k, s]) => (
          <div key={k} className="c3-row">
            <span className="c3-dot" style={{ background: s.color }}></span>
            {s.text} — {STATUS_HINT[k] ?? k}
          </div>
        ))}
      <div className="c3-q">Traffic</div>
      <div className="c3-row">Packets travel only on active routes (flows, staged sources) unless "all roads" is on. Reduced motion stops all of it.</div>
    </div>
  );
}

const STATUS_HINT: Record<string, string> = {
  stable: "frozen, eval holds; still",
  changing: "staged; core breathes, traffic on its routes",
  drift: "code contradicts the fact; slow red lamp",
  review: "needs judgement; amber beacon",
  unchecked: "no eval spec",
  governance: "ref or rule",
  open: "change-unit open",
  accepted: "change-unit accepted",
  done: "change-unit done",
  superseded: "change-unit superseded",
};
