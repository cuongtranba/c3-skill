import { useEffect, useRef, useState } from "react";
import { ExplorerScene } from "./scene/ExplorerScene";
import { installExplorerAPI } from "./api/explorerAPI";
import { useExplorerSnapshot } from "./state/explorerState";
import { startLiveClient, type ActionEvent } from "./live/liveClient";
import { TopBar } from "./components/TopBar";
import { ContainerPicker } from "./components/ContainerPicker";
import { Legend } from "./components/Legend";
import { DetailPanel } from "./components/DetailPanel";
import { Tooltip } from "./components/Tooltip";
import { Breadcrumb } from "./components/Breadcrumb";
import { TimelineBar } from "./components/TimelineBar";
import { KaraokeList } from "./components/KaraokeList";
import { LoadingOverlay } from "./components/LoadingOverlay";
import { LiveFeed } from "./components/LiveFeed";
import { LiveBanner } from "./components/LiveBanner";
import type { C3Payload } from "./data";

export function App({ data }: { data: C3Payload }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [scene, setScene] = useState<ExplorerScene | null>(null);
  const snap = useExplorerSnapshot(scene);

  const isLive = !!window.C3_LIVE;
  const [feed, setFeed] = useState<ActionEvent[]>([]);
  const [connected, setConnected] = useState(true);
  const [issues, setIssues] = useState<string[]>([]);

  useEffect(() => {
    if (!canvasRef.current) return;
    const sc = new ExplorerScene(canvasRef.current, data);
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

  // Live updates replace the scene's payload; render from the scene's current
  // copy (snapshot emission re-renders) so chrome never shows stale data.
  const liveData = scene ? scene.getData() : data;

  return (
    <>
      <canvas id="c3-canvas" ref={canvasRef}></canvas>
      {scene && (
        <>
          <TopBar scene={scene} snap={snap} project={liveData.project || "C3"} live={isLive ? { connected } : null} />
          <ContainerPicker scene={scene} snap={snap} data={liveData} />
          <Legend scene={scene} snap={snap} data={liveData}>
            {isLive && <LiveFeed items={feed} lastUpdate={snap.lastUpdate} />}
          </Legend>
          <DetailPanel scene={scene} snap={snap} />
          <Tooltip snap={snap} />
          <Breadcrumb snap={snap} data={liveData} />
          <TimelineBar scene={scene} snap={snap} data={liveData} />
          <KaraokeList scene={scene} snap={snap} data={liveData} />
          {isLive && <LiveBanner issues={issues} />}
          <div className="c3-hints">
            <span>
              <b>WASD/↑↓←→</b> move
            </span>
            ·
            <span>
              <b>Drag</b> orbit
            </span>
            ·
            <span>
              <b>Scroll</b> zoom
            </span>
            ·
            <span>
              <b>Click</b> detail
            </span>
            ·
            <span>
              <b>Double-click</b> drill
            </span>
          </div>
        </>
      )}
      {!snap.ready && <LoadingOverlay />}
    </>
  );
}
