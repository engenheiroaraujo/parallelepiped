// File tree in the sidebar.

import type { TreeEntry } from '../api/api';
import { byId, h } from '../ui/dom';

export class TreeView {
  private readonly root = byId('tree');
  private readonly collapsed = new Set<string>();
  private active = '';
  private entry: TreeEntry | null = null;

  constructor(private readonly onOpen: (path: string) => void) {}

  render(entry: TreeEntry): void {
    this.entry = entry;
    this.root.replaceChildren(this.list(entry.children ?? []));
  }

  setActive(path: string): void {
    this.active = path;
    if (this.entry) this.render(this.entry);
  }

  /** All note paths in the tree, for link autocompletion. */
  notePaths(): string[] {
    const out: string[] = [];
    const walk = (e: TreeEntry) => {
      if (!e.is_dir) out.push(e.path);
      (e.children ?? []).forEach(walk);
    };
    if (this.entry) walk(this.entry);
    return out;
  }

  private list(entries: TreeEntry[]): HTMLUListElement {
    const ul = h('ul');
    for (const entry of entries) {
      ul.append(entry.is_dir ? this.folder(entry) : this.file(entry));
    }
    return ul;
  }

  private folder(entry: TreeEntry): HTMLLIElement {
    const open = !this.collapsed.has(entry.path);
    const label = h('button', { className: 'tree-item folder', textContent: `${open ? '▾' : '▸'} ${entry.name}` });
    label.addEventListener('click', () => {
      if (open) this.collapsed.add(entry.path);
      else this.collapsed.delete(entry.path);
      if (this.entry) this.render(this.entry);
    });
    const li = h('li', {}, label);
    if (open) li.append(this.list(entry.children ?? []));
    return li;
  }

  private file(entry: TreeEntry): HTMLLIElement {
    const button = h('button', {
      className: 'tree-item file' + (entry.path === this.active ? ' active' : ''),
      textContent: entry.name.replace(/\.md$/i, ''),
      title: entry.path,
    });
    button.addEventListener('click', () => this.onOpen(entry.path));
    return h('li', {}, button);
  }
}
