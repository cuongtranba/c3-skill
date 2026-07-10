/* C3 Architecture Explorer — vanilla JS, Three.js r128, no imports
 * Reads window.C3_DATA, mounts on #c3-canvas, exposes window.C3_EXPLORER.
 */
(function () {
  'use strict';

  /* ─── constants ─────────────────────────────────────────────────── */
  const RING_DEFS = [
    { key: 'governance', label: 'Governance',  tint: '#5b4a8a', r: 58 },
    { key: 'security',   label: 'Security',    tint: '#2f7a6f', r: 50 },
    { key: 'infra',      label: 'Infra',       tint: '#5b6a72', r: 42 },
    { key: 'service',    label: 'Service',     tint: '#3f7fc4', r: 32 },
    { key: 'platform',   label: 'Platform',    tint: '#2a9184', r: 19 },
    { key: 'core',       label: 'Core',        tint: '#2fa89a', r: 9  },
  ];

  const LIFECYCLE_COLORS = {
    frozen:      '#2fa89a',
    staged:      '#d98a2b',
    open:        '#3f7fc4',
    accepted:    '#2f7a6f',
    done:        '#6b7280',
    superseded:  '#9a3a31',
  };

  const EDGE_COLORS = {
    contains: '#7c8794',
    uses:     '#2fa89a',
    affects:  '#d98a2b',
  };

  /* ─── state ─────────────────────────────────────────────────────── */
  const st = {
    level: 'container',          // context | container | component
    focusContainer: null,        // id of selected container at component level
    selected: null,              // { id, lifecycle, node }
    query: '',
    dimmedLifecycles: new Set(), // lifecycle keys toggled off in legend
    _t: 0,
    _cam: { flying: false, t: 0, fromPos: null, toPos: null, fromTgt: null, toTgt: null },
  };

  /* ─── timeline state ────────────────────────────────────────────── */
  const tl = { active: false, index: 0, playing: false, timer: null };
  // remembered state for restore on exit
  let tlSavedLevel = 'container';
  let tlSavedFocus = null;
  // null when mode is off; Set of node ids when mode is on
  let tlVisibleSet = null;

  /* ─── three.js handles ───────────────────────────────────────────── */
  let renderer, scene, camera, controls, raycaster, pointer, worldGroup;
  let nodeMeshes = [];   // THREE.Mesh with userData.node
  let edgeMeshes = [];   // invisible fat pick tubes with userData.rec
  let edgeRecs = [];     // { e, curve, tube, arrowMeshes, particles, crossed, kind }
  let nodeById = {};     // id → node (with _x,_z,_y,_hgt injected)
  let hover = null;
  let hoverEdge = null;
  let _raf = null;
  let _booted = false;

  /* ─── exposed API shell (filled after init) ──────────────────────── */
  window.C3_EXPLORER = {
    ready: false,
    renderedNodeIds:  () => nodeMeshes.map(m => m.userData.node.id),
    allDataNodeIds:   () => (window.C3_DATA && window.C3_DATA.nodes ? window.C3_DATA.nodes.map(n => n.id) : []),
    renderedEdgeCount: () => edgeRecs.length,
    dataEdgeCount:    () => (window.C3_DATA && window.C3_DATA.edges ? window.C3_DATA.edges.length : 0),
    nodesWithoutStatus: () => nodeMeshes.filter(m => !m.userData.hasStatusGlyph).map(m => m.userData.node.id),
    selectNodeById: (id) => {
      const mesh = nodeMeshes.find(m => m.userData.node.id === id);
      if (!mesh) return false;
      selectNode(mesh.userData.node);
      return true;
    },
    setLevel: (lvl) => setLevel(lvl),
    currentSelection: () => st.selected ? { id: st.selected.id, lifecycle: st.selected.lifecycle } : null,
    timeline: {
      active:       () => tl.active,
      eventCount:   () => tlEvents().length,
      index:        () => tl.index,
      goTo: (i) => {
        const events = tlEvents();
        if (!events.length) return false;
        const idx = Math.max(0, Math.min(i, events.length - 1));
        if (!tl.active) toggleTimeline(true);
        goToEvent(idx);
        return true;
      },
      play:  () => { tlPlay(); },
      pause: () => { tlPause(); },
      visibleNodeIds: () => nodeMeshes.map(m => m.userData.node.id),
      toggle: (on) => toggleTimeline(on),
    },
  };

  /* ─── helpers ───────────────────────────────────────────────────── */
  function q(id) { return document.getElementById(id); }

  function hash(s) {
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
    return h;
  }

  function easeInOutQuad(t) {
    return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
  }

  function ringByKey(k) {
    return RING_DEFS.find(r => r.key === k) || RING_DEFS[2]; // default infra
  }

  /* ─── data filtering per level ──────────────────────────────────── */
  function getVisibleGraph() {
    const data = window.C3_DATA;
    if (!data) return { nodes: [], edges: [] };

    const allNodes = data.nodes || [];
    const allEdges = data.edges || [];

    let visNodes, visEdges;

    // Timeline mode: force full-graph branch, then filter to visible set
    if (tl.active) {
      visNodes = allNodes.slice();
    } else if (st.level === 'context') {
      // system + containers (direct children of system nodes)
      visNodes = allNodes.filter(n => n.level === 'context' || n.type === 'system' || n.type === 'container');
      // also include direct container children of any system
      const systemIds = new Set(visNodes.filter(n => n.type === 'system').map(n => n.id));
      const ctxIds = new Set(visNodes.map(n => n.id));
      allNodes.filter(n => n.type === 'container' && n.parent && systemIds.has(n.parent)).forEach(n => ctxIds.add(n.id));
      visNodes = allNodes.filter(n => ctxIds.has(n.id));
    } else if (st.level === 'container') {
      // C2 = the full platform: system + containers + every component, plus the
      // refs/rules they depend on and any change-unit (adr) nodes. Showing the whole
      // graph here is what keeps components and their dependency edges from ever
      // being lost at the default level.
      visNodes = allNodes.slice();
    } else {
      // component level: components + refs/rules they use
      let candidates = allNodes.filter(n => n.type === 'component' || n.type === 'ref' || n.type === 'rule' || n.level === 'component');
      if (st.focusContainer) {
        // filter to this container + what components in it use
        const directIds = new Set(candidates.filter(n => n.parent === st.focusContainer).map(n => n.id));
        // follow uses edges outward one hop
        allEdges.filter(e => e.kind === 'uses' && directIds.has(e.from)).forEach(e => directIds.add(e.to));
        candidates = allNodes.filter(n => directIds.has(n.id));
      }
      visNodes = candidates;
    }

    // Timeline visible set filter: applied after level branch, IDs in tlVisibleSet only
    if (tlVisibleSet !== null) {
      visNodes = visNodes.filter(n => tlVisibleSet.has(n.id));
    }

    const visIds = new Set(visNodes.map(n => n.id));
    visEdges = allEdges.filter(e => visIds.has(e.from) && visIds.has(e.to));
    return { nodes: visNodes, edges: visEdges };
  }

  /* ─── layout: radial rings ───────────────────────────────────────── */
  function layoutNodes(nodes, edges) {
    const byRing = {};
    nodes.forEach(n => {
      const rk = n.ring || 'infra';
      if (!byRing[rk]) byRing[rk] = [];
      byRing[rk].push(n);
    });

    const placed = {};

    function neighborAngle(n) {
      let sx = 0, sy = 0, c = 0;
      edges.forEach(e => {
        let other = null;
        if (e.from === n.id) other = placed[e.to];
        else if (e.to === n.id) other = placed[e.from];
        if (other) {
          const a = Math.atan2(other._z, other._x);
          sx += Math.cos(a); sy += Math.sin(a); c++;
        }
      });
      return c ? Math.atan2(sy, sx) : null;
    }

    // Group components by parent container for angular proximity
    function getParentAngle(n) {
      if (!n.parent || !placed[n.parent]) return null;
      const p = placed[n.parent];
      return Math.atan2(p._z, p._x);
    }

    RING_DEFS.forEach(ring => {
      const arr = byRing[ring.key];
      if (!arr || !arr.length) return;

      if (ring.key === 'core' && arr.length === 1) {
        arr[0]._x = 0; arr[0]._z = 0; arr[0]._y = 0;
        placed[arr[0].id] = arr[0];
        return;
      }

      arr.forEach(n => {
        const pa = getParentAngle(n);
        const na = neighborAngle(n);
        n._want = pa !== null ? pa : (na !== null ? na : ((hash(n.id) % 1000) / 1000) * Math.PI * 2);
      });
      arr.sort((a, b) => a._want - b._want);

      const slots = arr.map((_, i) => (i / arr.length) * Math.PI * 2);
      let sx = 0, sy = 0;
      arr.forEach((n, i) => { const d = n._want - slots[i]; sx += Math.cos(d); sy += Math.sin(d); });
      const off = Math.atan2(sy, sx);

      arr.forEach((n, i) => {
        const h = hash(n.id);
        const angle = slots[i] + off + ((h % 100) / 100 - 0.5) * 0.18;
        const rad = ring.r + (((h >> 3) % 100) / 100 - 0.5) * 3.5;
        n._x = Math.cos(angle) * rad;
        n._z = Math.sin(angle) * rad;
        n._y = 0;
        placed[n.id] = n;
      });
    });

    // Any node not placed (unknown ring) → place at infra radius
    nodes.forEach(n => {
      if (n._x === undefined) {
        const h = hash(n.id);
        const angle = ((h % 1000) / 1000) * Math.PI * 2;
        n._x = Math.cos(angle) * 42;
        n._z = Math.sin(angle) * 42;
        n._y = 0;
      }
    });
  }

  /* ─── scene rebuild ─────────────────────────────────────────────── */
  function clearWorld() {
    if (!worldGroup) return;
    while (worldGroup.children.length) {
      const c = worldGroup.children[worldGroup.children.length - 1];
      c.traverse(o => {
        if (o.geometry) o.geometry.dispose();
        if (o.material) {
          if (o.material.map) o.material.map.dispose();
          o.material.dispose();
        }
      });
      worldGroup.remove(c);
    }
    nodeMeshes = []; edgeMeshes = []; edgeRecs = []; nodeById = {};
    hover = null; hoverEdge = null;
  }

  function rebuild() {
    if (!scene) return;
    clearWorld();
    const { nodes, edges } = getVisibleGraph();

    layoutNodes(nodes, edges);
    nodes.forEach(n => { nodeById[n.id] = n; });

    buildRingGuides(nodes);
    nodes.forEach(n => addNodeMesh(n));
    edges.forEach(e => addEdgeMesh(e));

    updateStatusLegend(nodes);
    updateContainerPicker();
    updateCrumb();

    window.C3_EXPLORER.ready = true;
  }

  /* ─── ring guide circles ─────────────────────────────────────────── */
  function buildRingGuides(nodes) {
    const THREE = window.THREE;
    const usedRings = new Set(nodes.map(n => n.ring || 'infra'));

    RING_DEFS.forEach((ring, i) => {
      if (!usedRings.has(ring.key)) return;
      const prev = RING_DEFS[i - 1];
      const next = RING_DEFS[i + 1];
      const outer = prev ? (prev.r + ring.r) / 2 : ring.r + 8;
      const inner = next ? (ring.r + next.r) / 2 : 0.001;

      // band fill
      const band = new THREE.Mesh(
        new THREE.RingGeometry(inner, outer, 96),
        new THREE.MeshBasicMaterial({ color: ring.tint, transparent: true, opacity: 0.045, side: THREE.DoubleSide, depthWrite: false })
      );
      band.rotation.x = -Math.PI / 2; band.position.y = 0.01;
      worldGroup.add(band);

      // orbit line
      const orb = new THREE.Mesh(
        new THREE.RingGeometry(ring.r - 0.1, ring.r + 0.1, 96),
        new THREE.MeshBasicMaterial({ color: ring.tint, transparent: true, opacity: 0.18, side: THREE.DoubleSide, depthWrite: false })
      );
      orb.rotation.x = -Math.PI / 2; orb.position.y = 0.02;
      worldGroup.add(orb);

      // label
      const lbl = makeLabel(ring.label.toUpperCase(), { ring: true, color: ring.tint });
      lbl.position.set(Math.cos(2.5) * ring.r, 0.4, Math.sin(2.5) * ring.r);
      worldGroup.add(lbl);
    });
  }

  /* ─── node mesh ─────────────────────────────────────────────────── */
  function lifecycleColor(n) {
    const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
    return LIFECYCLE_COLORS[lc] || LIFECYCLE_COLORS.open;
  }

  function addNodeMesh(n) {
    const THREE = window.THREE;
    const grp = new THREE.Group();
    grp.position.set(n._x, 0, n._z);

    const isBig = n.type === 'system' || n.type === 'container';
    const rad = isBig ? 3.2 : 2.4;
    const hgt = isBig ? 2.4 : 1.6;

    let geo;
    if (n.type === 'system')    geo = new THREE.CylinderGeometry(rad, rad * 0.92, hgt, 32);
    else if (n.type === 'adr')  geo = new THREE.BoxGeometry(rad * 1.3, hgt, rad * 1.3);
    else if (n.type === 'rule') geo = new THREE.BoxGeometry(rad * 1.2, hgt, rad * 1.2);
    else if (n.type === 'ref')  geo = new THREE.CylinderGeometry(rad, rad, hgt, 8);
    else                        geo = new THREE.CylinderGeometry(rad, rad * 0.94, hgt, 32);

    const ringTint = ringByKey(n.ring || 'infra').tint;
    const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
    const lcColor = LIFECYCLE_COLORS[lc] || LIFECYCLE_COLORS.open;
    const col = new THREE.Color(ringTint);

    const mat = new THREE.MeshStandardMaterial({
      color: col,
      roughness: 0.62,
      metalness: 0.04,
      emissive: new THREE.Color(ringTint),
      emissiveIntensity: 0.0,
    });
    const mesh = new THREE.Mesh(geo, mat);
    mesh.position.y = hgt / 2;
    grp.add(mesh);

    // lifecycle halo ring (status glyph — AG-4)
    const haloColor = new THREE.Color(lcColor);
    const haloRad = rad * 1.05;
    const halo = new THREE.Mesh(
      new THREE.TorusGeometry(haloRad, 0.18, 8, 48),
      new THREE.MeshBasicMaterial({ color: haloColor, transparent: true, opacity: 0.85 })
    );
    halo.rotation.x = Math.PI / 2;
    halo.position.y = hgt + 0.05;
    halo.userData.isHalo = true;
    halo.userData.baseOpacity = 0.85;
    grp.add(halo);

    // staged pulse indicator: small amber cone glyph
    if (n.staged || lc === 'staged') {
      const stageCone = new THREE.Mesh(
        new THREE.ConeGeometry(0.38, 0.9, 6),
        new THREE.MeshBasicMaterial({ color: new THREE.Color('#d98a2b') })
      );
      stageCone.position.y = hgt + 1.6;
      stageCone.userData.isStagedGlyph = true;
      grp.add(stageCone);
    }

    // contact shadow
    const sh = new THREE.Mesh(
      new THREE.CircleGeometry(rad * 1.5, 24),
      new THREE.MeshBasicMaterial({ color: 0x14161a, transparent: true, opacity: 0.07, depthWrite: false })
    );
    sh.rotation.x = -Math.PI / 2; sh.position.y = 0.04;
    sh.userData.baseOpacity = 0.07;
    grp.add(sh);

    // label
    const lbl = makeLabel(n.title || n.id, { big: isBig });
    lbl.position.y = hgt + (isBig ? 3.0 : 2.2);
    grp.add(lbl);

    mesh.userData.node = n;
    mesh.userData.hasStatusGlyph = true; // every node gets a halo — AG-4 satisfied
    grp.userData = { node: n, mesh, mat, halo, hgt };
    n._grp = grp; n._mesh = mesh; n._mat = mat; n._hgt = hgt;

    worldGroup.add(grp);
    nodeMeshes.push(mesh);
  }

  /* ─── edge mesh ─────────────────────────────────────────────────── */
  function addEdgeMesh(e) {
    const THREE = window.THREE;
    const a = nodeById[e.from], b = nodeById[e.to];
    if (!a || !b) return;

    const kind = e.kind || 'uses';
    const col = new THREE.Color(EDGE_COLORS[kind] || EDGE_COLORS.uses);

    const p1 = new THREE.Vector3(a._x, (a._hgt || 1.5) * 0.7, a._z);
    const p2 = new THREE.Vector3(b._x, (b._hgt || 1.5) * 0.7, b._z);
    const dist = p1.distanceTo(p2);

    // boundary-gate CatmullRom routing
    const pa = { r: Math.hypot(a._x, a._z), t: Math.atan2(a._z, a._x) };
    const pb = { r: Math.hypot(b._x, b._z), t: Math.atan2(b._z, b._x) };
    let dtt = pb.t - pa.t;
    while (dtt > Math.PI) dtt -= 2 * Math.PI;
    while (dtt < -Math.PI) dtt += 2 * Math.PI;
    const lo = Math.min(pa.r, pb.r), hi = Math.max(pa.r, pb.r);
    const crossed = RING_DEFS.filter(rr => rr.r > lo + 1.4 && rr.r < hi - 1.4);
    const gatePts = crossed.map(rr => {
      const f = (rr.r - pa.r) / (pb.r - pa.r || 0.001);
      const ang = pa.t + dtt * f;
      return { f, x: Math.cos(ang) * rr.r, z: Math.sin(ang) * rr.r, label: rr.label };
    }).sort((u, v) => u.f - v.f);

    const peak = Math.min(2.2 + crossed.length * 1.4 + dist * 0.045, 10);
    let curve;
    if (gatePts.length) {
      const pts = [p1];
      gatePts.forEach(gp => pts.push(new THREE.Vector3(gp.x, 1.0 + peak * Math.sin(Math.PI * gp.f), gp.z)));
      pts.push(p2);
      curve = new THREE.CatmullRomCurve3(pts, false, 'centripetal', 0.7);
    } else {
      const mid = p1.clone().add(p2).multiplyScalar(0.5);
      mid.y += Math.min(2.4 + dist * 0.18, 10);
      curve = new THREE.QuadraticBezierCurve3(p1, mid, p2);
    }

    // tube opacity/style by kind
    const tubeOpacity = kind === 'contains' ? 0.22 : 0.32;
    const tubeRadius  = kind === 'contains' ? 0.055 : 0.08;

    const tube = new THREE.Mesh(
      new THREE.TubeGeometry(curve, 32, tubeRadius, 6, false),
      new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: tubeOpacity })
    );
    worldGroup.add(tube);

    // arrowhead at 0.88
    const arrowMeshes = [];
    const addArrow = (t, flip) => {
      const pos = curve.getPoint(t);
      const tan = curve.getTangent(t);
      if (flip) tan.negate();
      const cone = new THREE.Mesh(
        new THREE.ConeGeometry(0.38, 1.05, 8),
        new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: 0.8 })
      );
      cone.position.copy(pos);
      cone.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), tan);
      worldGroup.add(cone);
      arrowMeshes.push(cone);
    };
    if (kind !== 'contains') addArrow(0.88, false);

    // gate markers
    const gates = gatePts.map(gp => {
      const gm = new THREE.Mesh(
        new THREE.RingGeometry(0.3, 0.58, 20),
        new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: 0.5, side: THREE.DoubleSide, depthWrite: false })
      );
      gm.rotation.x = -Math.PI / 2;
      gm.position.set(gp.x, 0.07, gp.z);
      worldGroup.add(gm);
      return gm;
    });

    // particles for uses + affects edges
    const particles = [];
    if (kind === 'uses' || kind === 'affects') {
      const pc = 2;
      const pgeo = new THREE.SphereGeometry(0.22, 8, 8);
      for (let i = 0; i < pc; i++) {
        const pm = new THREE.Mesh(pgeo, new THREE.MeshBasicMaterial({ color: col }));
        pm.userData.t = i / pc;
        pm.userData.speed = 0.09 + Math.random() * 0.03;
        worldGroup.add(pm);
        particles.push(pm);
      }
    }

    // invisible fat pick tube
    const pickTube = new THREE.Mesh(
      new THREE.TubeGeometry(curve, 20, 0.55, 4, false),
      new THREE.MeshBasicMaterial({ visible: false })
    );
    const rec = {
      e, curve, tube, arrowMeshes, particles, gates,
      kind, from: e.from, to: e.to,
      baseOpacity: tubeOpacity,
      crossedLabels: gatePts.map(g => g.label),
    };
    pickTube.userData.rec = rec;
    worldGroup.add(pickTube);
    edgeMeshes.push(pickTube);
    edgeRecs.push(rec);
  }

  /* ─── CanvasTexture label sprites ───────────────────────────────── */
  function makeLabel(text, opts) {
    opts = opts || {};
    const THREE = window.THREE;
    const dpr = 2;
    const fs = opts.ring ? 20 : (opts.big ? 28 : 24);
    const pad = opts.ring ? 0 : 12;
    const cnv = document.createElement('canvas');
    const ctx = cnv.getContext('2d');
    ctx.font = `600 ${fs}px system-ui,-apple-system,sans-serif`;
    const tw = ctx.measureText(text).width;
    cnv.width = (tw + pad * 2) * dpr;
    cnv.height = (fs + pad * 1.2) * dpr;
    const c2 = cnv.getContext('2d');
    c2.scale(dpr, dpr);
    if (!opts.ring) {
      const rw = tw + pad * 2, rh = fs + pad * 1.2, r = 7;
      c2.fillStyle = 'rgba(255,255,255,0.93)';
      c2.strokeStyle = '#e5e7e9'; c2.lineWidth = 1;
      c2.beginPath();
      c2.moveTo(r, 0); c2.arcTo(rw, 0, rw, rh, r); c2.arcTo(rw, rh, 0, rh, r);
      c2.arcTo(0, rh, 0, 0, r); c2.arcTo(0, 0, rw, 0, r); c2.closePath();
      c2.fill(); c2.stroke();
    }
    c2.font = `600 ${fs}px system-ui,-apple-system,sans-serif`;
    c2.fillStyle = opts.ring ? (opts.color || '#6b6f75') : '#1c1f24';
    c2.textBaseline = 'middle';
    if (opts.ring) c2.globalAlpha = 0.65;
    c2.fillText(text, pad, (fs + pad * 1.2) / 2);
    const tex = new THREE.CanvasTexture(cnv);
    tex.minFilter = THREE.LinearFilter;
    tex.needsUpdate = true;
    const spr = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, depthWrite: false, depthTest: true }));
    const wscale = (cnv.width / dpr) / 24 * (opts.big ? 1.15 : 1);
    spr.scale.set(wscale, (cnv.height / dpr) / 24, 1);
    spr.userData.isLabel = true;
    return spr;
  }

  /* ─── status legend ─────────────────────────────────────────────── */
  function updateStatusLegend(nodes) {
    const el = q('c3-status-legend');
    if (!el) return;

    const counts = {};
    nodes.forEach(n => {
      const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
      counts[lc] = (counts[lc] || 0) + 1;
    });

    el.innerHTML = Object.keys(LIFECYCLE_COLORS).filter(lc => counts[lc]).map(lc => {
      const color = LIFECYCLE_COLORS[lc];
      const dimmed = st.dimmedLifecycles.has(lc);
      return `<div class="c3-legend-row" data-lc="${lc}" style="cursor:pointer;display:flex;align-items:center;gap:8px;${dimmed ? 'opacity:0.34;' : ''}">
        <span style="width:12px;height:12px;border-radius:50%;background:${color};flex:none;display:inline-block;"></span>
        <span style="${dimmed ? 'text-decoration:line-through;' : ''}font-size:12.5px;color:var(--ink);">${lc}</span>
        <span style="margin-left:auto;font-size:11px;font-family:ui-monospace,monospace;color:var(--ink-subtle);">${counts[lc]}</span>
      </div>`;
    }).join('');

    el.querySelectorAll('[data-lc]').forEach(row => {
      row.addEventListener('click', () => {
        const lc = row.dataset.lc;
        if (st.dimmedLifecycles.has(lc)) st.dimmedLifecycles.delete(lc);
        else st.dimmedLifecycles.add(lc);
        applyLifecycleDim();
        updateStatusLegend(nodes);
      });
    });
  }

  function applyLifecycleDim() {
    nodeMeshes.forEach(mesh => {
      const n = mesh.userData.node;
      const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
      const dimmed = st.dimmedLifecycles.size > 0 && st.dimmedLifecycles.has(lc);
      const grp = n._grp;
      if (!grp) return;
      grp.traverse(o => {
        const base = (o.userData && o.userData.baseOpacity !== undefined) ? o.userData.baseOpacity : 1;
        if (o.material && !o.userData.isLabel) {
          o.material.transparent = true;
          o.material.opacity = dimmed ? base * 0.25 : base;
        }
        if (o.userData && o.userData.isLabel) {
          o.material.opacity = dimmed ? 0.15 : 1;
        }
      });
    });
  }

  /* ─── container picker ──────────────────────────────────────────── */
  function updateContainerPicker() {
    const el = q('c3-containers');
    if (!el) return;
    el.style.display = st.level === 'component' ? 'flex' : 'none';
    if (st.level !== 'component') return;

    const data = window.C3_DATA;
    if (!data) return;
    const containers = (data.nodes || []).filter(n => n.type === 'container');
    el.innerHTML = containers.map(c => {
      const active = st.focusContainer === c.id;
      return `<button data-container="${c.id}" style="border:0;background:${active ? 'var(--mint-pale)' : 'transparent'};color:${active ? 'var(--mint-ink)' : 'var(--ink-muted)'};font-family:inherit;font-size:12px;font-weight:600;padding:6px 12px;border-radius:6px;cursor:pointer;">${c.title || c.id}</button>`;
    }).join('');
    el.querySelectorAll('[data-container]').forEach(btn => {
      btn.addEventListener('click', () => {
        st.focusContainer = btn.dataset.container;
        clearSelection();
        rebuild();
        resetCamera();
      });
    });
  }

  /* ─── level switcher ────────────────────────────────────────────── */
  function setLevel(lvl) {
    st.level = lvl;
    st.query = '';
    const si = q('c3-search');
    if (si) si.value = '';
    clearSelection();
    paintLevelButtons();
    rebuild();
    resetCamera();
  }

  function paintLevelButtons() {
    const el = q('c3-levels');
    if (!el) return;
    el.querySelectorAll('button[data-level]').forEach(btn => {
      const on = btn.dataset.level === st.level;
      btn.style.background = on ? 'var(--ink)' : 'transparent';
      btn.style.color = on ? '#fff' : 'var(--ink-muted)';
    });
  }

  /* ─── interactions ──────────────────────────────────────────────── */
  function ndcFromEvent(e) {
    const rect = renderer.domElement.getBoundingClientRect();
    pointer.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    pointer.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
  }

  function pickNode() {
    raycaster.setFromCamera(pointer, camera);
    const hits = raycaster.intersectObjects(nodeMeshes, false);
    return hits.length ? hits[0].object : null;
  }

  function pickEdge() {
    raycaster.setFromCamera(pointer, camera);
    const hits = raycaster.intersectObjects(edgeMeshes, false);
    return hits.length ? hits[0].object.userData.rec : null;
  }

  function onPointerMove(e) {
    ndcFromEvent(e);
    const m = pickNode();
    const tip = q('c3-tooltip');

    if (m) {
      renderer.domElement.style.cursor = 'pointer';
      const n = m.userData.node;
      if (hoverEdge) { hoverEdge = null; highlightConnections(hover); }
      if (hover !== n) { hover = n; highlightConnections(n); }
      if (tip) {
        tip.hidden = false;
        tip.style.left = e.clientX + 'px';
        tip.style.top = e.clientY + 'px';
        const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
        tip.textContent = (n.title || n.id) + ' · ' + (n.type || '') + ' · ' + lc;
      }
      return;
    }

    const rec = pickEdge();
    if (rec) {
      renderer.domElement.style.cursor = 'pointer';
      if (hover) { hover = null; highlightConnections(null); }
      if (hoverEdge !== rec) { hoverEdge = rec; boostEdge(rec); }
      const na = nodeById[rec.from], nb = nodeById[rec.to];
      if (tip) {
        tip.hidden = false;
        tip.style.left = e.clientX + 'px';
        tip.style.top = e.clientY + 'px';
        tip.textContent = (na ? (na.title || na.id) : rec.from) + ' → ' + (nb ? (nb.title || nb.id) : rec.to) + ' · ' + rec.kind;
      }
      return;
    }

    renderer.domElement.style.cursor = 'grab';
    if (hoverEdge) { hoverEdge = null; highlightConnections(hover); }
    if (hover) { hover = null; highlightConnections(null); }
    if (tip) tip.hidden = true;
  }

  let _downX = 0, _downY = 0;
  function onPointerDown(e) { _downX = e.clientX; _downY = e.clientY; }
  function onPointerUp(e) {
    if (Math.abs(e.clientX - _downX) < 5 && Math.abs(e.clientY - _downY) < 5) onSingleClick(e);
  }
  function onDblClick(e) {
    ndcFromEvent(e);
    const m = pickNode();
    if (!m) return;
    const n = m.userData.node;
    // if it's a container, drill into component level focused on it
    if (n.type === 'container' || n.type === 'system') {
      st.focusContainer = n.id;
      setLevel('component');
    } else {
      selectNode(n);
    }
  }

  function onSingleClick(e) {
    ndcFromEvent(e);
    const m = pickNode();
    if (m) { selectNode(m.userData.node); return; }
    const rec = pickEdge();
    if (rec) selectEdge(rec);
  }

  /* ─── highlight / dim ────────────────────────────────────────────── */
  function highlightConnections(n) {
    edgeRecs.forEach(r => {
      const touch = n && (r.from === n.id || r.to === n.id);
      const op = n ? (touch ? 0.88 : 0.06) : r.baseOpacity;
      r.tube.material.opacity = op;
      r.arrowMeshes.forEach(ar => { ar.material.opacity = n ? (touch ? 0.92 : 0.05) : 0.8; });
      r.gates.forEach(g => { g.material.opacity = n ? (touch ? 0.8 : 0.04) : 0.5; });
      r.particles.forEach(p => {
        p.visible = !n || touch;
        if (!n || touch) p.scale.setScalar(n && touch ? 1.45 : 1);
      });
    });
    nodeMeshes.forEach(mesh => {
      const nn = mesh.userData.node;
      const grp = nn._grp;
      if (!grp) return;
      let connected = !n || nn.id === n.id;
      if (n && !connected) {
        connected = edgeRecs.some(r => (r.from === n.id && r.to === nn.id) || (r.to === n.id && r.from === nn.id));
      }
      const dim = n && !connected;
      grp.traverse(o => {
        const base = (o.userData && o.userData.baseOpacity !== undefined) ? o.userData.baseOpacity : 1;
        if (o.material && !o.userData.isLabel) {
          o.material.transparent = true;
          o.material.opacity = dim ? base * 0.22 : base;
        }
        if (o.userData && o.userData.isLabel) o.material.opacity = dim ? 0.12 : 1;
      });
      mesh.material.emissiveIntensity = (n && nn.id === n.id) ? 0.32 :
        (st.selected && st.selected.id === nn.id ? 0.38 : 0);
    });
  }

  function boostEdge(rec) {
    highlightConnections(null);
    edgeRecs.forEach(r => {
      if (r === rec) return;
      r.tube.material.opacity = Math.min(r.tube.material.opacity, 0.07);
      r.arrowMeshes.forEach(a2 => { a2.material.opacity = 0.06; });
      r.gates.forEach(g2 => { g2.material.opacity = 0.04; });
      r.particles.forEach(p => { p.visible = false; });
    });
    rec.tube.material.opacity = 0.95;
    rec.arrowMeshes.forEach(a2 => { a2.material.opacity = 1; });
    rec.gates.forEach(g2 => { g2.material.opacity = 0.88; });
    rec.particles.forEach(p => { p.visible = true; p.scale.setScalar(1.6); });
  }

  /* ─── search ────────────────────────────────────────────────────── */
  function matches(n) {
    const qi = st.query;
    if (!qi) return false;
    return ((n.title || '') + ' ' + n.id + ' ' + (n.type || '') + ' ' + (n.lifecycle || '')).toLowerCase().includes(qi);
  }

  function applySearch() {
    const qi = st.query;
    nodeMeshes.forEach(mesh => {
      const n = mesh.userData.node;
      const hit = qi && matches(n);
      n._searchHit = hit;
      const grp = n._grp;
      if (!grp) return;
      if (qi) {
        grp.traverse(o => {
          const base = (o.userData && o.userData.baseOpacity !== undefined) ? o.userData.baseOpacity : 1;
          if (o.material && !o.userData.isLabel) { o.material.transparent = true; o.material.opacity = hit ? base : base * 0.17; }
          if (o.userData && o.userData.isLabel) o.material.opacity = hit ? 1 : 0.13;
        });
      } else {
        grp.traverse(o => {
          const base = (o.userData && o.userData.baseOpacity !== undefined) ? o.userData.baseOpacity : 1;
          if (o.material) o.material.opacity = base;
        });
      }
    });
    if (!qi) highlightConnections(hover);
  }

  /* ─── selection ─────────────────────────────────────────────────── */
  function selectNode(n) {
    st.selected = n;
    const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
    const lcColor = LIFECYCLE_COLORS[lc] || LIFECYCLE_COLORS.open;

    const db = q('c3-detail');
    if (db) db.hidden = false;

    const badge = q('c3-detail-badge');
    if (badge) {
      badge.textContent = (n.type || 'node').toUpperCase();
      badge.style.background = lcColor + '22';
      badge.style.color = lcColor;
    }

    const titleEl = q('c3-detail-title');
    if (titleEl) titleEl.textContent = n.title || n.id;

    const lcEl = q('c3-detail-lifecycle');
    if (lcEl) {
      lcEl.textContent = lc.toUpperCase();
      lcEl.style.color = lcColor;
    }

    // build detail body
    const bodyEl = q('c3-detail-body');
    if (bodyEl) {
      const usesEdges = edgeRecs.filter(r => r.kind === 'uses' && r.from === n.id);
      const usedByEdges = edgeRecs.filter(r => r.kind === 'uses' && r.to === n.id);
      const containsEdges = edgeRecs.filter(r => r.kind === 'contains' && r.from === n.id);

      const edgeList = (arr, label) => arr.length ? `<div style="margin-top:10px;"><div style="font-size:11px;font-weight:600;letter-spacing:0.7px;text-transform:uppercase;color:var(--ink-subtle);margin-bottom:5px;">${label}</div>${arr.map(r => {
        const other = r.from === n.id ? r.to : r.from;
        const on = nodeById[other];
        return `<div style="font-size:12.5px;color:var(--ink-muted);padding:2px 0;">${on ? (on.title || on.id) : other}</div>`;
      }).join('')}</div>` : '';

      let stagedSection = '';
      if (n.staged && n.transition) {
        stagedSection = `<div style="margin-top:10px;padding:8px 10px;background:#d98a2b1a;border-radius:6px;border-left:3px solid #d98a2b;">
          <div style="font-size:11px;font-weight:600;color:#d98a2b;margin-bottom:4px;">STAGED TRANSITION</div>
          <div style="font-size:12.5px;color:var(--ink);">${n.transition.from} → ${n.transition.to}</div>
          <div style="font-size:12px;color:var(--ink-muted);">by ${n.transition.by}</div>
          ${(n.stagedBy && n.stagedBy.length) ? `<div style="font-size:12px;color:var(--ink-muted);margin-top:4px;">staged by: ${n.stagedBy.join(', ')}</div>` : ''}
        </div>`;
      }

      bodyEl.innerHTML = `
        ${n.goal ? `<div style="font-size:13.5px;line-height:1.5;color:var(--ink);margin-bottom:10px;">${n.goal}</div>` : ''}
        <div style="font-size:11px;color:var(--ink-subtle);font-family:ui-monospace,monospace;">${n.id}${n.parent ? ` ∈ ${n.parent}` : ''}</div>
        ${stagedSection}
        ${edgeList(containsEdges, 'Contains')}
        ${edgeList(usesEdges, 'Uses')}
        ${edgeList(usedByEdges, 'Used by')}
      `;
    }

    highlightConnections(null);
    flyTo(n);
    updateCrumb();
  }

  function selectEdge(rec) {
    st.selected = null;
    const na = nodeById[rec.from], nb = nodeById[rec.to];
    const db = q('c3-detail');
    if (db) db.hidden = false;
    const badge = q('c3-detail-badge');
    if (badge) { badge.textContent = 'EDGE · ' + rec.kind.toUpperCase(); badge.style.background = ''; badge.style.color = EDGE_COLORS[rec.kind] || '#7c8794'; }
    const titleEl = q('c3-detail-title');
    if (titleEl) titleEl.textContent = (na ? (na.title || na.id) : rec.from) + ' → ' + (nb ? (nb.title || nb.id) : rec.to);
    const lcEl = q('c3-detail-lifecycle');
    if (lcEl) lcEl.textContent = '';
    const bodyEl = q('c3-detail-body');
    if (bodyEl) {
      const crossSection = rec.crossedLabels && rec.crossedLabels.length
        ? `<div style="margin-top:8px;font-size:12.5px;color:var(--ink-muted);">Crosses boundaries: ${rec.crossedLabels.join(', ')}</div>`
        : `<div style="margin-top:8px;font-size:12.5px;color:var(--ink-muted);">Stays within one boundary layer.</div>`;
      bodyEl.innerHTML = `<div style="font-size:13.5px;color:var(--ink);">Kind: <b>${rec.kind}</b></div>${crossSection}`;
    }
    boostEdge(rec);
    const mid = rec.curve.getPoint(0.5);
    const dir = camera.position.clone().sub(controls.target).normalize();
    const toPos = mid.clone().add(dir.multiplyScalar(28));
    toPos.y = Math.max(toPos.y, 14);
    st._cam = { flying: true, t: 0, fromPos: camera.position.clone(), toPos, fromTgt: controls.target.clone(), toTgt: mid.clone() };
  }

  function clearSelection() {
    st.selected = null;
    const db = q('c3-detail');
    if (db) db.hidden = true;
    highlightConnections(hover);
    updateCrumb();
  }

  /* ─── camera ────────────────────────────────────────────────────── */
  function flyTo(n) {
    const THREE = window.THREE;
    const tgt = new THREE.Vector3(n._x, (n._hgt || 1.5), n._z);
    const dir = camera.position.clone().sub(controls.target).normalize();
    const dist = n.type === 'system' || n.type === 'container' ? 32 : 24;
    const toPos = tgt.clone().add(dir.multiplyScalar(dist));
    toPos.y = Math.max(toPos.y, 14);
    st._cam = { flying: true, t: 0, fromPos: camera.position.clone(), toPos, fromTgt: controls.target.clone(), toTgt: tgt };
  }

  function resetCamera() {
    const THREE = window.THREE;
    const toPos = new THREE.Vector3(6, 74, 88);
    const toTgt = new THREE.Vector3(0, 0, 0);
    st._cam = { flying: true, t: 0, fromPos: camera.position.clone(), toPos, fromTgt: controls.target.clone(), toTgt };
  }

  /* ─── breadcrumb ────────────────────────────────────────────────── */
  function updateCrumb() {
    const el = q('c3-crumb');
    if (!el) return;
    const levelMap = { context: 'C1 · Context', container: 'C2 · Containers', component: 'C3 · Components' };
    const data = window.C3_DATA;
    const proj = data ? (data.project || 'c3') : 'c3';
    let t = proj + ' / ' + (levelMap[st.level] || st.level);
    if (st.level === 'component' && st.focusContainer) {
      const cn = (data && data.nodes || []).find(n => n.id === st.focusContainer);
      t += ' / ' + (cn ? (cn.title || cn.id) : st.focusContainer);
    }
    if (st.selected) t += '  ›  ' + (st.selected.title || st.selected.id);
    el.textContent = t;
  }

  /* ─── animation loop ─────────────────────────────────────────────── */
  function loop() {
    _raf = requestAnimationFrame(loop);
    st._t += 0.016;

    // particles
    edgeRecs.forEach(r => {
      r.particles.forEach(p => {
        if (!p.visible) { p.material.opacity = 0; return; }
        p.userData.t = (p.userData.t + p.userData.speed * 0.016) % 1;
        const pos = r.curve.getPoint(p.userData.t);
        p.position.copy(pos);
        p.material.transparent = true;
        p.material.opacity = 0.9;
        // affects edges: pulsing opacity
        if (r.kind === 'affects') p.material.opacity = 0.5 + 0.5 * Math.sin(st._t * 6 + p.userData.t * Math.PI * 2);
      });
    });

    // timeline: born-scale animation (ease-out scale from ~0.01 to 1 over 0.6s)
    if (tl.active) {
      nodeMeshes.forEach(mesh => {
        const n = mesh.userData.node;
        const grp = n._grp;
        if (!grp) return;
        if (n._bornAt !== undefined) {
          const elapsed = st._t - n._bornAt;
          if (elapsed < 0.6) {
            const raw = elapsed / 0.6;
            // ease-out cubic: 1 - (1-t)^3
            const eased = 1 - Math.pow(1 - raw, 3);
            const s = Math.max(0.01, Math.min(1, eased));
            grp.scale.setScalar(s);
          } else {
            grp.scale.setScalar(1);
          }
        }
        // flash modifier nodes: amber emissive sine pulse
        if (n._flashUntil !== undefined && st._t < n._flashUntil) {
          mesh.material.emissive = new window.THREE.Color('#d98a2b');
          mesh.material.emissiveIntensity = 0.25 + 0.35 * (0.5 + 0.5 * Math.sin(st._t * 6));
        } else if (n._flashUntil !== undefined && st._t >= n._flashUntil) {
          // restore: clear flash marker and reset emissive
          delete n._flashUntil;
          mesh.material.emissive = new window.THREE.Color(0x000000);
          mesh.material.emissiveIntensity = 0;
        }
      });
    }

    // staged node amber pulse (emissive sine)
    nodeMeshes.forEach(mesh => {
      const n = mesh.userData.node;
      const lc = n.lifecycle || (n.staged ? 'staged' : 'open');
      if (lc === 'staged' || n.staged) {
        mesh.material.emissive = new window.THREE.Color('#d98a2b');
        mesh.material.emissiveIntensity = 0.15 + 0.22 * (0.5 + 0.5 * Math.sin(st._t * 3.5));
      }
    });

    // search pulse
    if (st.query) {
      const pulse = 0.5 + 0.5 * Math.sin(st._t * 4);
      nodeMeshes.forEach(mesh => {
        if (mesh.userData.node._searchHit) {
          mesh.material.emissive = new window.THREE.Color('#2fa89a');
          mesh.material.emissiveIntensity = 0.22 + pulse * 0.45;
        }
      });
    }

    // selected pulse
    if (st.selected && st.selected._mesh) {
      st.selected._mesh.material.emissiveIntensity = 0.28 + 0.22 * (0.5 + 0.5 * Math.sin(st._t * 2.8));
    }

    // camera flyTo
    if (st._cam && st._cam.flying) {
      st._cam.t += 0.022;
      const t = Math.min(st._cam.t, 1);
      const e = easeInOutQuad(t);
      camera.position.lerpVectors(st._cam.fromPos, st._cam.toPos, e);
      controls.target.lerpVectors(st._cam.fromTgt, st._cam.toTgt, e);
      if (t >= 1) st._cam.flying = false;
    }

    controls.update();
    renderer.render(scene, camera);
  }

  /* ─── init scene ─────────────────────────────────────────────────── */
  function initScene() {
    const THREE = window.THREE;
    const canvas = q('c3-canvas');
    if (!canvas) { console.error('C3 Explorer: #c3-canvas not found'); return; }

    const w = canvas.clientWidth || window.innerWidth;
    const h = canvas.clientHeight || window.innerHeight;

    renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.setSize(w, h);
    if (THREE.sRGBEncoding !== undefined) renderer.outputEncoding = THREE.sRGBEncoding;

    scene = new THREE.Scene();
    scene.background = new THREE.Color('#f5f4f0');
    scene.fog = new THREE.Fog('#f5f4f0', 100, 230);

    camera = new THREE.PerspectiveCamera(46, w / h, 0.5, 3000);
    camera.position.set(6, 74, 88);

    controls = new THREE.OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.08;
    controls.minDistance = 15;
    controls.maxDistance = 180;
    controls.maxPolarAngle = 1.44;
    controls.target.set(0, 0, 0);

    // lights
    scene.add(new THREE.HemisphereLight(0xffffff, 0xe4e2dc, 1.05));
    const dl = new THREE.DirectionalLight(0xffffff, 0.52); dl.position.set(34, 70, 26); scene.add(dl);
    const dl2 = new THREE.DirectionalLight(0xffffff, 0.20); dl2.position.set(-40, 30, -30); scene.add(dl2);

    // ground
    const ground = new THREE.Mesh(
      new THREE.CircleGeometry(130, 64),
      new THREE.MeshStandardMaterial({ color: '#eeede8', roughness: 1, metalness: 0 })
    );
    ground.rotation.x = -Math.PI / 2; ground.position.y = -0.05;
    scene.add(ground);

    raycaster = new THREE.Raycaster();
    pointer = new THREE.Vector2(-2, -2);

    worldGroup = new THREE.Group();
    scene.add(worldGroup);

    // events
    const dom = renderer.domElement;
    dom.addEventListener('pointermove', onPointerMove);
    dom.addEventListener('pointerdown', onPointerDown);
    dom.addEventListener('pointerup', onPointerUp);
    dom.addEventListener('dblclick', onDblClick);

    // resize
    const ro = new ResizeObserver(() => {
      if (!renderer) return;
      const mount = q('c3-canvas');
      if (!mount) return;
      const nw = mount.clientWidth, nh = mount.clientHeight;
      if (!nw || !nh) return;
      camera.aspect = nw / nh;
      camera.updateProjectionMatrix();
      renderer.setSize(nw, nh);
    });
    ro.observe(canvas.parentElement || document.body);

    window.addEventListener('resize', () => {
      const nw = canvas.clientWidth || window.innerWidth;
      const nh = canvas.clientHeight || window.innerHeight;
      if (!nw || !nh) return;
      camera.aspect = nw / nh;
      camera.updateProjectionMatrix();
      renderer.setSize(nw, nh);
    });

    _booted = true;
    rebuild();

    const loading = q('c3-loading');
    if (loading) loading.hidden = true;

    loop();
  }

  /* ─── timeline mode ─────────────────────────────────────────────── */

  function tlEvents() {
    return (window.C3_DATA && window.C3_DATA.events) ? window.C3_DATA.events : [];
  }

  // Compute the visible set for a given event index (union of creates[0..i] + adr nodes whose event <= i)
  function tlComputeVisibleSet(idx) {
    const events = tlEvents();
    const allNodes = (window.C3_DATA && window.C3_DATA.nodes) || [];

    // Build a map: adr event id -> event index
    const adrEventIndex = {};
    events.forEach((ev, i) => { adrEventIndex[ev.id] = i; });

    const visible = new Set();

    // Add all creates from events[0..idx]
    for (let i = 0; i <= idx; i++) {
      const ev = events[i];
      if (!ev) continue;
      (ev.creates || []).forEach(id => visible.add(id));
    }

    // Add adr nodes: show if their matching event index <= idx, or always if no matching event
    allNodes.forEach(n => {
      if (n.type !== 'adr') return;
      if (visible.has(n.id)) return; // already added via creates
      const evIdx = adrEventIndex[n.id];
      if (evIdx === undefined || evIdx <= idx) {
        visible.add(n.id);
      }
    });

    return visible;
  }

  function tlUpdateCard(idx) {
    const events = tlEvents();
    const ev = events[idx];
    const cardEl = q('c3-tl-card');
    if (!cardEl || !ev) return;

    const statusColor = LIFECYCLE_COLORS[ev.status] || '#6b7280';
    const creates = (ev.creates || []).length;
    const modifies = (ev.modifies || []).length;
    cardEl.innerHTML =
      '<div class="c3-tl-date">' + (ev.date || '') + ' · ' + (idx + 1) + '/' + events.length + '</div>' +
      '<div class="c3-tl-title">' + (ev.title || ev.id) + '</div>' +
      '<div class="c3-tl-delta">' +
        '<span class="c3-tl-status" style="background:' + statusColor + '">' + (ev.status || '') + '</span>' +
        ' +' + creates + ' created · ~' + modifies + ' touched' +
      '</div>';
  }

  function tlUpdateScrubber(idx) {
    const scrub = q('c3-tl-scrub');
    if (scrub) scrub.value = idx;
  }

  // Fly camera to centroid of a set of node ids (among currently placed nodes)
  function tlFlyToCentroid(ids) {
    if (!ids || !ids.length) return;
    const THREE = window.THREE;
    if (!THREE) return;
    let sx = 0, sz = 0, count = 0;
    ids.forEach(id => {
      const n = nodeById[id];
      if (n && n._x !== undefined) { sx += n._x; sz += n._z; count++; }
    });
    if (!count) return;
    const cx = sx / count, cz = sz / count;
    const tgt = new THREE.Vector3(cx, 0, cz);
    const dir = camera.position.clone().sub(controls.target).normalize();
    const toPos = tgt.clone().add(dir.multiplyScalar(48));
    toPos.y = Math.max(toPos.y, 20);
    st._cam = { flying: true, t: 0, fromPos: camera.position.clone(), toPos, fromTgt: controls.target.clone(), toTgt: tgt };
  }

  function goToEvent(idx) {
    const events = tlEvents();
    if (!events.length) return;
    idx = Math.max(0, Math.min(idx, events.length - 1));
    tl.index = idx;

    const ev = events[idx];

    // Compute and apply new visible set
    tlVisibleSet = tlComputeVisibleSet(idx);

    // Rebuild with new visible set
    rebuild();

    // Mark entering nodes (this event's creates) for born-scale animation
    if (ev) {
      (ev.creates || []).forEach(id => {
        const n = nodeById[id];
        if (n) n._bornAt = st._t;
      });

      // Mark modified nodes for amber flash
      (ev.modifies || []).forEach(id => {
        const n = nodeById[id];
        if (n) n._flashUntil = st._t + 1.6;
      });

      // Fly to centroid of creates + modifies
      const camIds = (ev.creates || []).concat(ev.modifies || []);
      if (camIds.length) tlFlyToCentroid(camIds);
    }

    tlUpdateScrubber(idx);
    tlUpdateCard(idx);
  }

  function tlGetSpeed() {
    const sel = q('c3-tl-speed');
    if (!sel) return 1;
    return parseFloat(sel.value) || 1;
  }

  function tlPause() {
    tl.playing = false;
    if (tl.timer !== null) { clearTimeout(tl.timer); tl.timer = null; }
    const btn = q('c3-tl-play');
    if (btn) btn.textContent = '▶';
  }

  function tlAdvance() {
    if (!tl.active || !tl.playing) return;
    const events = tlEvents();
    if (tl.index >= events.length - 1) {
      tlPause();
      return;
    }
    goToEvent(tl.index + 1);
    const delay = 2200 / tlGetSpeed();
    tl.timer = setTimeout(tlAdvance, delay);
  }

  function tlPlay() {
    if (!tl.active) return;
    const events = tlEvents();
    if (!events.length) return;
    // If at the end, restart from beginning
    if (tl.index >= events.length - 1) goToEvent(0);
    tl.playing = true;
    const btn = q('c3-tl-play');
    if (btn) btn.textContent = '⏸';
    const delay = 2200 / tlGetSpeed();
    tl.timer = setTimeout(tlAdvance, delay);
  }

  function toggleTimeline(on) {
    const events = tlEvents();

    // If events missing or empty: hide toggle and return
    if (!events.length) {
      const tog = q('c3-timeline-toggle');
      if (tog) tog.hidden = true;
      return;
    }

    if (on === undefined) on = !tl.active;

    if (on && !tl.active) {
      // Remember current state
      tlSavedLevel = st.level;
      tlSavedFocus = st.focusContainer;

      tl.active = true;
      tl.index = 0;
      tl.playing = false;

      // Switch to full-graph (container) level
      st.level = 'container';
      clearSelection();
      st.query = '';
      const si = q('c3-search');
      if (si) si.value = '';

      // Show bar, wire scrubber max
      const bar = q('c3-timeline');
      if (bar) bar.removeAttribute('hidden');
      const tog = q('c3-timeline-toggle');
      if (tog) tog.classList.add('active');
      const scrub = q('c3-tl-scrub');
      if (scrub) scrub.max = events.length - 1;
      const btn = q('c3-tl-play');
      if (btn) btn.textContent = '▶';

      goToEvent(0);

    } else if (!on && tl.active) {
      // Exit mode
      tlPause();
      tl.active = false;
      tl.index = 0;

      // Clear visible set filter
      tlVisibleSet = null;

      // Hide bar, remove active
      const bar = q('c3-timeline');
      if (bar) bar.hidden = true;
      const tog = q('c3-timeline-toggle');
      if (tog) tog.classList.remove('active');

      // Restore level/focus
      st.level = tlSavedLevel;
      st.focusContainer = tlSavedFocus;

      rebuild();
      resetCamera();
    }
  }

  function tlWireUI() {
    const events = tlEvents();

    // Hide toggle if no events
    const tog = q('c3-timeline-toggle');
    if (tog) {
      if (!events.length) {
        tog.hidden = true;
      } else {
        tog.hidden = false;
        tog.addEventListener('click', () => toggleTimeline());
      }
    }

    // Play/pause
    const playBtn = q('c3-tl-play');
    if (playBtn) {
      playBtn.addEventListener('click', () => {
        if (tl.playing) tlPause(); else tlPlay();
      });
    }

    // Scrubber
    const scrub = q('c3-tl-scrub');
    if (scrub) {
      scrub.addEventListener('input', () => {
        tlPause();
        goToEvent(parseInt(scrub.value, 10) || 0);
      });
    }

    // Speed select: no action needed, tlGetSpeed() reads it on each tick
  }

  /* ─── wire up DOM controls ───────────────────────────────────────── */
  function wireUI() {
    // level buttons
    const levelsEl = q('c3-levels');
    if (levelsEl) {
      levelsEl.querySelectorAll('button[data-level]').forEach(btn => {
        btn.addEventListener('mouseenter', () => {
          if (btn.dataset.level !== st.level) { btn.style.background = 'var(--surface-hover)'; btn.style.color = 'var(--ink)'; }
        });
        btn.addEventListener('mouseleave', () => paintLevelButtons());
        btn.addEventListener('click', () => setLevel(btn.dataset.level));
      });
    }
    paintLevelButtons();

    // search
    const si = q('c3-search');
    if (si) {
      si.addEventListener('input', () => { st.query = si.value.trim().toLowerCase(); applySearch(); });
      si.addEventListener('keydown', e => {
        if (e.key === 'Enter') {
          const hit = nodeMeshes.find(m => matches(m.userData.node));
          if (hit) selectNode(hit.userData.node);
        }
      });
      si.addEventListener('focus', () => { si.style.borderColor = 'var(--mint)'; });
      si.addEventListener('blur', () => { si.style.borderColor = 'var(--input-stroke)'; });
    }

    // close button
    const close = q('c3-detail-close');
    if (close) close.addEventListener('click', clearSelection);

    // project name from data
    const data = window.C3_DATA;
    if (data && data.project) {
      const pn = q('c3-project-name');
      if (pn) pn.textContent = data.project;
    }

    // initial container focus
    if (data && data.nodes) {
      const first = data.nodes.find(n => n.type === 'container');
      if (first && !st.focusContainer) st.focusContainer = first.id;
    }

    tlWireUI();
  }

  /* ─── boot ───────────────────────────────────────────────────────── */
  function boot() {
    if (!window.C3_DATA) {
      const lo = q('c3-loading');
      if (lo) {
        const sub = lo.querySelector('div:last-child');
        if (sub) sub.textContent = 'No architecture data. Set window.C3_DATA before loading this script.';
      }
      return;
    }

    wireUI();

    // wait for THREE + OrbitControls
    let tries = 0;
    function tryInit() {
      if (window.THREE && window.THREE.OrbitControls) { initScene(); return; }
      tries++;
      if (tries > 120) {
        const lo = q('c3-loading');
        if (lo) { const sub = lo.querySelector('div:last-child'); if (sub) sub.textContent = 'Could not load 3D engine.'; }
        return;
      }
      setTimeout(tryInit, 60);
    }
    tryInit();
  }

  document.addEventListener('DOMContentLoaded', boot);
  // also run immediately in case DOMContentLoaded already fired
  if (document.readyState === 'interactive' || document.readyState === 'complete') boot();

})();
