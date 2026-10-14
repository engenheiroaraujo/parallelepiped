// Context column for the open note: a small map of its neighbourhood,
// backlinks, outgoing links and tags. Outgoing links and tags are read from
// the editor text, so they follow typing without waiting for the index.

import type { Backlink } from '../api/api';
import { colorOfNote, UNRESOLVED_COLOR } from '../ui/colors';
import { byId, h } from '../ui/dom';
import { dot } from './tree';

export interface OutgoingLink {
  title: string;
  /** Resolved note path, or null when the note does not exist yet. */
  path: string | null;
}

interface MapNode {
  title: string;
  path: string | null;
  color: string;
}

const SVG = 'http://www.w3.org/2000/svg';
const LABEL_CHARS = 16;

export class ContextView {
  private readonly map = byId<HTMLElement>('local-map') as unknown as SVGSVGElement;
  private readonly backlinkList = byId('backlinks');
  private readonly outgoingList = byId('outgoing');
  private readonly tagBox = byId('tags');
  private backlinks: Backlink[] = [];
  private outgoing: OutgoingLink[] = [];
  private title = '';
  private path = '';
  private colors = new Map<string, string>();

  constructor(
    private readonly onOpen: (path: string) => void,
    private readonly onFollow: (target: string) => void,
  ) {}

  /** Shows the context of a note: its path, text, backlinks and the vault's notes. */
  show(path: string, content: string, backlinks: Backlink[], notePaths: string[], colors: Map<string, string>): void {
    this.path = path;
    this.title = titleOf(path);
    this.backlinks = backlinks;
    this.colors = colors;
    this.updateText(content, notePaths);
    this.renderBacklinks();
  }

  /** Refreshes what depends on the note's text (outgoing links and tags). */
  updateText(content: string, notePaths: string[]): void {
    if (!this.path) return;
    this.outgoing = parseLinks(content).map((title) => ({ title, path: resolve(title, notePaths) }));
    this.renderOutgoing();
    this.renderTags(parseTags(content));
    this.renderMap();
  }

  clear(): void {
    this.path = '';
    this.backlinks = [];
    this.outgoing = [];
    this.map.replaceChildren();
    this.backlinkList.replaceChildren(empty('Abra uma nota para ver as ligações.'));
    this.outgoingList.replaceChildren();
    this.tagBox.replaceChildren();
    byId('backlinks-count').textContent = '';
    byId('outgoing-count').textContent = '';
  }

  private renderBacklinks(): void {
    byId('backlinks-count').textContent = String(this.backlinks.length);
    if (this.backlinks.length === 0) {
      this.backlinkList.replaceChildren(empty('Nenhuma nota aponta para esta.'));
      return;
    }
    this.backlinkList.replaceChildren(...this.backlinks.map((b) => this.item(b.title, b.path, () => this.onOpen(b.path))));
  }

  private renderOutgoing(): void {
    byId('outgoing-count').textContent = String(this.outgoing.length);
    if (this.outgoing.length === 0) {
      this.outgoingList.replaceChildren(empty('Escreva [[nome]] para ligar outra nota.'));
      return;
    }
    this.outgoingList.replaceChildren(
      ...this.outgoing.map((link) =>
        link.path
          ? this.item(link.title, link.path, () => this.onOpen(link.path!))
          : this.item(link.title, null, () => this.onFollow(link.title), 'nova'),
      ),
    );
  }

  private renderTags(tags: string[]): void {
    this.tagBox.replaceChildren(
      ...(tags.length ? tags.map((t) => h('span', { className: 'tag', textContent: `#${t}` })) : [empty('Sem tags.')]),
    );
  }

  private item(title: string, path: string | null, onClick: () => void, badge = ''): HTMLLIElement {
    const button = h(
      'button',
      { className: 'link-item', title: path ?? title },
      dot(path ? colorOfNote(this.colors, path) : UNRESOLVED_COLOR),
      h('span', { className: 'name', textContent: title }),
    );
    if (badge) button.append(h('span', { className: 'badge', textContent: badge }));
    button.addEventListener('click', onClick);
    return h('li', {}, button);
  }

  /** Radial map: backlinks on the left, outgoing links on the right. */
  private renderMap(): void {
    const width = this.map.clientWidth || 288;
    const height = this.map.clientHeight || 190;
    this.map.setAttribute('viewBox', `0 0 ${width} ${height}`);
    const cx = width / 2;
    const cy = height / 2 - 6;

    const incoming: MapNode[] = this.backlinks.map((b) => ({ title: b.title, path: b.path, color: colorOfNote(this.colors, b.path) }));
    const outgoing: MapNode[] = this.outgoing.map((l) => ({
      title: l.title,
      path: l.path,
      color: l.path ? colorOfNote(this.colors, l.path) : UNRESOLVED_COLOR,
    }));

    const children: SVGElement[] = [];
    const place = (nodes: MapNode[], side: -1 | 1) => {
      const shown = nodes.slice(0, 7);
      shown.forEach((node, i) => {
        const spread = shown.length === 1 ? 0 : (i / (shown.length - 1) - 0.5) * Math.PI * 0.85;
        const x = cx + side * Math.cos(spread) * (width * 0.36);
        const y = cy + Math.sin(spread) * (height * 0.4);
        children.unshift(svg('line', { x1: cx, y1: cy, x2: x, y2: y, stroke: node.color, 'stroke-opacity': 0.45, 'stroke-width': 1.2 }));
        const group = svg('g', { class: 'node' });
        group.append(svg('circle', { cx: x, cy: y, r: 6, fill: node.color }));
        const label = svg('text', { x, y: y + 17, 'text-anchor': 'middle' });
        label.textContent = truncate(node.title);
        group.append(label);
        group.addEventListener('click', () => (node.path ? this.onOpen(node.path) : this.onFollow(node.title)));
        children.push(group);
      });
    };
    place(incoming, -1);
    place(outgoing, 1);

    const centre = svg('g', {});
    centre.append(
      svg('circle', { cx, cy, r: 16, fill: colorOfNote(this.colors, this.path), opacity: 0.2 }),
      svg('circle', { cx, cy, r: 9, fill: colorOfNote(this.colors, this.path) }),
    );
    const label = svg('text', { x: cx, y: cy + 26, 'text-anchor': 'middle', class: 'center-label' });
    label.textContent = truncate(this.title);
    centre.append(label);
    children.push(centre);

    this.map.replaceChildren(...children);
  }
}

function svg(tag: string, attrs: Record<string, string | number>): SVGElement {
  const el = document.createElementNS(SVG, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, String(v));
  return el;
}

function empty(text: string): HTMLElement {
  return h('li', { className: 'empty', textContent: text });
}

function titleOf(path: string): string {
  return (path.split('/').pop() ?? path).replace(/\.md$/i, '');
}

function truncate(text: string): string {
  return text.length > LABEL_CHARS ? text.slice(0, LABEL_CHARS - 1) + '…' : text;
}

/** Lines outside fenced code blocks, with inline code removed. */
function proseLines(content: string): string[] {
  const lines: string[] = [];
  let fence = '';
  for (const line of content.split('\n')) {
    const trimmed = line.trim();
    if (fence) {
      if (trimmed.startsWith(fence)) fence = '';
      continue;
    }
    if (trimmed.startsWith('```') || trimmed.startsWith('~~~')) {
      fence = trimmed.slice(0, 3);
      continue;
    }
    lines.push(line.replace(/`[^`\n]*`/g, ''));
  }
  return lines;
}

/** Distinct [[link]] targets, in order of appearance (same rules as the index). */
export function parseLinks(content: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const line of proseLines(content)) {
    for (const match of line.matchAll(/!?\[\[([^\[\]|#^]+)(?:[#^][^\[\]|]*)?(?:\|[^\[\]]*)?\]\]/g)) {
      const target = match[1].trim();
      const key = target.toLowerCase();
      if (target && !seen.has(key)) {
        seen.add(key);
        out.push(target);
      }
    }
  }
  return out;
}

/** Distinct #tags, lower case; purely numeric tokens are not tags. */
export function parseTags(content: string): string[] {
  const seen = new Set<string>();
  for (const line of proseLines(content)) {
    for (const match of line.matchAll(/(?:^|[\s(])#([\p{L}\p{N}_\-/]+)/gu)) {
      const tag = match[1].replace(/^\/+|\/+$/g, '').toLowerCase();
      if (tag && !/^\d+$/.test(tag)) seen.add(tag);
    }
  }
  return [...seen];
}

/** Resolves a link target to a note path: full path first, then title. */
export function resolve(target: string, notePaths: string[]): string | null {
  const wanted = target.toLowerCase().replace(/\.md$/, '');
  return (
    notePaths.find((p) => p.toLowerCase().replace(/\.md$/, '') === wanted) ??
    notePaths.find((p) => titleOf(p).toLowerCase() === wanted) ??
    null
  );
}
