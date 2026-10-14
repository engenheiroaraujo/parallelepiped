// Markdown editor (CodeMirror 6) with [[wikilink]] highlighting,
// autocompletion and Ctrl+click navigation.

import { basicSetup } from 'codemirror';
import { EditorView, Decoration, MatchDecorator, ViewPlugin, type DecorationSet, type ViewUpdate } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { markdown } from '@codemirror/lang-markdown';
import { autocompletion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';

const wikilinkDecorator = new MatchDecorator({
  regexp: /\[\[[^\[\]\n]+\]\]/g,
  decoration: Decoration.mark({ class: 'cm-wikilink' }),
});

const wikilinks = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = wikilinkDecorator.createDeco(view);
    }
    update(update: ViewUpdate) {
      this.decorations = wikilinkDecorator.updateDeco(update, this.decorations);
    }
  },
  { decorations: (plugin) => plugin.decorations },
);

// YAML front matter ("---" ... "---" at the top). Markdown alone reads its
// last line plus the closing "---" as a setext heading; mark the block so CSS
// shows it as quiet metadata instead.
const MAX_FRONT_MATTER_LINES = 80;
const frontMatterLine = Decoration.line({ class: 'cm-frontmatter' });

function frontMatterDecorations(view: EditorView): DecorationSet {
  const doc = view.state.doc;
  if (doc.lines < 2 || doc.line(1).text.trim() !== '---') return Decoration.none;
  const last = Math.min(doc.lines, MAX_FRONT_MATTER_LINES);
  for (let n = 2; n <= last; n++) {
    if (doc.line(n).text.trim() !== '---') continue;
    const lines = [];
    for (let i = 1; i <= n; i++) lines.push(frontMatterLine.range(doc.line(i).from));
    return Decoration.set(lines);
  }
  return Decoration.none;
}

const frontMatter = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = frontMatterDecorations(view);
    }
    update(update: ViewUpdate) {
      if (update.docChanged) this.decorations = frontMatterDecorations(update.view);
    }
  },
  { decorations: (plugin) => plugin.decorations },
);

const markdownHighlight = HighlightStyle.define([
  { tag: tags.heading1, fontSize: '1.6em', fontWeight: '700' },
  { tag: tags.heading2, fontSize: '1.35em', fontWeight: '700' },
  { tag: tags.heading3, fontSize: '1.15em', fontWeight: '700' },
  { tag: [tags.heading4, tags.heading5, tags.heading6], fontWeight: '700' },
  { tag: tags.strong, fontWeight: '700' },
  { tag: tags.emphasis, fontStyle: 'italic' },
  { tag: tags.strikethrough, textDecoration: 'line-through' },
  { tag: tags.link, color: 'var(--accent)' },
  { tag: tags.url, color: 'var(--muted)' },
  { tag: tags.monospace, fontFamily: 'var(--mono)', color: 'var(--code)' },
  { tag: tags.quote, color: 'var(--muted)', fontStyle: 'italic' },
  { tag: [tags.processingInstruction, tags.meta], color: 'var(--faint)' },
]);

const theme = EditorView.theme(
  {
    // Transparent: the editor sits on the "paper" panel of the layout.
    '&': { height: '100%', backgroundColor: 'transparent', color: 'var(--text)', fontSize: '15px' },
    '.cm-scroller': { fontFamily: 'var(--sans)', lineHeight: '1.75', padding: '36px 0' },
    '.cm-content': { maxWidth: '760px', margin: '0 auto', padding: '0 40px', caretColor: 'var(--accent)' },
    '.cm-gutters': { display: 'none' },
    '.cm-activeLine': { backgroundColor: 'transparent' },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': { backgroundColor: 'var(--selection) !important' },
    '.cm-cursor': { borderLeftColor: 'var(--accent)' },
    '.cm-tooltip': { backgroundColor: 'var(--panel)', border: '1px solid var(--border)' },
  },
);

export interface EditorOptions {
  onChange: (content: string) => void;
  onFollowLink: (target: string) => void;
  linkTargets: () => string[];
}

export class Editor {
  private readonly view: EditorView;
  private silent = false;

  constructor(parent: HTMLElement, private readonly options: EditorOptions) {
    this.view = new EditorView({
      parent,
      state: this.createState(''),
    });
  }

  /** Replaces the document without firing onChange. */
  load(content: string): void {
    this.silent = true;
    this.view.setState(this.createState(content));
    this.silent = false;
  }

  /** Current document text. */
  content(): string {
    return this.view.state.doc.toString();
  }

  focus(): void {
    this.view.focus();
  }

  private createState(doc: string): EditorState {
    return EditorState.create({
      doc,
      extensions: [
        basicSetup,
        markdown(),
        syntaxHighlighting(markdownHighlight),
        theme,
        EditorView.lineWrapping,
        wikilinks,
        frontMatter,
        autocompletion({ override: [(ctx) => this.completeLink(ctx)] }),
        EditorView.updateListener.of((update) => {
          if (update.docChanged && !this.silent) this.options.onChange(update.state.doc.toString());
        }),
        EditorView.domEventHandlers({
          mousedown: (event, view) => this.followLinkOnCtrlClick(event, view),
        }),
      ],
    });
  }

  private completeLink(ctx: CompletionContext): CompletionResult | null {
    const before = ctx.matchBefore(/\[\[[^\[\]\n|]*/);
    if (!before) return null;
    return {
      from: before.from + 2,
      options: this.options.linkTargets().map((path) => {
        const title = path.replace(/\.md$/i, '').split('/').pop() ?? path;
        return {
          label: title,
          detail: path,
          // closeBrackets may already have inserted "]]" after the cursor.
          apply: (view: EditorView, _completion: unknown, from: number, to: number) => {
            const closed = view.state.sliceDoc(to, to + 2) === ']]';
            const insert = closed ? title : `${title}]]`;
            view.dispatch({
              changes: { from, to, insert },
              selection: { anchor: from + title.length + 2 },
            });
          },
        };
      }),
      validFor: /^[^\[\]\n|]*$/,
    };
  }

  private followLinkOnCtrlClick(event: MouseEvent, view: EditorView): boolean {
    if (!(event.ctrlKey || event.metaKey)) return false;
    const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
    if (pos === null) return false;

    const line = view.state.doc.lineAt(pos);
    const offset = pos - line.from;
    for (const match of line.text.matchAll(/\[\[([^\[\]|#^\n]+)[^\[\]\n]*\]\]/g)) {
      const start = match.index ?? 0;
      if (offset >= start && offset <= start + match[0].length) {
        event.preventDefault();
        this.options.onFollowLink(match[1].trim());
        return true;
      }
    }
    return false;
  }
}
