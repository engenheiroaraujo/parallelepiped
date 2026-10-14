// Backlinks panel and search results.

import type { Backlink, SearchHit } from '../api/api';
import { byId, h } from '../ui/dom';

const HIGHLIGHT_START = '\x02';
const HIGHLIGHT_END = '\x03';

export class BacklinksView {
  private readonly list = byId('backlinks');

  constructor(private readonly onOpen: (path: string) => void) {}

  render(items: Backlink[]): void {
    if (items.length === 0) {
      this.list.replaceChildren(h('li', { className: 'empty', textContent: 'Nenhuma nota aponta para esta.' }));
      return;
    }
    this.list.replaceChildren(
      ...items.map((item) => {
        const button = h('button', { className: 'link-item', textContent: item.title, title: item.path });
        button.addEventListener('click', () => this.onOpen(item.path));
        return h('li', {}, button);
      }),
    );
  }

  clear(): void {
    this.list.replaceChildren();
  }
}

export class SearchView {
  private readonly results = byId('search-results');

  constructor(private readonly onOpen: (path: string) => void) {}

  render(hits: SearchHit[]): void {
    this.results.hidden = false;
    if (hits.length === 0) {
      this.results.replaceChildren(h('p', { className: 'empty', textContent: 'Nada encontrado.' }));
      return;
    }
    this.results.replaceChildren(
      ...hits.map((hit) => {
        const button = h('button', { className: 'search-hit' }, h('strong', { textContent: hit.title }), snippet(hit.snippet));
        button.addEventListener('click', () => this.onOpen(hit.path));
        return button;
      }),
    );
  }

  hide(): void {
    this.results.hidden = true;
    this.results.replaceChildren();
  }
}

/** Builds the snippet with <mark> nodes, never through innerHTML. */
function snippet(text: string): HTMLElement {
  const out = h('span', { className: 'snippet' });
  const parts = text.split(HIGHLIGHT_START);
  out.append(parts[0]);
  for (const part of parts.slice(1)) {
    const [marked, rest = ''] = part.split(HIGHLIGHT_END);
    out.append(h('mark', { textContent: marked }), rest);
  }
  return out;
}
