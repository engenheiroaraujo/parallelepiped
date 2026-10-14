// Application controller: wires the views to the backend calls.

import './style.css';
import { brainApi, brainEvents, indexApi, notesApi, vaultApi, vaultEvents, type Backlink, type ImportResult, type VaultInfo } from './api/api';
import { AppError, describeError } from './api/client';
import { folderOf } from './ui/colors';
import { askText, byId, confirmAction, debounce, h, toast } from './ui/dom';
import { initTheme, toggleTheme } from './ui/theme';
import { ContextView, resolve } from './views/context';
import { Editor } from './views/editor';
import { GraphView } from './views/graph';
import { SearchView } from './views/panels';
import { TabsView } from './views/tabs';
import { TreeView } from './views/tree';

type ViewName = 'editor' | 'graph';

const SAVE_DELAY_MS = 600;
const CONTEXT_DELAY_MS = 400;
const GRAPH_FOCUS_KEY = 'parallelepiped.graphFocus';

class App {
  private vault: VaultInfo | null = null;
  private currentPath = '';
  private dirtyContent: string | null = null;
  private currentBacklinks: Backlink[] = [];
  /** Serialises handling of outside changes so refreshes never interleave. */
  private outsideChanges: Promise<void> = Promise.resolve();

  private readonly tree = new TreeView((path) => this.openNote(path));
  private readonly tabs = new TabsView(
    (path) => this.openNote(path),
    (path) => void this.closeTab(path),
  );
  private readonly context = new ContextView(
    (path) => this.openNote(path),
    (target) => this.followLink(target),
  );
  private readonly search = new SearchView((path) => this.openNote(path));
  private readonly graph = new GraphView(byId('graph'), (node) =>
    node.unresolved ? this.followLink(node.title) : this.openNote(node.id),
  );
  private readonly editor = new Editor(byId('editor'), {
    onChange: (content) => this.onEdit(content),
    onFollowLink: (target) => this.followLink(target),
    linkTargets: () => this.tree.notePaths(),
  });

  private readonly saveSoon = debounce(() => void this.flush(), SAVE_DELAY_MS);
  private readonly searchSoon = debounce((query: string) => void this.runSearch(query), 200);
  private readonly contextSoon = debounce((content: string) => this.context.updateText(content, this.tree.notePaths()), CONTEXT_DELAY_MS);

  async start(): Promise<void> {
    initTheme();
    this.bindControls();
    vaultEvents.onChanged((paths) => {
      this.outsideChanges = this.outsideChanges.then(() => this.onVaultChanged(paths));
    });
    const recent = await vaultApi.recent().catch(() => []);
    if (recent.length > 0) {
      try {
        await this.useVault(await vaultApi.open(recent[0].path));
        return;
      } catch {
        // The last vault may have been moved or deleted; fall back to the welcome screen.
      }
    }
    this.showWelcome(recent);
  }

  private bindControls(): void {
    byId('open-vault').addEventListener('click', () => void this.chooseVault());
    byId('welcome-open').addEventListener('click', () => void this.chooseVault());
    byId('new-note').addEventListener('click', () => void this.createNote());
    byId('import-project').addEventListener('click', () => void this.importProject());
    byId('delete-note').addEventListener('click', () => void this.deleteNote());
    byId('rail-notes').addEventListener('click', () => this.showView('editor'));
    byId('rail-graph').addEventListener('click', () => this.showView('graph'));
    byId('open-graph').addEventListener('click', () => this.showView('graph'));
    byId('theme-toggle').addEventListener('click', () => toggleTheme());
    byId('graph-focus').addEventListener('click', () => this.setGraphFocus(!this.graphFocus()));

    const title = byId<HTMLInputElement>('note-title');
    title.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') title.blur();
      if (e.key === 'Escape') {
        title.value = titleOf(this.currentPath);
        title.blur();
      }
    });
    title.addEventListener('change', () => void this.renameNote(title.value));

    const searchInput = byId<HTMLInputElement>('search');
    searchInput.addEventListener('input', () => this.searchSoon(searchInput.value));
    searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        searchInput.value = '';
        void this.runSearch('');
        searchInput.blur();
      }
    });

    window.addEventListener('keydown', (e) => {
      // Graph-only mode: F toggles it, Esc leaves it (not while typing).
      if (!byId('graph-view').hidden && !e.ctrlKey && !e.metaKey && !e.altKey && !isTyping(e.target)) {
        if (e.key.toLowerCase() === 'f') {
          e.preventDefault();
          this.setGraphFocus(!this.graphFocus());
          return;
        }
        if (e.key === 'Escape' && this.graphFocus()) {
          this.setGraphFocus(false);
          return;
        }
      }
      if (!(e.ctrlKey || e.metaKey)) return;
      const key = e.key.toLowerCase();
      if (key === 'n') {
        e.preventDefault();
        void this.createNote();
      } else if (key === 'g') {
        e.preventDefault();
        this.showView(byId('graph-view').hidden ? 'graph' : 'editor');
      } else if (key === 'k' || (key === 'f' && e.shiftKey)) {
        e.preventDefault();
        this.focusSearch();
      } else if (key === 's') {
        e.preventDefault();
        void this.flush();
      } else if (key === 'w' && this.currentPath) {
        e.preventDefault();
        void this.closeTab(this.currentPath);
      }
    });
    window.addEventListener('beforeunload', () => void this.flush());
  }

  private focusSearch(): void {
    this.showView('editor');
    const input = byId<HTMLInputElement>('search');
    input.focus();
    input.select();
  }

  private async chooseVault(): Promise<void> {
    try {
      const info = await vaultApi.choose();
      if (info.path) await this.useVault(info);
    } catch (err) {
      toast(describeError(err));
    }
  }

  private async useVault(info: VaultInfo): Promise<void> {
    await this.flush();
    this.vault = info;
    this.currentPath = '';
    byId('vault-name').textContent = info.name;
    byId('vault-name').title = info.path;
    byId('welcome').hidden = true;
    byId('workspace').hidden = false;
    this.tabs.clear();
    await this.refreshTree();
    this.showEmptyNote();
    this.showView('editor');
  }

  private showWelcome(recent: VaultInfo[]): void {
    byId('workspace').hidden = true;
    byId('welcome').hidden = false;
    const list = byId('recent-vaults');
    list.replaceChildren(
      ...recent.map((v) => {
        const button = h('button', { className: 'link-item', title: v.path }, h('span', { className: 'name', textContent: v.name }));
        button.addEventListener('click', () => vaultApi.open(v.path).then((info) => this.useVault(info)).catch((err) => toast(describeError(err))));
        return h('li', {}, button);
      }),
    );
  }

  private async refreshTree(): Promise<void> {
    this.tree.render(await vaultApi.tree());
    const { notes, folders } = this.tree.stats();
    byId('vault-stats').textContent = `${notes} ${notes === 1 ? 'nota' : 'notas'} · ${folders} ${folders === 1 ? 'pasta' : 'pastas'}`;
    this.tabs.setColors(this.tree.colors());
    this.tabs.retain(new Set(this.tree.notePaths()));
  }

  async openNote(path: string): Promise<void> {
    await this.flush();
    try {
      const note = await notesApi.read(path);
      this.currentPath = note.path;
      this.editor.load(note.content);
      this.tree.setActive(note.path);
      this.tabs.open(note.path);
      this.showNoteBar(note.path, note.content);
      byId('editor-empty').hidden = true;
      this.setStatus('');
      this.showView('editor');
      this.editor.focus();
      this.currentBacklinks = await indexApi.backlinks(note.path);
      this.context.show(note.path, note.content, this.currentBacklinks, this.tree.notePaths(), this.tree.colors());
    } catch (err) {
      toast(describeError(err));
    }
  }

  private showNoteBar(path: string, content: string): void {
    const folder = folderOf(path);
    byId('note-folder').textContent = folder ? `${folder} /` : '';
    byId<HTMLInputElement>('note-title').value = titleOf(path);
    this.setWordCount(content);
    byId('note-bar').hidden = false;
  }

  private setWordCount(content: string): void {
    const words = content.trim() ? content.trim().split(/\s+/).length : 0;
    byId('word-count').textContent = `${words} ${words === 1 ? 'palavra' : 'palavras'}`;
  }

  private showEmptyNote(): void {
    this.currentPath = '';
    this.editor.load('');
    this.tree.setActive('');
    this.context.clear();
    byId('note-bar').hidden = true;
    byId('editor-empty').hidden = false;
  }

  private async closeTab(path: string): Promise<void> {
    if (path === this.currentPath) await this.flush();
    const next = this.tabs.close(path);
    if (path !== this.currentPath) return;
    if (next) await this.openNote(next);
    else this.showEmptyNote();
  }

  /** Opens the note a [[link]] points to, creating it when missing. */
  private async followLink(target: string): Promise<void> {
    const match = resolve(target, this.tree.notePaths());
    if (match) return this.openNote(match);

    try {
      const note = await notesApi.create(target);
      await this.refreshTree();
      await this.openNote(note.path);
    } catch (err) {
      toast(describeError(err));
    }
  }

  private onEdit(content: string): void {
    if (!this.currentPath) return;
    this.dirtyContent = content;
    this.setStatus('Editando…');
    this.setWordCount(content);
    this.saveSoon();
    this.contextSoon(content);
  }

  private async flush(): Promise<void> {
    this.saveSoon.cancel();
    if (this.dirtyContent === null || !this.currentPath) return;
    const content = this.dirtyContent;
    this.dirtyContent = null;
    try {
      await notesApi.save(this.currentPath, content);
      this.setStatus('Salvo');
    } catch (err) {
      this.dirtyContent ??= content;
      this.setStatus('Erro ao salvar');
      toast(describeError(err));
    }
  }

  /** Notes changed on disk outside the app (another editor, a sync client). */
  private async onVaultChanged(paths: string[]): Promise<void> {
    if (!this.vault) return;
    try {
      await this.refreshTree();
      const open = this.currentPath;
      if (open && paths.some((p) => covers(p, open))) await this.reloadOpenNote(open);
      if (this.currentPath) {
        this.currentBacklinks = await indexApi.backlinks(this.currentPath);
        this.context.show(this.currentPath, this.editor.content(), this.currentBacklinks, this.tree.notePaths(), this.tree.colors());
      }
      if (!byId('graph-view').hidden) this.graph.setData(await indexApi.graph());
    } catch (err) {
      toast(describeError(err));
    }
  }

  /** Reloads the open note from disk unless the user has unsaved edits. */
  private async reloadOpenNote(path: string): Promise<void> {
    if (this.dirtyContent !== null) return;
    try {
      const note = await notesApi.read(path);
      // The user may have switched notes or typed while the read ran.
      if (this.currentPath !== path || this.dirtyContent !== null) return;
      if (note.content !== this.editor.content()) {
        this.editor.load(note.content);
        this.setWordCount(note.content);
      }
    } catch (err) {
      if (!(err instanceof AppError && err.code === 'not_found')) throw err;
      if (this.currentPath !== path || this.dirtyContent !== null) return;
      this.tabs.close(path);
      this.showEmptyNote();
      toast('A nota aberta foi movida ou apagada fora do aplicativo.');
    }
  }

  private async createNote(): Promise<void> {
    if (!this.vault) return;
    const name = await askText('Nome da nova nota');
    if (!name) return;
    try {
      const note = await notesApi.create(name);
      await this.refreshTree();
      await this.openNote(note.path);
    } catch (err) {
      toast(describeError(err));
    }
  }

  private async renameNote(newTitle: string): Promise<void> {
    const title = newTitle.trim();
    if (!this.currentPath || !title || title === titleOf(this.currentPath)) return;
    await this.flush();
    const folder = folderOf(this.currentPath);
    try {
      const old = this.currentPath;
      const note = await notesApi.rename(old, folder ? `${folder}/${title}` : title);
      this.currentPath = note.path;
      this.tabs.rename(old, note.path);
      await this.refreshTree();
      this.tree.setActive(note.path);
      this.showNoteBar(note.path, this.editor.content());
    } catch (err) {
      byId<HTMLInputElement>('note-title').value = titleOf(this.currentPath);
      toast(describeError(err));
    }
  }

  private async deleteNote(): Promise<void> {
    if (!this.currentPath) return;
    if (!(await confirmAction(`Mover "${titleOf(this.currentPath)}" para a lixeira do cofre?`))) return;
    this.saveSoon.cancel();
    this.dirtyContent = null;
    const path = this.currentPath;
    try {
      await notesApi.remove(path);
      const next = this.tabs.close(path);
      await this.refreshTree();
      if (next) await this.openNote(next);
      else this.showEmptyNote();
    } catch (err) {
      toast(describeError(err));
    }
  }

  /** Maps the vault's source code into linked notes and shows them in the graph. */
  private async importProject(): Promise<void> {
    if (!this.vault) return;
    const ok = await confirmAction(
      'Mapear o código deste cofre em notas? Elas serão criadas em docs/cerebro/ ' +
        '(dentro da pasta do projeto). Notas suas nunca são sobrescritas.',
    );
    if (!ok) return;

    const button = byId<HTMLButtonElement>('import-project');
    button.disabled = true;
    const stop = brainEvents.onProgress((done, total) => toast(`Mapeando o projeto… ${done}/${total}`, 'info'));
    try {
      const result = await brainApi.importProject();
      await this.refreshTree();
      toast(importSummary(result), 'info');
      this.showView('graph');
    } catch (err) {
      toast(describeError(err));
    } finally {
      stop();
      button.disabled = false;
    }
  }

  private async runSearch(query: string): Promise<void> {
    if (!query.trim()) {
      this.search.hide();
      byId('tree').hidden = false;
      return;
    }
    try {
      const hits = await indexApi.search(query);
      byId('tree').hidden = true;
      this.search.render(hits);
    } catch (err) {
      toast(describeError(err));
    }
  }

  private showView(view: ViewName): void {
    byId('editor-view').hidden = view !== 'editor';
    byId('tabs').hidden = view !== 'editor';
    byId('graph-view').hidden = view !== 'graph';
    byId('rail-notes').classList.toggle('active', view === 'editor');
    byId('rail-graph').classList.toggle('active', view === 'graph');
    // Graph-only mode applies to the graph view and is remembered for next time.
    byId('workspace').classList.toggle('graph-focus', view === 'graph' && this.graphFocus());
    if (view === 'graph') void this.loadGraph();
    else this.graph.hide();
  }

  private graphFocus(): boolean {
    try {
      return localStorage.getItem(GRAPH_FOCUS_KEY) === '1';
    } catch {
      return byId('workspace').classList.contains('graph-focus');
    }
  }

  /** Shows only the rail and the graph, hiding the explorer and the context column. */
  private setGraphFocus(on: boolean): void {
    try {
      localStorage.setItem(GRAPH_FOCUS_KEY, on ? '1' : '0');
    } catch {
      // Without storage the mode still applies until the app closes.
    }
    byId('workspace').classList.toggle('graph-focus', on && !byId('graph-view').hidden);
    byId('graph-focus').title = on ? 'Voltar ao layout completo (F ou Esc)' : 'Mostrar só o grafo (F)';
  }

  private async loadGraph(): Promise<void> {
    await this.flush();
    try {
      this.graph.setData(await indexApi.graph());
      this.graph.show();
    } catch (err) {
      toast(describeError(err));
    }
  }

  private setStatus(text: string): void {
    byId('save-status').textContent = text;
  }
}

/** Whether a changed path (a note, a folder, or "" for the whole vault) covers note. */
function covers(changed: string, note: string): boolean {
  // Vault paths are case-insensitive on Windows, the platform this app targets.
  const c = changed.toLowerCase();
  const n = note.toLowerCase();
  return c === '' || c === n || n.startsWith(c + '/');
}

function isTyping(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));
}

function importSummary(r: ImportResult): string {
  const parts = [`${r.units} pastas de código mapeadas`];
  if (r.created) parts.push(`${r.created} notas criadas`);
  if (r.updated) parts.push(`${r.updated} atualizadas`);
  if (r.removed) parts.push(`${r.removed} removidas`);
  if (!r.created && !r.updated && !r.removed) parts.push('nada mudou desde a última importação');
  if (r.skipped.length) parts.push(`${r.skipped.length} ignoradas por já existirem notas suas no lugar`);
  if (r.kept.length) parts.push(`${r.kept.length} mantidas por terem anotações suas`);
  return parts.join(' · ') + '.';
}

function titleOf(path: string): string {
  return (path.split('/').pop() ?? path).replace(/\.md$/i, '');
}

void new App().start();
