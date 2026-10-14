// File tree in the explorer. Folders show their colour (the same as in the
// graph) and how many notes they hold directly.

import type { TreeEntry } from '../api/api';
import { colorOfNote, folderColors, folderOf } from '../ui/colors';
import { byId, h } from '../ui/dom';

export class TreeView {
  private readonly root = byId('tree');
  private readonly collapsed = new Set<string>();
  private active = '';
  private entry: TreeEntry | null = null;
  private paths: string[] = [];
  private colorMap = new Map<string, string>();

  constructor(private readonly onOpen: (path: string) => void) {}

  render(entry: TreeEntry): void {
    this.entry = entry;
    this.paths = [];
    const walk = (e: TreeEntry) => {
      if (!e.is_dir) this.paths.push(e.path);
      (e.children ?? []).forEach(walk);
    };
    walk(entry);
    this.colorMap = folderColors(this.paths.map(folderOf));
    this.redraw();
  }

  setActive(path: string): void {
    this.active = path;
    this.redraw();
  }

  /** All note paths in the tree, for link autocompletion and resolution. */
  notePaths(): string[] {
    return this.paths;
  }

  /** Folder colours, shared with the context panel. */
  colors(): Map<string, string> {
    return this.colorMap;
  }

  /** Number of notes and of folders holding notes. */
  stats(): { notes: number; folders: number } {
    return { notes: this.paths.length, folders: this.colorMap.size };
  }

  private redraw(): void {
    if (this.entry) this.root.replaceChildren(this.list(this.entry.children ?? []));
  }

  private list(entries: TreeEntry[]): HTMLUListElement {
    const ul = h('ul');
    for (const entry of entries) ul.append(entry.is_dir ? this.folder(entry) : this.file(entry));
    return ul;
  }

  private folder(entry: TreeEntry): HTMLLIElement {
    const open = !this.collapsed.has(entry.path);
    const notes = (entry.children ?? []).filter((c) => !c.is_dir).length;
    const color = this.colorMap.get(entry.path);

    const label = h(
      'button',
      { className: 'tree-item folder', title: entry.path },
      h('span', { className: 'chevron', textContent: open ? '▾' : '▸' }),
      color ? dot(color) : h('span', { className: 'dot' }),
      h('span', { className: 'name', textContent: entry.name }),
    );
    if (notes > 0) label.append(h('span', { className: 'count', textContent: String(notes) }));
    label.addEventListener('click', () => {
      if (open) this.collapsed.add(entry.path);
      else this.collapsed.delete(entry.path);
      this.redraw();
    });

    const li = h('li', {}, label);
    if (open) li.append(this.list(entry.children ?? []));
    return li;
  }

  private file(entry: TreeEntry): HTMLLIElement {
    const button = h(
      'button',
      { className: 'tree-item file' + (entry.path === this.active ? ' active' : ''), title: entry.path },
      h('span', { className: 'name', textContent: entry.name.replace(/\.md$/i, '') }),
    );
    button.style.setProperty('--marker', colorOfNote(this.colorMap, entry.path));
    button.addEventListener('click', () => this.onOpen(entry.path));
    return h('li', {}, button);
  }
}

export function dot(color: string): HTMLSpanElement {
  const span = h('span', { className: 'dot' });
  span.style.background = color;
  return span;
}
