// Tabs of open notes above the editor.

import { colorOfNote } from '../ui/colors';
import { byId, h } from '../ui/dom';
import { dot } from './tree';

const MAX_TABS = 8;

export class TabsView {
  private readonly root = byId('tabs');
  private paths: string[] = [];
  private active = '';
  private colors = new Map<string, string>();

  constructor(
    private readonly onSelect: (path: string) => void,
    private readonly onClose: (path: string) => void,
  ) {}

  /** Shows `path` as the active tab, adding it when new. */
  open(path: string): void {
    if (!this.paths.includes(path)) {
      this.paths.push(path);
      // Drop the oldest inactive tab beyond the limit.
      while (this.paths.length > MAX_TABS) {
        const oldest = this.paths.findIndex((p) => p !== path);
        this.paths.splice(oldest, 1);
      }
    }
    this.active = path;
    this.render();
  }

  /** Removes a tab and returns the path that should become active ("" if none). */
  close(path: string): string {
    const at = this.paths.indexOf(path);
    if (at < 0) return this.active;
    this.paths.splice(at, 1);
    if (this.active === path) this.active = this.paths[Math.min(at, this.paths.length - 1)] ?? '';
    this.render();
    return this.active;
  }

  rename(from: string, to: string): void {
    this.paths = this.paths.map((p) => (p === from ? to : p));
    if (this.active === from) this.active = to;
    this.render();
  }

  /** Drops tabs whose notes no longer exist. */
  retain(existing: Set<string>): void {
    this.paths = this.paths.filter((p) => existing.has(p));
    if (!existing.has(this.active)) this.active = '';
    this.render();
  }

  clear(): void {
    this.paths = [];
    this.active = '';
    this.render();
  }

  setColors(colors: Map<string, string>): void {
    this.colors = colors;
    this.render();
  }

  private render(): void {
    this.root.replaceChildren(
      ...this.paths.map((path) => {
        const title = (path.split('/').pop() ?? path).replace(/\.md$/i, '');
        const close = h('span', { className: 'tab-close', title: 'Fechar', textContent: '×' });
        close.setAttribute('role', 'button');
        close.addEventListener('click', (event) => {
          event.stopPropagation();
          this.onClose(path);
        });
        const tab = h(
          'button',
          { className: 'tab' + (path === this.active ? ' active' : ''), title: path },
          dot(colorOfNote(this.colors, path)),
          h('span', { className: 'tab-title', textContent: title }),
          close,
        );
        tab.setAttribute('role', 'tab');
        tab.addEventListener('click', () => this.onSelect(path));
        tab.addEventListener('auxclick', (event) => {
          if (event.button === 1) this.onClose(path);
        });
        return tab;
      }),
    );
  }
}
