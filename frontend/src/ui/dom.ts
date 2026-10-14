// Small DOM helpers shared by the views.

export function byId<T extends HTMLElement = HTMLElement>(id: string): T {
  const el = document.getElementById(id);
  if (!el) throw new Error(`missing #${id}`);
  return el as T;
}

export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: Partial<HTMLElementTagNameMap[K]> & { className?: string } = {},
  ...children: (Node | string)[]
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  Object.assign(el, props);
  el.append(...children);
  return el;
}

let toastTimer = 0;

export function toast(message: string, kind: 'error' | 'info' = 'error'): void {
  const el = byId('toast');
  el.textContent = message;
  el.dataset.kind = kind;
  el.hidden = false;
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (el.hidden = true), 4000);
}

/** Shows the shared <dialog> and resolves with the typed text, or null on cancel. */
export function askText(title: string, initial = ''): Promise<string | null> {
  return showDialog(title, initial, true);
}

/** Shows the shared <dialog> as a yes/no confirmation. */
export async function confirmAction(title: string): Promise<boolean> {
  return (await showDialog(title, '', false)) !== null;
}

function showDialog(title: string, initial: string, withInput: boolean): Promise<string | null> {
  const dialog = byId<HTMLDialogElement>('dialog');
  const input = byId<HTMLInputElement>('dialog-input');
  byId('dialog-title').textContent = title;
  input.hidden = !withInput;
  input.value = initial;

  return new Promise((resolve) => {
    dialog.addEventListener(
      'close',
      () => resolve(dialog.returnValue === 'ok' ? input.value.trim() : null),
      { once: true },
    );
    dialog.returnValue = '';
    dialog.showModal();
    if (withInput) input.select();
  });
}

export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number) {
  let timer = 0;
  const debounced = (...args: A) => {
    window.clearTimeout(timer);
    timer = window.setTimeout(() => fn(...args), ms);
  };
  debounced.cancel = () => window.clearTimeout(timer);
  return debounced;
}
