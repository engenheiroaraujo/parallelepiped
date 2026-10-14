// Light and dark themes. The choice is stored per machine; until the user
// picks one, the app follows the operating system.

export type Theme = 'dark' | 'light';

const STORAGE_KEY = 'parallelepiped.theme';
const listeners = new Set<(theme: Theme) => void>();

function stored(): Theme | null {
  try {
    const value = localStorage.getItem(STORAGE_KEY);
    return value === 'dark' || value === 'light' ? value : null;
  } catch {
    return null;
  }
}

function system(): Theme {
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
}

export function currentTheme(): Theme {
  return (document.documentElement.dataset.theme as Theme | undefined) ?? stored() ?? system();
}

function apply(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
  listeners.forEach((listener) => listener(theme));
}

/** Applies the stored or system theme and follows system changes until the user chooses. */
export function initTheme(): void {
  apply(stored() ?? system());
  window.matchMedia('(prefers-color-scheme: light)').addEventListener('change', () => {
    if (!stored()) apply(system());
  });
}

export function toggleTheme(): Theme {
  const next: Theme = currentTheme() === 'dark' ? 'light' : 'dark';
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // Storage may be unavailable; the theme still applies for this session.
  }
  apply(next);
  return next;
}

export function onThemeChange(listener: (theme: Theme) => void): void {
  listeners.add(listener);
}
