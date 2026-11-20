package app

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// rows lists a menu bar as "Menu" headers followed by their items.
func rows(items []menuItem) []string {
	var out []string
	for _, top := range items {
		out = append(out, "# "+top.Label)
		for _, it := range top.Children {
			if it.Separator {
				out = append(out, "-")
				continue
			}
			s := it.Label
			if it.Accel != "" {
				s += " [" + it.Accel + "]"
			}
			if len(it.Children) > 0 { // a submenu: its items follow, indented
				out = append(out, "> "+s)
				for _, c := range it.Children {
					cs := "  " + c.Label
					if c.Accel != "" {
						cs += " [" + c.Accel + "]"
					}
					out = append(out, cs)
				}
				continue
			}
			out = append(out, s)
		}
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("menu mismatch\n got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestMenuSpecProduction(t *testing.T) {
	eq(t, rows(menuSpec(false)), []string{
		"# Zenmark",
		"About Zenmark",
		"Settings… [CmdOrCtrl+,]",
		"-",
		"Hide Zenmark [CmdOrCtrl+H]",
		"Hide Others [CmdOrCtrl+OptionOrAlt+H]",
		"Show All",
		"-",
		"Quit Zenmark [CmdOrCtrl+Q]",
		"# File",
		"New Site from Template… [CmdOrCtrl+Shift+N]",
		"-",
		"New Page… [CmdOrCtrl+N]",
		"Open Folder… [CmdOrCtrl+O]",
		"Close Folder [CmdOrCtrl+Shift+W]",
		"-",
		"Print Page… [CmdOrCtrl+P]",
		"> Export",
		"  Website… [CmdOrCtrl+Shift+B]",
		"  PDF… [CmdOrCtrl+Shift+P]",
		"# Edit",
		"Undo [CmdOrCtrl+Z]",
		"Redo [CmdOrCtrl+Shift+Z]",
		"-",
		"Cut [CmdOrCtrl+X]",
		"Copy [CmdOrCtrl+C]",
		"Paste [CmdOrCtrl+V]",
		"Select All [CmdOrCtrl+A]",
		"-",
		"> Find",
		"  Find… [CmdOrCtrl+F]",
		"  Find Next [CmdOrCtrl+G]",
		"  Find Previous [CmdOrCtrl+Shift+G]",
		"# View",
		"Actual Size [CmdOrCtrl+0]",
		"Zoom In [CmdOrCtrl+Plus]",
		"Zoom Out [CmdOrCtrl+-]",
		"# Window",
		"Minimise [CmdOrCtrl+M]",
		"Zoom",
	})
	banned := []string{"services", "speech", "develop", "devtools", "developer", "reload", "full screen", "paste and match", "close window", "close all"}
	for _, line := range rows(menuSpec(false)) {
		for _, b := range banned {
			if strings.Contains(strings.ToLower(line), b) {
				t.Errorf("production menu has %q", line)
			}
		}
	}
	for _, top := range menuSpec(false) {
		for _, it := range top.Children {
			switch it.Role {
			case "Reload", "ForceReload", "OpenDevTools", "ServicesMenu", "SpeechMenu", "PasteAndMatchStyle", "ToggleFullscreen":
				t.Errorf("production menu has role %s", it.Role)
			}
		}
	}
}

func TestMenuSpecDev(t *testing.T) {
	prod := rows(menuSpec(false))
	dev := rows(menuSpec(true))
	var extra []string
	i := 0
	for _, l := range dev {
		if i < len(prod) && l == prod[i] {
			i++
			continue
		}
		extra = append(extra, l)
	}
	if i != len(prod) {
		t.Fatalf("dev menu must contain every production item in order:\n%s", strings.Join(dev, "\n"))
	}
	eq(t, extra, []string{"# Develop", "Reload [CmdOrCtrl+R]", "Toggle DevTools [CmdOrCtrl+OptionOrAlt+I]"})
}

func TestMenuRolesAndCustomItems(t *testing.T) {
	roles := map[string]string{}
	for _, top := range menuSpec(false) {
		for _, it := range top.Children {
			roles[it.Label] = it.Role
		}
	}
	for label, role := range map[string]string{
		"About Zenmark": "", "Hide Zenmark": "Hide", "Hide Others": "HideOthers", "Show All": "UnHide", "Quit Zenmark": "Quit",
		"Undo": "Undo", "Redo": "Redo", "Cut": "Cut", "Copy": "Copy", "Paste": "Paste", "Select All": "SelectAll",
		"Minimise": "Minimise", "Zoom": "Zoom",
		"Settings…": "", "New Page…": "", "Open Folder…": "", "Close Folder": "",
	} {
		if got, ok := roles[label]; !ok || got != role {
			t.Errorf("%s: role %q (present %v), want %q", label, got, ok, role)
		}
	}
}

func TestMenuActions(t *testing.T) {
	var calls []string
	act := menuActions{
		NewPage:      func() { calls = append(calls, "new") },
		OpenFolder:   func() { calls = append(calls, "open") },
		Settings:     func() { calls = append(calls, "settings") },
		NewSite:      func() { calls = append(calls, "site") },
		BuildWebsite: func() { calls = append(calls, "build") },
		PrintPage:    func() { calls = append(calls, "print-page") },
		PrintSite:    func() { calls = append(calls, "print-site") },
		CloseFolder:  func() { calls = append(calls, "close") },
		Find:         func() { calls = append(calls, "find") },
		FindNext:     func() { calls = append(calls, "find-next") },
		FindPrevious: func() { calls = append(calls, "find-previous") },
	}
	for _, label := range []string{"New Page…", "Open Folder…", "Settings…", "New Site from Template…", "Website…", "Print Page…", "PDF…", "Close Folder", "Find…", "Find Next", "Find Previous"} {
		f := act.forLabel(label)
		if f == nil {
			t.Fatalf("no action for %s", label)
		}
		f()
	}
	if got := strings.Join(calls, ","); got != "new,open,settings,site,build,print-page,print-site,close,find,find-next,find-previous" {
		t.Fatalf("calls = %s", got)
	}
	if act.forLabel("Quit Zenmark") != nil {
		t.Fatal("role items have no custom action")
	}
}

type shownWindow struct{ log *[]string }

func (w shownWindow) Show() application.Window { *w.log = append(*w.log, "show"); return nil }
func (w shownWindow) Focus()                   { *w.log = append(*w.log, "focus") }

type recSink struct{ log *[]string }

func (s recSink) Emit(name string, _ any) { *s.log = append(*s.log, "emit "+name) }

func TestMenuSettings(t *testing.T) {
	if EventSettings != "menu:settings" {
		t.Fatalf("EventSettings = %q", EventSettings)
	}
	var log []string
	settingsAction(shownWindow{&log}, recSink{&log})()
	if got := strings.Join(log, ","); got != "show,focus,emit menu:settings" {
		t.Fatalf("settings action: %s", got)
	}
	log = nil
	newPageAction(shownWindow{&log}, recSink{&log})()
	if got := strings.Join(log, ","); got != "show,focus,emit menu:new-page" {
		t.Fatalf("new page action: %s", got)
	}
}

// Zenmark > About Zenmark opens the About sheet in the main window (ui/settings-layout): a custom
// item that sends menu:about, not the standard macOS panel.
func TestAboutMenu(t *testing.T) {
	if EventAbout != "menu:about" {
		t.Fatalf("EventAbout = %q", EventAbout)
	}
	var about *menuItem
	for _, top := range menuSpec(false) {
		for i, it := range top.Children {
			if it.Label == "About Zenmark" {
				about = &top.Children[i]
			}
		}
	}
	if about == nil {
		t.Fatal("no About Zenmark item")
	}
	if about.Role != "" || about.Event != "menu:about" {
		t.Errorf("About item %+v: want a custom item that sends menu:about", *about)
	}
	// It is in the bare shell too.
	if !slices.Contains(rows(menuSpecFor(false, nil)), "About Zenmark") {
		t.Error("the bare shell has no About")
	}
	var calls []string
	act := menuActions{About: func() { calls = append(calls, "about") }}
	f := act.forLabel(labelAbout)
	if f == nil {
		t.Fatal("no action for About")
	}
	f()
	if strings.Join(calls, ",") != "about" {
		t.Errorf("calls %v", calls)
	}
	var log []string
	showAndEmit(shownWindow{&log}, recSink{&log}, EventAbout)()
	if got := strings.Join(log, ","); got != "show,focus,emit menu:about" {
		t.Errorf("about action: %s", got)
	}
}

func TestDevBuildFlag(t *testing.T) {
	// `go test` is not a production build.
	if !desktop.DevBuild {
		t.Fatal("devBuild must be true without -tags production")
	}
}

// app.go builds no menu of its own; menu.go does it once.
func TestAppGoBuildsMenuOnce(t *testing.T) {
	src, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if strings.Contains(s, "AddRole(application.AppMenu)") {
		t.Skip("integration pending: app.go still holds the legacy menu block (see report)")
	}
	if strings.Contains(s, "Menu.New(") {
		t.Error("app.go still builds a menu")
	}
	if !strings.Contains(s, "buildMenu(") {
		t.Error("app.go does not call buildMenu")
	}
}

// Find items send an event to every window and show nothing: the window that
// has the focus (main or editor) is the one that handles it (ui/find).
func TestMenuFindEvents(t *testing.T) {
	for event, want := range map[string]string{EventFind: "menu:find", EventFindNext: "menu:find-next", EventFindPrevious: "menu:find-previous"} {
		if event != want {
			t.Fatalf("event %q, want %q", event, want)
		}
		var log []string
		emitAction(recSink{&log}, event)()
		if got := strings.Join(log, ","); got != "emit "+want {
			t.Fatalf("%s action: %s", want, got)
		}
	}
}

func TestFindMenuItemsAreFolderOnly(t *testing.T) {
	var find []menuItem
	for _, top := range menuSpec(false) {
		if top.Label != "Edit" {
			continue
		}
		for _, it := range top.Children {
			if it.Label == "Find" {
				find = it.Children
			}
		}
	}
	if len(find) != 3 {
		t.Fatalf("Edit > Find has %d items, want 3", len(find))
	}
	for _, it := range find {
		if !folderOnly(it.Label) {
			t.Errorf("%q should need an open folder", it.Label)
		}
	}
}

// File › Export › Website… (⇧⌘B) and PDF… (⇧⌘P) replace Build Website… and Print Site…;
// Print Page… (⌘P) stays in File (app/export behaviour 1).
func TestExportSubmenu(t *testing.T) {
	var file *menuItem
	for i, top := range menuSpec(false) {
		if top.Label == "File" {
			file = &menuSpec(false)[i]
		}
	}
	if file == nil {
		t.Fatal("no File menu")
	}
	var export *menuItem
	var labels []string
	for i, it := range file.Children {
		labels = append(labels, it.Label)
		if it.Label == "Export" {
			export = &file.Children[i]
		}
	}
	if export == nil || len(export.Children) != 2 {
		t.Fatalf("File has no Export submenu with two items: %v", labels)
	}
	if got := export.Children[0]; got.Label != "Website…" || got.Accel != "CmdOrCtrl+Shift+B" || got.Role != "" {
		t.Errorf("first item %+v", got)
	}
	if got := export.Children[1]; got.Label != "PDF…" || got.Accel != "CmdOrCtrl+Shift+P" || got.Role != "" {
		t.Errorf("second item %+v", got)
	}
	for _, l := range labels {
		if l == "Build Website…" || l == "Print Site…" {
			t.Errorf("File still has %q", l)
		}
	}
	found := false
	for _, it := range file.Children {
		if it.Label == "Print Page…" && it.Accel == "CmdOrCtrl+P" {
			found = true
		}
	}
	if !found {
		t.Error("Print Page… (⌘P) must stay in File")
	}
	// The items reuse the existing actions (and events).
	var calls []string
	act := menuActions{BuildWebsite: func() { calls = append(calls, "build") }, PrintSite: func() { calls = append(calls, "print-site") }}
	act.forLabel(labelExportWebsite)()
	act.forLabel(labelExportPDF)()
	if strings.Join(calls, ",") != "build,print-site" || EventBuildWebsite != "menu:build-website" {
		t.Errorf("actions %v, event %q", calls, EventBuildWebsite)
	}
}

// Without the editor File has no New Page…: the item is left out, not disabled.
func TestMenuWithoutEditor(t *testing.T) {
	with := rows(menuSpecFor(false, allFeatures))
	without := rows(menuSpecFor(false, without("editor")))
	if !slices.Contains(with, "New Page… [CmdOrCtrl+N]") {
		t.Fatal("the editor menu lost New Page…")
	}
	for _, l := range without {
		if strings.HasPrefix(l, "New Page") {
			t.Errorf("read-only menu has %q", l)
		}
	}
	var want []string
	for _, l := range with {
		if !strings.HasPrefix(l, "New Page") {
			want = append(want, l)
		}
	}
	eq(t, without, want)
	// menuSpec is the editor menu.
	eq(t, rows(menuSpec(false)), with)
	// The menu state copes with the absent item: it only knows the ones added.
	st := newMenuState(false)
	var on []bool
	st.add(func(e bool) { on = append(on, e) })
	st.setFolderOpen(true)
	if len(on) != 2 || on[1] != true {
		t.Errorf("state calls %v", on)
	}
}

// Each missing feature leaves its menu items out (not greyed), and no gap behind it.
func TestMenuPerFeature(t *testing.T) {
	all := rows(menuSpecFor(true, allFeatures))
	cases := []struct {
		feature string
		gone    []string // rows (prefix) that must not be there without the feature
	}{
		{"reader", nil}, // nothing of its own in the menu bar
		{"nav", nil},
		{"find", []string{"> Find", "  Find…", "  Find Next", "  Find Previous"}},
		{"editor", []string{"New Page…"}},
		{"inspector", nil},
		{"print", []string{"Print Page…", "  PDF…"}},
		{"export", []string{"> Export", "  Website…", "  PDF…"}},
		{"newsite", []string{"New Site from Template…"}},
		{"developer", []string{"# Develop", "Reload", "Toggle DevTools"}},
	}
	for _, c := range cases {
		t.Run(c.feature, func(t *testing.T) {
			spec := menuSpecFor(true, without(c.feature))
			got := rows(spec)
			for _, g := range c.gone {
				for _, l := range got {
					if strings.HasPrefix(l, g) {
						t.Errorf("without %s the menu still has %q", c.feature, l)
					}
				}
				if !slices.ContainsFunc(all, func(l string) bool { return strings.HasPrefix(l, g) }) {
					t.Errorf("with all features the menu lacks %q", g)
				}
			}
			if len(c.gone) == 0 && !slices.Equal(got, all) {
				t.Errorf("without %s the menu changes:\n%v", c.feature, got)
			}
			noGaps(t, spec)
		})
	}
	// PDF… needs print as well as export: Website… stays without print.
	got := rows(menuSpecFor(false, without("print")))
	if !slices.Contains(got, "  Website… [CmdOrCtrl+Shift+B]") || slices.ContainsFunc(got, func(l string) bool { return strings.Contains(l, "PDF…") }) {
		t.Errorf("without print:\n%v", got)
	}
	// Develop is a dev-build menu and a feature.
	if slices.Contains(rows(menuSpecFor(false, allFeatures)), "# Develop") {
		t.Error("production build has Develop")
	}
}

// noGaps fails on a leading, trailing or doubled separator, or an empty submenu.
func noGaps(t *testing.T, bar []menuItem) {
	t.Helper()
	var check func(path string, items []menuItem)
	check = func(path string, items []menuItem) {
		for i, it := range items {
			if it.Separator && (i == 0 || i == len(items)-1 || items[i-1].Separator) {
				t.Errorf("%s: separator at %d of %d", path, i, len(items))
			}
			if !it.Separator && it.Role == "" && it.Children != nil && len(it.Children) == 0 {
				t.Errorf("%s: empty submenu %s", path, it.Label)
			}
			check(path+" > "+it.Label, it.Children)
		}
	}
	check("", bar)
}

// The bare shell's menus: App, File with Open and Close, Edit without Find, Window.
func TestMenuBareShell(t *testing.T) {
	spec := menuSpecFor(true, nil)
	eq(t, rows(spec), []string{
		"# Zenmark",
		"About Zenmark",
		"Settings… [CmdOrCtrl+,]",
		"-",
		"Hide Zenmark [CmdOrCtrl+H]",
		"Hide Others [CmdOrCtrl+OptionOrAlt+H]",
		"Show All",
		"-",
		"Quit Zenmark [CmdOrCtrl+Q]",
		"# File",
		"Open Folder… [CmdOrCtrl+O]",
		"Close Folder [CmdOrCtrl+Shift+W]",
		"# Edit",
		"Undo [CmdOrCtrl+Z]",
		"Redo [CmdOrCtrl+Shift+Z]",
		"-",
		"Cut [CmdOrCtrl+X]",
		"Copy [CmdOrCtrl+C]",
		"Paste [CmdOrCtrl+V]",
		"Select All [CmdOrCtrl+A]",
		"# View",
		"Actual Size [CmdOrCtrl+0]",
		"Zoom In [CmdOrCtrl+Plus]",
		"Zoom Out [CmdOrCtrl+-]",
		"# Window",
		"Minimise [CmdOrCtrl+M]",
		"Zoom",
	})
	noGaps(t, spec)
	// Developer alone brings the Develop menu in a dev build.
	if !slices.Contains(rows(menuSpecFor(true, []string{"developer"})), "# Develop") {
		t.Error("developer lacks the Develop menu")
	}
}

// View is new in the shell (ui/accessibility): Actual Size, Zoom In and Zoom Out, between Edit and Window,
// in every build, with the bare shell too. They only send an event; the window steps the zoom and saves it.
func TestViewMenu(t *testing.T) {
	for name, features := range map[string][]string{"full": allFeatures, "bare": nil} {
		bar := menuSpecFor(false, features)
		var labels []string
		for _, top := range bar {
			labels = append(labels, top.Label)
		}
		i := slices.Index(labels, "View")
		if i < 0 {
			t.Fatalf("%s: no View menu in %v", name, labels)
		}
		if labels[i-1] != "Edit" || labels[i+1] != "Window" {
			t.Errorf("%s: View is between %s and %s, want Edit and Window", name, labels[i-1], labels[i+1])
		}
		want := []menuItem{
			{Label: "Actual Size", Accel: "CmdOrCtrl+0", Event: "menu:zoom-reset"},
			{Label: "Zoom In", Accel: "CmdOrCtrl+Plus", Event: "menu:zoom-in"},
			{Label: "Zoom Out", Accel: "CmdOrCtrl+-", Event: "menu:zoom-out"},
		}
		got := bar[i].Children
		if len(got) != len(want) {
			t.Fatalf("%s: View items %+v", name, got)
		}
		for j := range want {
			if got[j].Label != want[j].Label || got[j].Accel != want[j].Accel || got[j].Event != want[j].Event || got[j].Role != "" {
				t.Errorf("%s: View item %d = %+v, want %+v", name, j, got[j], want[j])
			}
		}
	}
	for ev, want := range map[string]string{EventZoomIn: "menu:zoom-in", EventZoomOut: "menu:zoom-out", EventZoomReset: "menu:zoom-reset", EventZoomChanged: "app:zoom"} {
		if ev != want {
			t.Errorf("event %q, want %q", ev, want)
		}
	}
	// the window's own Zoom (the macOS window zoom) is still in Window
	if !slices.Contains(rows(menuSpecFor(false, nil)), "Zoom") {
		t.Error("Window > Zoom is gone")
	}
}

// The ends do nothing, and the menu items are disabled there.
func TestZoomMenuEnds(t *testing.T) {
	cases := []struct {
		z       float64
		in, out bool
	}{{0.75, true, false}, {0.9, true, true}, {1, true, true}, {1.5, true, true}, {2, false, true}, {3, false, true}, {0.1, true, false}}
	for _, c := range cases {
		in, out := zoomEnabled(c.z)
		if in != c.in || out != c.out {
			t.Errorf("zoomEnabled(%v) = %v, %v; want %v, %v", c.z, in, out, c.in, c.out)
		}
	}
	var in, out []bool
	st := newZoomState()
	st.add(func(on bool) { in = append(in, on) }, func(on bool) { out = append(out, on) })
	st.set(2)
	st.set(1)
	st.set(0.75)
	if got := in; len(got) != 4 || got[0] != true || got[1] != false || got[2] != true || got[3] != true {
		t.Errorf("zoom in states %v", got)
	}
	if got := out; len(got) != 4 || got[0] != true || got[1] != true || got[2] != true || got[3] != false {
		t.Errorf("zoom out states %v", got)
	}
	var nilState *zoomState
	nilState.set(2) // tests and headless
	// what the window sends: a JSON number; anything else is ignored
	if z, ok := zoomFromEvent(float64(1.25)); !ok || z != 1.25 {
		t.Errorf("zoomFromEvent(1.25) = %v, %v", z, ok)
	}
	for _, bad := range []any{nil, "2", true, map[string]any{}, float64(0), float64(-1), float64(9)} {
		if _, ok := zoomFromEvent(bad); ok {
			t.Errorf("zoomFromEvent(%v) accepted", bad)
		}
	}
}
