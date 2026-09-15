import { useEffect, useRef, useState } from "react";
import { CityScene } from "./scene/CityScene";
import { installExplorerAPI } from "./api/explorerAPI";
import { useExplorerSnapshot } from "./state/explorerState";
import { startLiveClient, type ActionEvent } from "./live/liveClient";
import { TopBar } from "./components/TopBar";
import { FilterPanel } from "./components/FilterPanel";
import { Inspector } from "./components/Inspector";
import { Tooltip } from "./components/Tooltip";
import { TimelineBar } from "./components/TimelineBar";
import { KaraokeList } from "./components/KaraokeList";
import { LoadingOverlay } from "./components/LoadingOverlay";
import { LiveFeed } from "./components/LiveFeed";
import { LiveBanner } from "./components/LiveBanner";
import type { C3Payload } from "./data";
import { applyChrome, initialSkinId, resolveSkin } from "./skin";

export function App({ data }: { data: C3Payload }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [scene, setScene] = useState<CityScene | null>(null);
  const snap = useExplorerSnapshot(scene);

  const isLive = !!window.C3_LIVE;
  const [feed, setFeed] = useState<ActionEvent[]>([]);
  const [connected, setConnected] = useState(true);
  const [issues, setIssues] = useState<string[]>([]);

  useEffect(() => {
    if (!canvasRef.current) return;
    const skin = resolveSkin(initialSkinId());
    applyChrome(skin);
    const sc = new CityScene(canvasRef.current, data, skin);
    installExplorerAPI(sc);
    setScene(sc);
    return () => {
      sc.dispose();
      setScene(null);
    };
  }, [data]);

  useEffect(() => {
    if (!scene || !isLive) return;
    return startLiveClient(scene, {
      onAction: (e) => setFeed((f) => [e, ...f].slice(0, 50)),
      onInvalid: setIssues,
      onStatus: setConnected,
    });
  }, [scene, isLive]);

  // Live frames replace the scene's payload; chrome renders from the scene's copy so it never goes stale.
  const liveData = scene ? scene.getData() : data;

  return (
    <>
      <canvas id="c3-canvas" ref={canvasRef}></canvas>
      {scene && (
        <>
          <TopBar scene={scene} snap={snap} project={liveData.project || "C3"} live={isLive ? { connected } : null} />
          <div className="c3-left">
            {snap.timeline.active ? <KaraokeList scene={scene} snap={snap} data={liveData} /> : <FilterPanel scene={scene} snap={snap} data={liveData} />}
            {isLive && <LiveFeed items={feed} lastUpdate={snap.lastUpdate} />}
          </div>
          <Inspector scene={scene} snap={snap} data={liveData} />
          <Tooltip snap={snap} />
          <TimelineBar scene={scene} snap={snap} data={liveData} />
          {isLive && <LiveBanner issues={issues} />}
          <div className="c3-panel c3-hints">
            Drag to orbit (constrained) · right-drag to pan · scroll to zoom · <kbd>WASD</kbd> move · <kbd>dbl-click</kbd> travel · <kbd>Esc</kbd> clear
          </div>
        </>
      )}
      {!snap.ready && <LoadingOverlay />}
    </>
  );
}
