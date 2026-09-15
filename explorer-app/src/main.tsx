import { createRoot } from "react-dom/client";
import { App } from "./App";
import { LoadingOverlay } from "./components/LoadingOverlay";
import type { C3Payload } from "./data";
import "./styles.css";

const root = createRoot(document.getElementById("root")!);

function mount(data: C3Payload | undefined): void {
  if (!data) {
    root.render(<LoadingOverlay message="No architecture data. Set window.C3_DATA before loading this script." />);
    return;
  }
  root.render(<App data={data} />);
}

const data = window.C3_DATA;
if (!data && import.meta.env.DEV) {
  // Local `npm run dev` without a Go payload: load the hand-planned fixture. Dead code in the shipped bundle.
  import("./dev/fixture").then((m) => {
    window.C3_DATA = m.FIXTURE;
    mount(m.FIXTURE);
  });
} else {
  mount(data);
}
