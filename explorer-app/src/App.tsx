import { useEffect, useRef, useState } from "react";
import { ExplorerScene } from "./scene/ExplorerScene";
import { installExplorerAPI } from "./api/explorerAPI";
import { useExplorerSnapshot } from "./state/explorerState";
import { TopBar } from "./components/TopBar";
import { ContainerPicker } from "./components/ContainerPicker";
import { Legend } from "./components/Legend";
import { DetailPanel } from "./components/DetailPanel";
import { Tooltip } from "./components/Tooltip";
import { Breadcrumb } from "./components/Breadcrumb";
import { TimelineBar } from "./components/TimelineBar";
import { KaraokeList } from "./components/KaraokeList";
import { LoadingOverlay } from "./components/LoadingOverlay";
import type { C3Payload } from "./data";

export function App({ data }: { data: C3Payload }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [scene, setScene] = useState<ExplorerScene | null>(null);
  const snap = useExplorerSnapshot(scene);

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

  return (
    <>
      <canvas id="c3-canvas" ref={canvasRef}></canvas>
      {scene && (
        <>
          <TopBar scene={scene} snap={snap} project={data.project || "C3"} />
          <ContainerPicker scene={scene} snap={snap} data={data} />
          <Legend scene={scene} snap={snap} />
          <DetailPanel scene={scene} snap={snap} />
          <Tooltip snap={snap} />
          <Breadcrumb snap={snap} data={data} />
          <TimelineBar scene={scene} snap={snap} data={data} />
          <KaraokeList scene={scene} snap={snap} data={data} />
          <div className="c3-hints">
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
