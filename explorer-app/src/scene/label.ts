import * as THREE from "three";

export interface LabelOpts {
  big?: boolean;
  ring?: boolean;
  color?: string;
}

export function makeLabel(text: string, opts: LabelOpts = {}): THREE.Sprite {
  const dpr = 2;
  const fs = opts.ring ? 20 : opts.big ? 28 : 24;
  const pad = opts.ring ? 0 : 12;
  const cnv = document.createElement("canvas");
  const ctx = cnv.getContext("2d")!;
  ctx.font = `600 ${fs}px system-ui,-apple-system,sans-serif`;
  const tw = ctx.measureText(text).width;
  cnv.width = (tw + pad * 2) * dpr;
  cnv.height = (fs + pad * 1.2) * dpr;
  const c2 = cnv.getContext("2d")!;
  c2.scale(dpr, dpr);
  if (!opts.ring) {
    const rw = tw + pad * 2,
      rh = fs + pad * 1.2,
      r = 7;
    c2.fillStyle = "rgba(255,255,255,0.93)";
    c2.strokeStyle = "#e5e7e9";
    c2.lineWidth = 1;
    c2.beginPath();
    c2.moveTo(r, 0);
    c2.arcTo(rw, 0, rw, rh, r);
    c2.arcTo(rw, rh, 0, rh, r);
    c2.arcTo(0, rh, 0, 0, r);
    c2.arcTo(0, 0, rw, 0, r);
    c2.closePath();
    c2.fill();
    c2.stroke();
  }
  c2.font = `600 ${fs}px system-ui,-apple-system,sans-serif`;
  c2.fillStyle = opts.ring ? opts.color || "#6b6f75" : "#1c1f24";
  c2.textBaseline = "middle";
  if (opts.ring) c2.globalAlpha = 0.65;
  c2.fillText(text, pad, (fs + pad * 1.2) / 2);
  const tex = new THREE.CanvasTexture(cnv);
  tex.minFilter = THREE.LinearFilter;
  tex.needsUpdate = true;
  const spr = new THREE.Sprite(
    new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false, depthTest: true }),
  );
  const wscale = (cnv.width / dpr / 24) * (opts.big ? 1.15 : 1);
  spr.scale.set(wscale, cnv.height / dpr / 24, 1);
  spr.userData.isLabel = true;
  return spr;
}
