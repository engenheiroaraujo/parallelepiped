// Thin typed facade over the generated Wails bindings. Views import from here,
// never from wailsjs directly.

import * as Vault from '../../wailsjs/go/handler/Vault';
import * as Notes from '../../wailsjs/go/handler/Notes';
import * as Index from '../../wailsjs/go/handler/Index';
import type { dto } from '../../wailsjs/go/models';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { call } from './client';

export type VaultInfo = dto.VaultInfo;
export type TreeEntry = dto.TreeEntry;
export type Note = dto.NoteResponse;
export type GraphData = dto.GraphResponse;
export type GraphNodeData = dto.GraphNode;
export type Backlink = dto.BacklinkItem;
export type SearchHit = dto.SearchHitItem;

export const vaultApi = {
  choose: () => call(Vault.Choose()),
  open: (path: string) => call(Vault.Open(path)),
  recent: async () => (await call(Vault.Recent())).vaults ?? [],
  tree: async () => (await call(Vault.Tree())).root,
};

export const notesApi = {
  read: (path: string) => call(Notes.Read(path)),
  save: (path: string, content: string) => call(Notes.Save(path, content)),
  create: (path: string) => call(Notes.Create(path)),
  rename: (from: string, to: string) => call(Notes.Rename(from, to)),
  remove: (path: string) => call(Notes.Delete(path)),
};

export const indexApi = {
  graph: () => call(Index.Graph()),
  backlinks: async (path: string) => (await call(Index.Backlinks(path))).backlinks ?? [],
  search: async (query: string) => (await call(Index.Search(query, 0))).hits ?? [],
};

/** Payload of the "vault:changed" event (watcher/dto.VaultChanged). */
interface VaultChanged {
  paths: string[] | null;
}

export const vaultEvents = {
  /**
   * Notes changed on disk outside the app. Paths are vault-relative; a folder
   * path covers everything below it and "" is the whole vault.
   */
  onChanged: (callback: (paths: string[]) => void): (() => void) =>
    EventsOn('vault:changed', (payload: VaultChanged) => callback(payload.paths ?? [])),
};
