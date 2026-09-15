import type { C3Payload } from "./data";
import type { ExplorerAPI } from "./api/explorerAPI";

declare global {
  interface Window {
    C3_DATA?: C3Payload;
    C3_LIVE?: boolean;
    /** Skin id to start with; wins over `?skin=` and the persisted choice. */
    C3_SKIN?: string;
    C3_EXPLORER?: ExplorerAPI;
  }
}

export {};
