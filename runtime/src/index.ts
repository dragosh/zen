// @zenmd/runtime: the browser half of the engine. Plugin and layout mounting (the ctx contract), print
// preparation and the runtime of built sites. Imports nothing from the app (runtime/test/boundary.test.ts).
export { installRuntime, layoutOfUrl, pluginOfUrl } from "./plugins";
export type { CleanUp, PluginAssets, Runtime } from "./plugins";
export { whenReady, READY_TIMEOUT } from "./print/ready";
export type { ReadyOptions } from "./print/ready";
export { installPagePrint, prepareForPrint, printPage } from "./print/page";
export type { PagePrintApi, PagePrintOptions } from "./print/page";
export { start, staticApi, staticHooks, HOSTED_FROM_DISK } from "./static/static";
export type { StaticApi } from "./static/static";
export type { LayoutCtx, LayoutHandler, PluginCtx, PluginHandler, PluginStreamEvent, RuntimeApi, RuntimeHost } from "./types";
