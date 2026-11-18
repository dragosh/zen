package app

import "github.com/zenmark-org/zenmd/engine/session"

// The JSON types of the service are defined in engine/session; these aliases
// keep the names the frontend bindings and the rest of the app use.
type (
	NavNode           = session.NavNode
	Workspace         = session.Workspace
	Crumb             = session.Crumb
	PageAssets        = session.PageAssets
	PageLayout        = session.PageLayout
	Page              = session.Page
	Startup           = session.Startup
	PageChanged       = session.PageChanged
	PluginInfo        = session.PluginInfo
	PluginHelp        = session.PluginHelp
	PluginSnippet     = session.PluginSnippet
	PluginOption      = session.PluginOption
	PluginStreamEvent = session.PluginStreamEvent
	LayoutInfo        = session.LayoutInfo
	LayoutField       = session.LayoutField
	LayoutOption      = session.LayoutOption
	LintAlert         = session.LintAlert
	LintResult        = session.LintResult
	OpenProgress      = session.OpenProgress
	OpenTimeoutError  = session.OpenTimeoutError
	BuildOptions      = session.BuildOptions
	BuildResult       = session.BuildResult
	BuildWarning      = session.BuildWarning
	BuildProgress     = session.BuildProgress
	SiteTemplate      = session.SiteTemplate
)

// Event names the session emits, and the errors of an open.
const (
	EventWorkspaceChanged = session.EventWorkspaceChanged
	EventPageChanged      = session.EventPageChanged
	EventPluginsChanged   = session.EventPluginsChanged
	EventLayoutsChanged   = session.EventLayoutsChanged
	EventWorkspaceOpening = session.EventWorkspaceOpening
	EventBuildProgress    = session.EventBuildProgress
	EventPluginStream     = session.EventPluginStream
)

var (
	ErrOpenCancelled = session.ErrOpenCancelled
	errNoWorkspace   = session.ErrNoWorkspace
)

// Events the app sends to its own frontend (menus and windows).
const (
	// EventNewPage asks the main window to show the New Page dialog (File >
	// New Page…, Cmd+N). No payload.
	EventNewPage = "menu:new-page"
	// EventBuildWebsite asks the main window to show the Build Website dialog
	// (File > Build Website…, Shift+Cmd+B). No payload.
	EventBuildWebsite = "menu:build-website"
	// EventWorkspaceOpened tells the frontend to show a different workspace,
	// payload Startup. Sent after Finder "Open With" and File > Open Folder.
	EventWorkspaceOpened = "workspace:opened"
)

// EventSink delivers events to the frontend; it is the session's Events.
type EventSink = session.Events
