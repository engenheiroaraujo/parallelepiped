// Application controller: wires the views to the backend calls.

import './style.css';
import { brainApi, brainEvents, indexApi, notesApi, vaultApi, vaultEvents, type ImportResult, type VaultInfo } from './api/api';
import { AppError, describeError } from './api/client';
import { askText, byId, confirmAction, debounce, h, toast } from './ui/dom';
import { Editor } from './views/editor';
import { GraphView } from './views/graph';
import { BacklinksView, SearchView } from './views/panels';
import { TreeView } from './views/tree';

type ViewName = 'editor' | 'graph';

const SAVE_DELAY_MS = 600;

class App {
  private vault: VaultInfo | null = null;
  private currentPath = '';
  private dirtyContent: string | null = null;
  /** Serialises handling of outside changes so refreshes never interleave. */
  private outsideChanges: Promise<void> = Promise.resolve();

  private readonly tree = new TreeView((path) => this.openNote(path));
  private readonly backlinks = new BacklinksView((path) => this.openNote(path));
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

  async start(): Promise<void> {
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
    byId('tab-editor').addEventListener('click', () => this.showView('editor'));
    byId('tab-graph').addEventListener('click', () => this.showView('graph'));

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

    window.addEventListener('keydown', (e) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      const key = e.key.toLowerCase();
      if (key === 'n') {
        e.preventDefault();
        void this.createNote();
      } else if (key === 'g') {
        e.preventDefault();
        this.showView(byId('graph-view').hidden ? 'graph' : 'editor');
      } else if (key === 'f' && e.shiftKey) {
        e.preventDefault();
        searchInput.focus();
      } else if (key === 's') {
        e.preventDefault();
        void this.flush();
      }
    });
    window.addEventListener('beforeunload', () => void this.flush());
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
    byId('welcome').hidden = true;
    byId('workspace').hidden = false;
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
        const button = h('button', { className: 'link-item', textContent: v.name, title: v.path });
        button.addEventListener('click', () => vaultApi.open(v.path).then((info) => this.useVault(info)).catch((err) => toast(describeError(err))));
        return h('li', {}, button);
      }),
    );
  }

  private async refreshTree(): Promise<void> {
    this.tree.render(await vaultApi.tree());
  }

  async openNote(path: string): Promise<void> {
    await this.flush();
    try {
      const note = await notesApi.read(path);
      this.currentPath = note.path;
      this.editor.load(note.content);
      this.tree.setActive(note.path);
      byId<HTMLInputElement>('note-title').value = note.title;
      byId('note-bar').hidden = false;
      byId('editor-empty').hidden = true;
      this.setStatus('');
      this.showView('editor');
      this.editor.focus();
      this.backlinks.render(await indexApi.backlinks(note.path));
    } catch (err) {
      toast(describeError(err));
    }
  }

  private showEmptyNote(): void {
    this.editor.load('');
    this.tree.setActive('');
    this.backlinks.clear();
    byId('note-bar').hidden = true;
    byId('editor-empty').hidden = false;
  }

  /** Opens the note a [[link]] points to, creating it when missing. */
  private async followLink(target: string): Promise<void> {
    const wanted = target.toLowerCase().replace(/\.md$/, '');
    const paths = this.tree.notePaths();
    const match =
      paths.find((p) => p.toLowerCase().replace(/\.md$/, '') === wanted) ??
      paths.find((p) => titleOf(p).toLowerCase() === wanted);
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
    this.saveSoon();
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
      if (this.currentPath) this.backlinks.render(await indexApi.backlinks(this.currentPath));
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
      if (note.content !== this.editor.content()) this.editor.load(note.content);
    } catch (err) {
      if (!(err instanceof AppError && err.code === 'not_found')) throw err;
      if (this.currentPath !== path || this.dirtyContent !== null) return;
      this.currentPath = '';
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
    const folder = this.currentPath.includes('/') ? this.currentPath.slice(0, this.currentPath.lastIndexOf('/') + 1) : '';
    try {
      const note = await notesApi.rename(this.currentPath, folder + title);
      this.currentPath = note.path;
      await this.refreshTree();
      this.tree.setActive(note.path);
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
    try {
      await notesApi.remove(this.currentPath);
      this.currentPath = '';
      await this.refreshTree();
      this.showEmptyNote();
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
    byId('graph-view').hidden = view !== 'graph';
    byId('tab-editor').classList.toggle('active', view === 'editor');
    byId('tab-graph').classList.toggle('active', view === 'graph');
    if (view === 'graph') void this.loadGraph();
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
