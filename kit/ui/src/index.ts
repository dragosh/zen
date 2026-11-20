// @zenmd/kit: the shared desktop UI for apps built on zenmd. The window frame, the store and history,
// theme and keys, the opening loader, the sidebar, the page view, the editor, the lint UI, the print
// preview and the Settings frame. It imports @zenmd/runtime and its own files only
// (kit/ui/test/boundary.test.ts); an app gives it a KitHost.
export { hasAddon, hasFeature } from "./host";
export type {
  AddonInfo, Alert, BuildInfo, Crumb, DialectInfo, Draft, KitHost, LayoutSource, LintResult, LogEntry, LogLevel, NavNode, OpenProgress, Page, PageInfo, PageLayoutRef, PluginStreamEvent, Workspace,
} from "./host";
export { button, el, logo } from "./shell/dom";
export { copyText } from "./shell/clipboard";
export { createHistory, type Entry, type History } from "./shell/history";
export { closeWindowKey, reloadKey, shellAction, zoomKey, type ShellAction } from "./shell/keys";
export { createLayout, type Layout, type LayoutHandlers } from "./shell/layout";
export { createOpening, type Opening, type OpeningOptions } from "./shell/opening";
export { installConsoleCapture, type ConsoleCapture } from "./shell/console";
export { developerEnabled, developerKey, followDeveloper, setDeveloperEnabled } from "./shell/developer";
export { applyAppTheme, buildSection } from "./shell/addons";
export { A11Y_DEFAULTS, ZOOM_STEPS, applyAccessibility, followAccessibility, reducedMotion, resetAccessibility, setAccessibility, stepZoom, storedAccessibility, zoomStep, type Accessibility } from "./shell/a11y";
export { accessibilitySection, appearanceSection, createSettingsFrame, developerSection, reloadSettings, type SettingsItem, type SettingsSection, type SettingsView } from "./shell/settings";
export { openAbout } from "./shell/about";
export { createStore } from "./shell/store";
export { applyTheme, followTheme, setTheme, storedTheme, THEME_KEY, type Theme } from "./shell/theme";
export { FEATURE_IDS, type AppState, type Ctx, type Feature, type FeatureId, type Slots, type Store } from "./shell/types";
export { navFeature } from "./nav";
export { handleLinkClick, mountPage, pageFeature, pageHooks } from "./page";
export { editorFeature, CONFIG_PREFIX, isConfigPath } from "./editor";
export { mountPreview, type PreviewApi, type PreviewOptions } from "./print/preview";
export { clearMarks, flash, issueList, markText, renderPanel, type LintPanel } from "./lint";
export { bindFind, createFind, domFinder, findInText, textareaFinder, type Find, type FindApi, type FindEvents, type Finder } from "./find";
export { createInspector, INSPECTOR_CONSOLE, inspectorKey, pageStats, type Inspector, type InspectorOptions, type InspectorPanel, type PageStats } from "./inspector";
