import * as THREE from "three";
import type { LabelStyle } from "../skin/types";

function context2d(w: number, h: number): { cnv: HTMLCanvasElement; c: CanvasRenderingContext2D } | null {
  const cnv = document.createElement("canvas");
  cnv.width = w;
  cnv.height = h;
  const c = cnv.getContext("2d");
  return c ? { cnv, c } : null;
}

export interface LabelStatus {
  color: string;
}

/** Tag: `name / ID / ● STATUS`, sharp corners, thin border, colours and fonts from the skin.
 * Text never scales with importance; LOD decides which of the two variants is visible. */
export function labelSprite(style: LabelStyle, lines: string[], status?: LabelStatus): THREE.Sprite {
  const dpr = 2;
  const fs = [style.sizes.title, style.sizes.meta];
  const pad = style.sizes.pad;
  const { sans, mono } = style.fonts;
  const measure = context2d(8, 8);
  let w = 120;
  if (measure) {
    measure.c.font = `650 ${fs[0]}px ${sans}`;
    w = measure.c.measureText(lines[0]).width;
    measure.c.font = `500 ${fs[1]}px ${mono}`;
    for (const l of lines.slice(1)) w = Math.max(w, measure.c.measureText(l).width + (status ? 26 : 0));
  }
  const W = Math.ceil(w + pad * 2);
  const H = Math.ceil(fs[0] + (lines.length - 1) * (fs[1] + 6) + pad * 1.5);
  const target = context2d(W * dpr, H * dpr);
  let map: THREE.CanvasTexture | null = null;
  if (target) {
    const { cnv, c } = target;
    c.scale(dpr, dpr);
    c.fillStyle = style.tag.fill;
    c.fillRect(0, 0, W, H);
    c.strokeStyle = style.tag.stroke;
    c.lineWidth = 1;
    c.strokeRect(0.5, 0.5, W - 1, H - 1);
    c.fillStyle = style.tag.title;
    c.font = `650 ${fs[0]}px ${sans}`;
    c.textBaseline = "top";
    c.fillText(lines[0], pad, pad * 0.75);
    let y = pad * 0.75 + fs[0] + 6;
    for (let i = 1; i < lines.length; i++) {
      const last = i === lines.length - 1 && status;
      if (last) {
        c.fillStyle = status.color;
        c.beginPath();
        c.arc(pad + 7, y + fs[1] / 2 + 1, 6, 0, Math.PI * 2);
        c.fill();
      }
      c.fillStyle = last ? style.tag.status : style.tag.meta;
      c.font = `500 ${fs[1]}px ${mono}`;
      c.fillText(lines[i], pad + (last ? 22 : 0), y);
      y += fs[1] + 6;
    }
    map = new THREE.CanvasTexture(cnv);
    map.colorSpace = THREE.SRGBColorSpace;
    map.minFilter = THREE.LinearFilter;
  }
  const s = new THREE.Sprite(new THREE.SpriteMaterial({ map, transparent: true, depthWrite: false, depthTest: false }));
  s.scale.set(W / style.sizes.scale, H / style.sizes.scale, 1);
  s.userData.baseScale = s.scale.clone();
  s.userData.noExport = true;
  s.userData.isLabel = true;
  return s;
}

/** Stencil text painted flat on the ground (district titles, zone names). */
export function groundText(style: LabelStyle, text: string, w: number, color = style.stencil): THREE.Mesh {
  const target = context2d(1024, 128);
  let map: THREE.CanvasTexture | null = null;
  if (target) {
    const { cnv, c } = target;
    c.font = `700 60px ${style.fonts.sans}`;
    c.fillStyle = color;
    c.textAlign = "left";
    c.textBaseline = "middle";
    (c as CanvasRenderingContext2D & { letterSpacing?: string }).letterSpacing = "8px";
    c.fillText(text, 0, 64);
    map = new THREE.CanvasTexture(cnv);
    map.colorSpace = THREE.SRGBColorSpace;
    map.anisotropy = 8;
  }
  const m = new THREE.Mesh(
    new THREE.PlaneGeometry(w, w / 8),
    new THREE.MeshBasicMaterial({ map, transparent: true, depthWrite: false, toneMapped: false, opacity: map ? 1 : 0 }),
  );
  m.rotation.x = -Math.PI / 2;
  m.userData.noExport = true;
  return m;
}
