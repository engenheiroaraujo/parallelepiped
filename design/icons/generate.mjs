// Generates the app icon candidates as SVG: granite blocks (a
// "parallelepiped") drawn in isometric projection, with the stone's speckles
// placed dot by dot so the result stays a crisp vector at any size.
//
// Usage: node design/icons/generate.mjs   (writes option-*.svg + preview.html)

import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const OUT = dirname(fileURLToPath(import.meta.url));
const SIZE = 1024;

/** Small deterministic PRNG (mulberry32). */
function rng(seed) {
  let s = seed >>> 0;
  return () => {
    s = (s + 0x6d2b79f5) | 0;
    let t = Math.imul(s ^ (s >>> 15), 1 | s);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const f = (n) => Math.round(n * 10) / 10;
const pts = (list) => list.map(([x, y]) => `${f(x)},${f(y)}`).join(' ');

/** Isometric projection around an origin, `s` pixels per unit. */
function projector(ox, oy, s) {
  const c = Math.cos(Math.PI / 6);
  return (x, y, z) => [ox + (x - y) * c * s, oy + (x + y) * 0.5 * s - z * s];
}

/** Adds small random bumps along each edge so the stone looks hand-cut. */
function rough(poly, random, amount) {
  const out = [];
  for (let i = 0; i < poly.length; i++) {
    const [x1, y1] = poly[i];
    const [x2, y2] = poly[(i + 1) % poly.length];
    const steps = 5;
    for (let k = 0; k < steps; k++) {
      const t = k / steps;
      const jitter = k === 0 ? amount * 0.35 : amount;
      out.push([x1 + (x2 - x1) * t + (random() - 0.5) * jitter, y1 + (y2 - y1) * t + (random() - 0.5) * jitter]);
    }
  }
  return out;
}

function bbox(poly) {
  const xs = poly.map((p) => p[0]);
  const ys = poly.map((p) => p[1]);
  return [Math.min(...xs), Math.min(...ys), Math.max(...xs), Math.max(...ys)];
}

let clipId = 0;

/** Granite speckles clipped to a face. */
// Fine, dense grains in several greys, like real salt-and-pepper granite.
const GRANITE_GREYS = ['#161616', '#262626', '#383838', '#4d4d4d', '#6a6a6a', '#8a8a88'];

function speckles(poly, random, { density = 0.011, dark = GRANITE_GREYS, light = null, scale = 1 } = {}) {
  const id = `c${clipId++}`;
  const [x0, y0, x1, y1] = bbox(poly);
  const count = Math.round((x1 - x0) * (y1 - y0) * density);
  let dots = '';
  for (let i = 0; i < count; i++) {
    const x = x0 + random() * (x1 - x0);
    const y = y0 + random() * (y1 - y0);
    const r = (0.7 + random() ** 2.6 * 4.6) * scale;
    const color = light && random() < 0.18 ? light : dark[Math.floor(random() * dark.length)];
    // Irregular grains: squashed ellipses at random angles.
    dots += `<ellipse cx="${f(x)}" cy="${f(y)}" rx="${f(r)}" ry="${f(r * (0.45 + random() * 0.5))}" transform="rotate(${Math.round(random() * 180)} ${f(x)} ${f(y)})" fill="${color}"/>`;
  }
  return { def: `<clipPath id="${id}"><polygon points="${pts(poly)}"/></clipPath>`, body: `<g clip-path="url(#${id})">${dots}</g>` };
}

/**
 * A granite block W×D×H (units) whose bottom-front corner sits at (x, y, 0).
 * Returns SVG defs and body for its three visible faces.
 */
function block(project, { x = 0, y = 0, w, d, h }, seed, opts = {}) {
  const random = rng(seed);
  const P = (a, b, c) => project(x + a, y + b, c);
  const top = [P(0, 0, h), P(w, 0, h), P(w, d, h), P(0, d, h)];
  const right = [P(w, 0, 0), P(w, d, 0), P(w, d, h), P(w, 0, h)];
  const left = [P(0, d, 0), P(w, d, 0), P(w, d, h), P(0, d, h)];
  const roughness = opts.roughness ?? 14;
  const faces = [
    { poly: rough(left, random, roughness), fill: opts.left ?? '#b9b9b5' },
    { poly: rough(right, random, roughness), fill: opts.right ?? '#d3d3cf' },
    { poly: rough(top, random, roughness), fill: opts.top ?? '#ececea' },
  ];
  let defs = '';
  let body = '';
  for (const face of faces) {
    const s = speckles(face.poly, random, opts.speckles);
    defs += s.def;
    body += `<polygon points="${pts(face.poly)}" fill="${face.fill}"/>${s.body}`;
  }
  // A faint dark outline keeps light stone readable on light backgrounds.
  for (const face of faces) {
    body += `<polygon points="${pts(face.poly)}" fill="none" stroke="#000" stroke-opacity="0.22" stroke-width="3" stroke-linejoin="round"/>`;
  }
  // Light catches the top edges.
  body += `<polyline points="${pts([top[3], top[0], top[1]])}" fill="none" stroke="#ffffff" stroke-opacity="0.55" stroke-width="5" stroke-linejoin="round"/>`;
  // Edge between the two side faces.
  body += `<line x1="${f(right[1][0])}" y1="${f(right[1][1])}" x2="${f(right[2][0])}" y2="${f(right[2][1])}" stroke="#000" stroke-opacity="0.12" stroke-width="4"/>`;
  return { defs, body, top, right, left };
}

function shadow(project, { x = 0, y = 0, w, d }, spread = 0.18, opacity = 0.28) {
  const P = (a, b) => project(x + a + spread * w, y + b + spread * d, 0);
  const poly = [project(x, y, 0), P(w, -0.05 * d), P(w + 0.1 * w, d + 0.1 * d), P(-0.05 * w, d)];
  return `<polygon points="${pts(poly)}" fill="#000" opacity="${opacity}"/>`;
}

/**
 * Scales and centres a drawing so its blocks fill `target` pixels of the
 * icon, slightly below the middle (room for the shadow underneath).
 */
function framed(body, polys, target = 760, centerY = 500) {
  const [x0, y0, x1, y1] = bbox(polys.flat());
  const k = target / Math.max(x1 - x0, y1 - y0);
  const tx = SIZE / 2 - ((x0 + x1) / 2) * k;
  const ty = centerY - ((y0 + y1) / 2) * k;
  return `<g transform="translate(${f(tx)} ${f(ty)}) scale(${Math.round(k * 1000) / 1000})">${body}</g>`;
}

function svg(background, defs, body) {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${SIZE} ${SIZE}" width="${SIZE}" height="${SIZE}">
<defs><clipPath id="tile"><rect width="${SIZE}" height="${SIZE}" rx="224"/></clipPath>${defs}</defs>
<g clip-path="url(#tile)">${background}${body}</g>
</svg>
`;
}

/** An icon without a background tile: only the drawing, on transparency. */
function transparentSvg(defs, body) {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${SIZE} ${SIZE}" width="${SIZE}" height="${SIZE}">
<defs>${defs}</defs>
${body}
</svg>
`;
}

const options = [];

const origin = projector(0, 0, 1);
let chosen = null;
const faces = (b) => [b.top, b.left, b.right];
const darkTile = `<rect width="${SIZE}" height="${SIZE}" fill="#1e1e1e"/><circle cx="512" cy="420" r="460" fill="#2b2b30" opacity="0.7"/>`;

// 1. Classic: one rough granite cube on the app's dark background.
{
  clipId = 0;
  const dims = { x: -170, y: -170, w: 340, d: 340, h: 330 };
  const b = block(origin, dims, 11);
  const body = framed(shadow(origin, dims, 0.22, 0.35) + b.body, faces(b));
  options.push({ name: 'Bloco clássico', file: 'option-1.svg', content: svg(darkTile, b.defs, body) });
}

// 2. Trio: three blocks of different sizes, like the photo, on concrete grey.
{
  clipId = 0;
  const back = { x: -260, y: -300, w: 300, d: 270, h: 330 };
  const front = { x: -10, y: -90, w: 300, d: 290, h: 240 };
  const small = { x: -300, y: 40, w: 190, d: 190, h: 190 };
  const parts = [back, front, small].map((dims, i) => ({ dims, b: block(origin, dims, 21 + i, { roughness: 12 }) }));
  // Painter's order: farthest first.
  const order = [parts[0], parts[2], parts[1]];
  const bg = `<rect width="${SIZE}" height="${SIZE}" fill="#c4c5c3"/><circle cx="512" cy="380" r="520" fill="#d9dad8" opacity="0.7"/>`;
  const body = framed(
    order.map((p) => shadow(origin, p.dims, 0.2, 0.18) + p.b.body).join(''),
    order.flatMap((p) => faces(p.b)),
    800,
  );
  options.push({ name: 'Trio de pedras', file: 'option-2.svg', content: svg(bg, order.map((p) => p.b.defs).join(''), body) });

  // The chosen app icon: the same trio with no background tile. Without the
  // tile the stones can fill more of the canvas; the soft shadows stay so the
  // stones sit on something on any background.
  const bare = framed(
    order.map((p) => shadow(origin, p.dims, 0.2, 0.22) + p.b.body).join(''),
    order.flatMap((p) => faces(p.b)),
    // Framing counts only the stones; leave room below for their shadows.
    860,
    SIZE / 2 - 42,
  );
  chosen = { name: 'Trio de pedras (transparente)', file: 'app-icon.svg', content: transparentSvg(order.map((p) => p.b.defs).join(''), bare) };
}

// 3. Parallelepiped: a tall stone prism with glowing neural traces on top.
{
  clipId = 0;
  const dims = { x: -130, y: -130, w: 260, d: 260, h: 420 };
  const b = block(origin, dims, 31, { roughness: 10 });
  const random = rng(77);
  const t = b.top;
  const onTop = (u, v) => {
    const a = [t[0][0] + (t[1][0] - t[0][0]) * u, t[0][1] + (t[1][1] - t[0][1]) * u];
    const c = [t[3][0] + (t[2][0] - t[3][0]) * u, t[3][1] + (t[2][1] - t[3][1]) * u];
    return [a[0] + (c[0] - a[0]) * v, a[1] + (c[1] - a[1]) * v];
  };
  const nodes = [[0.22, 0.28], [0.62, 0.2], [0.78, 0.62], [0.32, 0.72]].map(([u, v]) => onTop(u, v));
  const edges = [[0, 1], [1, 2], [2, 3], [3, 0], [0, 2]];
  let neural = '';
  for (const [a, c] of edges) neural += `<line x1="${f(nodes[a][0])}" y1="${f(nodes[a][1])}" x2="${f(nodes[c][0])}" y2="${f(nodes[c][1])}" stroke="#8b7cf6" stroke-width="9" stroke-linecap="round"/>`;
  for (const [x, y] of nodes) neural += `<circle cx="${f(x)}" cy="${f(y)}" r="34" fill="#8b7cf6" opacity="0.3"/><circle cx="${f(x)}" cy="${f(y)}" r="${f(17 + random() * 4)}" fill="#b4a9ff"/>`;
  const bg = `<rect width="${SIZE}" height="${SIZE}" fill="#17161c"/><circle cx="512" cy="420" r="460" fill="#24222e" opacity="0.8"/>`;
  const body = framed(shadow(origin, dims, 0.22, 0.4) + b.body + neural, faces(b));
  options.push({ name: 'Paralelepípedo neural', file: 'option-3.svg', content: svg(bg, b.defs, body) });
}

// 4. Coloured granite: some grains take the graph's folder colours.
{
  clipId = 0;
  const dims = { x: -170, y: -170, w: 340, d: 340, h: 330 };
  const b = block(origin, dims, 41, {
    speckles: { dark: [...GRANITE_GREYS, ...GRANITE_GREYS, '#a855f7', '#10b981', '#ef4444', '#3b82f6', '#eab308'], density: 0.01 },
  });
  const body = framed(shadow(origin, dims, 0.22, 0.35) + b.body, faces(b));
  options.push({ name: 'Granito colorido', file: 'option-4.svg', content: svg(darkTile, b.defs, body) });
}

// 5. Minimal: clean-edged dark granite block on the app's purple.
{
  clipId = 0;
  const dims = { x: -175, y: -175, w: 350, d: 350, h: 300 };
  const b = block(origin, dims, 51, {
    roughness: 0,
    top: '#5a5a62',
    right: '#46464d',
    left: '#34343a',
    speckles: { dark: ['#1b1b1f', '#26262b', '#303036'], light: '#d9d9de', density: 0.012, scale: 0.85 },
  });
  const bg = `<rect width="${SIZE}" height="${SIZE}" fill="#8b7cf6"/><circle cx="512" cy="420" r="440" fill="#a597ff" opacity="0.35"/>`;
  const body = framed(shadow(origin, dims, 0.2, 0.25) + b.body, faces(b), 720);
  options.push({ name: 'Minimalista', file: 'option-5.svg', content: svg(bg, b.defs, body) });
}

for (const o of options) writeFileSync(join(OUT, o.file), o.content);

const cards = options
  .map(
    (o, i) => `<figure>
  <img src="${o.file}" width="220" height="220" alt="${o.name}">
  <figcaption><strong>${i + 1}. ${o.name}</strong></figcaption>
  <div class="small"><img src="${o.file}" width="64" height="64" alt=""><img src="${o.file}" width="32" height="32" alt=""><img src="${o.file}" width="16" height="16" alt=""></div>
</figure>`,
  )
  .join('\n');

writeFileSync(
  join(OUT, 'preview.html'),
  `<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Ícones do parallelepiped</title>
<style>
body { margin: 0; padding: 32px; background: #2a2a2e; color: #e4e4e7; font-family: 'Segoe UI', system-ui, sans-serif; }
h1 { font-weight: 500; font-size: 22px; margin: 0 0 24px; }
main { display: flex; flex-wrap: wrap; gap: 28px; }
figure { margin: 0; padding: 20px; background: #1e1e1e; border-radius: 16px; text-align: center; }
figcaption { margin: 12px 0 10px; }
.small { display: flex; gap: 14px; align-items: end; justify-content: center; }
</style></head>
<body><h1>Modelos de ícone — parallelepiped</h1><main>
${cards}
</main></body></html>
`,
);

writeFileSync(join(OUT, chosen.file), chosen.content);

// The chosen icon on the backgrounds it will meet: transparency checker,
// the app/taskbar dark and a light desktop.
const backgrounds = [
  ['Transparência', 'background: repeating-conic-gradient(#3a3a3e 0% 25%, #2a2a2e 0% 50%) 50% / 24px 24px;'],
  ['Escuro (app / barra de tarefas)', 'background: #1e1e1e;'],
  ['Claro', 'background: #f3f3f3; color: #1e1e1e;'],
];
const rows = backgrounds
  .map(
    ([label, style]) => `<section style="${style}">
  <h2>${label}</h2>
  <div class="sizes">${[256, 64, 32, 16].map((px) => `<figure><img src="${chosen.file}" width="${px}" height="${px}" alt=""><figcaption>${px}px</figcaption></figure>`).join('')}</div>
</section>`,
  )
  .join('\n');

writeFileSync(
  join(OUT, 'app-icon-preview.html'),
  `<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Ícone do parallelepiped</title>
<style>
body { margin: 0; padding: 24px; background: #2a2a2e; color: #e4e4e7; font-family: 'Segoe UI', system-ui, sans-serif; }
h1 { font-weight: 500; font-size: 22px; margin: 0 0 18px; }
h2 { font-weight: 500; font-size: 15px; margin: 0 0 14px; }
main { display: flex; gap: 20px; }
section { flex: 1; padding: 20px; border-radius: 16px; color: #e4e4e7; }
.sizes { display: flex; gap: 18px; align-items: end; }
figure { margin: 0; text-align: center; }
figcaption { font-size: 12px; opacity: 0.7; margin-top: 6px; }
</style></head>
<body><h1>${chosen.name}</h1><main>
${rows}
</main></body></html>
`,
);

console.log(`wrote ${options.length} icons, ${chosen.file} and the previews to ${OUT}`);
