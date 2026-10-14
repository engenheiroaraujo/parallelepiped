// Draws a fake neuron graph into an <svg data-graph> element for the layout
// prototypes: glowing nodes in folder colours, curved fibres, a few pulses.
// Attributes: data-nodes (count), data-seed, data-focus ("x,y" in % to draw
// a highlighted neuron with its neighbours), data-spread (0..1).
(function () {
  const palette = ['#c084fc', '#f472b6', '#34d399', '#60a5fa', '#fb923c', '#facc15', '#2dd4bf', '#f87171'];

  function rng(seed) {
    let s = seed >>> 0;
    return () => {
      s = (s + 0x6d2b79f5) | 0;
      let t = Math.imul(s ^ (s >>> 15), 1 | s);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  function draw(svg) {
    const w = svg.clientWidth || 800;
    const h = svg.clientHeight || 600;
    svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
    const random = rng(Number(svg.dataset.seed || 7));
    const count = Number(svg.dataset.nodes || 60);
    const spread = Number(svg.dataset.spread || 0.38);
    const ns = 'http://www.w3.org/2000/svg';
    const el = (tag, attrs) => {
      const e = document.createElementNS(ns, tag);
      for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v);
      return e;
    };

    // Clusters around a few centres, like folders.
    const centres = Array.from({ length: 5 }, () => [w * (0.5 + (random() - 0.5) * 0.55), h * (0.5 + (random() - 0.5) * 0.5)]);
    const nodes = Array.from({ length: count }, (_, i) => {
      const c = i % centres.length;
      const angle = random() * Math.PI * 2;
      const dist = Math.sqrt(random()) * Math.min(w, h) * spread * 0.55;
      const degree = Math.floor(random() ** 3 * 12) + 1;
      return { x: centres[c][0] + Math.cos(angle) * dist, y: centres[c][1] + Math.sin(angle) * dist, color: palette[c % palette.length], degree, cluster: c };
    });

    const defs = el('defs', {});
    defs.innerHTML = '<filter id="glow" x="-200%" y="-200%" width="500%" height="500%"><feGaussianBlur stdDeviation="6"/></filter>';
    svg.appendChild(defs);

    const links = [];
    nodes.forEach((n, i) => {
      const k = 1 + Math.floor(random() * 2);
      for (let j = 0; j < k; j++) {
        const sameCluster = random() < 0.8;
        const pool = nodes.filter((m, mi) => mi !== i && (!sameCluster || m.cluster === n.cluster));
        const m = pool[Math.floor(random() * pool.length)];
        if (m) links.push([n, m]);
      }
    });

    const fibres = el('g', { opacity: 0.55 });
    for (const [a, b] of links) {
      const mx = (a.x + b.x) / 2 + (random() - 0.5) * 40;
      const my = (a.y + b.y) / 2 + (random() - 0.5) * 40;
      fibres.appendChild(el('path', { d: `M${a.x},${a.y} Q${mx},${my} ${b.x},${b.y}`, stroke: a.color, 'stroke-opacity': 0.35, 'stroke-width': 0.8, fill: 'none' }));
    }
    svg.appendChild(fibres);

    // A few pulses travelling on fibres.
    for (let i = 0; i < Math.min(14, links.length); i++) {
      const [a, b] = links[Math.floor(random() * links.length)];
      const t = random();
      svg.appendChild(el('circle', { cx: a.x + (b.x - a.x) * t, cy: a.y + (b.y - a.y) * t, r: 1.8, fill: '#e0f2fe', opacity: 0.9 }));
    }

    for (const n of nodes) {
      const r = 2 + Math.sqrt(n.degree) * 1.6;
      svg.appendChild(el('circle', { cx: n.x, cy: n.y, r: r * 2.6, fill: n.color, opacity: 0.18, filter: 'url(#glow)' }));
      // Short dendrites.
      for (let d = 0; d < 4; d++) {
        const ang = random() * Math.PI * 2;
        const len = r * (2 + random() * 2.5);
        svg.appendChild(el('line', { x1: n.x, y1: n.y, x2: n.x + Math.cos(ang) * len, y2: n.y + Math.sin(ang) * len, stroke: n.color, 'stroke-opacity': 0.45, 'stroke-width': 0.8, 'stroke-linecap': 'round' }));
      }
      svg.appendChild(el('circle', { cx: n.x, cy: n.y, r, fill: n.color }));
    }

    if (svg.dataset.labels) {
      const names = svg.dataset.labels.split('|');
      nodes.slice(0, names.length).forEach((n, i) => {
        const t = el('text', { x: n.x, y: n.y + 16 + n.degree, fill: '#a3a3ab', 'font-size': 11, 'text-anchor': 'middle', 'font-family': 'Segoe UI, sans-serif' });
        t.textContent = names[i];
        svg.appendChild(t);
      });
    }

    if (svg.dataset.focus) {
      const [fx, fy] = svg.dataset.focus.split(',').map(Number);
      const cx = (w * fx) / 100;
      const cy = (h * fy) / 100;
      const ring = el('g', {});
      const neighbours = nodes.filter(() => random() < 0.12).slice(0, 7);
      for (const n of neighbours) ring.appendChild(el('line', { x1: cx, y1: cy, x2: n.x, y2: n.y, stroke: '#b4a9ff', 'stroke-width': 1.4, opacity: 0.8 }));
      ring.appendChild(el('circle', { cx, cy, r: 34, fill: '#8b7cf6', opacity: 0.25, filter: 'url(#glow)' }));
      ring.appendChild(el('circle', { cx, cy, r: 9, fill: '#c4b9ff' }));
      svg.appendChild(ring);
    }
  }

  window.addEventListener('load', () => document.querySelectorAll('svg[data-graph]').forEach(draw));
})();
