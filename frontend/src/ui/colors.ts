// Folder colours shared by the file tree, the context panel and the 3D graph,
// so a folder is the same colour everywhere.

export const FOLDER_PALETTE = ['#a855f7', '#ef4444', '#f97316', '#eab308', '#10b981', '#3b82f6', '#ec4899', '#14b8a6', '#84cc16', '#f43f5e'];
export const ROOT_COLOR = '#a1a1aa';
export const UNRESOLVED_COLOR = '#71717a';

/** The folder that directly contains a note ("" at the vault root). */
export function folderOf(path: string): string {
  const at = path.lastIndexOf('/');
  return at < 0 ? '' : path.slice(0, at);
}

/**
 * Assigns palette colours to folders in sorted order. The graph and the tree
 * pass the same folders (those that hold notes), so they get the same map.
 */
export function folderColors(folders: Iterable<string>): Map<string, string> {
  const sorted = [...new Set(folders)].filter((f) => f !== '').sort();
  return new Map(sorted.map((folder, i) => [folder, FOLDER_PALETTE[i % FOLDER_PALETTE.length]]));
}

/** Colour of the folder holding `path`, or the neutral root colour. */
export function colorOfNote(colors: Map<string, string>, path: string): string {
  return colors.get(folderOf(path)) ?? ROOT_COLOR;
}
