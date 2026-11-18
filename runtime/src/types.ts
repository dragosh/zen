// What the runtime needs from its host: the app's service API (apps/zenmd-app/frontend/src/api, whose `Api` extends this)
// or the storage of a built site (static/static.ts). The runtime imports nothing from the app.

/** One piece of a plugin stream (core/plugin-extras): a chunk, or the end (`done`, with `error` "" on success). */
export type PluginStreamEvent = { call: string; chunk: string; done: boolean; error: string };

export interface RuntimeHost {
  /** A layout's stored value as JSON text; "null" when the key is absent. */
  layoutStorageGet(id: string, key: string): Promise<string>;
  layoutStorageSet(id: string, key: string, json: string): Promise<void>;
  /** The storage keys of a layout that start with prefix ("" for all), sorted. */
  layoutStorageKeys(id: string, prefix: string): Promise<string[]>;
  /** A plugin's stored value as JSON text; "null" when the key is absent. */
  pluginStorageGet(id: string, key: string): Promise<string>;
  pluginStorageSet(id: string, key: string, json: string): Promise<void>;
  /** The storage keys of a plugin that start with prefix ("" for all), sorted. */
  pluginStorageKeys(id: string, prefix: string): Promise<string[]>;
  /**
   * Runs the plugin's backend with ZEN_HOOK=action; JSON text in, JSON text out. `page` is the
   * workspace path of the page the block is on: Go resolves the plugin's settings for that page.
   */
  pluginInvoke(id: string, page: string, json: string): Promise<string>;
  /** Starts the plugin's stream; resolves to the call id. Output arrives as "plugin:stream" events. */
  pluginStream(id: string, page: string, json: string): Promise<string>;
  /** Cancels a stream; it ends with error "cancelled". */
  pluginCancel(call: string): Promise<void>;
  /** Opens a link outside the app; the host asks the reader first. */
  openExternal(url: string): Promise<boolean>;
  /** The plugin or layout set changed: mount the page again. */
  on(event: "plugins:changed", fn: () => void): () => void;
  on(event: "layouts:changed", fn: () => void): () => void;
  on(event: "plugin:stream", fn: (e: PluginStreamEvent) => void): () => void;
}

/** What installRuntime calls: a host without openExternal (a built site has no confirm dialog; links are plain). */
export type RuntimeApi = Omit<RuntimeHost, "openExternal">;

export type { PluginCtx, LayoutCtx, PluginHandler, LayoutHandler } from "./plugins";
