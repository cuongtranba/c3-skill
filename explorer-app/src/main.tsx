import { createRoot } from "react-dom/client";
import { App } from "./App";
import { LoadingOverlay } from "./components/LoadingOverlay";
import "./styles.css";

const data = window.C3_DATA;
const root = createRoot(document.getElementById("root")!);

if (!data) {
  root.render(<LoadingOverlay message="No architecture data. Set window.C3_DATA before loading this script." />);
} else {
  root.render(<App data={data} />);
}
