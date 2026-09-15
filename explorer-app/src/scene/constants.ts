/* Skin-independent constants. Everything visual (palette, kinds, status, boundary and
 * district colours, materials) lives in the active skin — see src/skin/. */

export const ARCH_LABEL: Record<string, string> = {
  headquarters: "headquarters",
  gatehouse: "sector gatehouse",
  comms: "comms tower",
  command: "command centre",
  plant: "compute plant",
  bunker: "storage bunker",
  transmit: "transmit facility",
  outpost: "perimeter outpost",
  record: "record marker",
  zone: "perimeter zone",
  route: "flow signpost",
};

/** Heights the `--export scene.json` contract uses; the renderer's roofs match them closely. */
export const ARCH_HEIGHT: Record<string, number> = {
  headquarters: 20,
  command: 19,
  comms: 18,
  bunker: 7,
  plant: 9,
  transmit: 8,
  gatehouse: 6,
  outpost: 3,
  record: 1.5,
  zone: 0.1,
  route: 0.5,
};

export const LOD = { far: 260, near: 110 };

export function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h;
}

export function easeInOutQuad(t: number): number {
  return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
}
