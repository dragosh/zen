package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/zenmark-org/zenmd/builtins"
	"github.com/zenmark-org/zenmd/engine/session"
	"github.com/zenmark-org/zenmd/kit/assembly"
)

// generatedInfo and the generated registries are what app-assembly.yml makes today. Zenmark is
// rebuilt from a starting shell (specs/kit/shell-features), so its own assembly lists only some
// features, plugins and layouts; the tests of this package would then test a partial app.
var (
	generatedInfo     = assemblyInfo
	generatedPlugins  = builtinPlugins
	generatedLayouts  = builtinLayouts
	generatedDialects = builtinDialects
	generatedSiteSet  = siteBuiltins
)

// fullInfo is the build `features: all` with every built-in of the catalog.
func fullInfo() session.BuildInfo {
	cat, err := assembly.BuiltinCatalog()
	if err != nil {
		panic(err)
	}
	return session.BuildInfo{
		App: "zenmark", Features: slices.Clone(cat.Features), Plugins: slices.Clone(cat.Plugins), Layouts: slices.Clone(cat.Layouts),
		Templates: slices.Clone(cat.Templates), Dialects: slices.Clone(cat.Dialects), Theme: assembly.DefaultTheme,
	}
}

// useFullBuild makes the package's build the full one: all features, every built-in plugin, layout,
// template and dialect.
func useFullBuild() {
	assemblyInfo = fullInfo()
	builtinPlugins = builtins.Plugins
	builtinLayouts = builtins.Layouts
	builtinDialects = builtins.Dialects
	siteBuiltins = builtins.SiteTemplates()
}

// useGeneratedBuild makes the build the one app-assembly.yml makes, for the length of the test.
func useGeneratedBuild(t *testing.T) {
	t.Helper()
	oldInfo, oldP, oldL, oldD, oldS := assemblyInfo, builtinPlugins, builtinLayouts, builtinDialects, siteBuiltins
	assemblyInfo, builtinPlugins, builtinLayouts, builtinDialects, siteBuiltins = generatedInfo, generatedPlugins, generatedLayouts, generatedDialects, generatedSiteSet
	t.Cleanup(func() {
		assemblyInfo, builtinPlugins, builtinLayouts, builtinDialects, siteBuiltins = oldInfo, oldP, oldL, oldD, oldS
	})
}

// The app's own assembly is a working build: a folder opens, the session has what the file lists
// and the menu follows its features (whatever step of the roadmap the file is at).
func TestGeneratedAssemblyWorks(t *testing.T) {
	useGeneratedBuild(t)
	z, err := newZen(zenOptions{Events: &sink{}, GlobalPath: filepath.Join(t.TempDir(), "config.yml"), Dir: tree(t, docs), Dialogs: &fakeDialogs{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(z.close)
	st := z.Startup()
	if st.Workspace == nil || st.Build == nil || !slices.Equal(st.Build.Features, generatedInfo.Features) {
		t.Fatalf("startup %+v", st)
	}
	p, l, tp := builtinIDs(t, z)
	if !sameIDs(p, generatedInfo.Plugins) || !sameIDs(l, generatedInfo.Layouts) {
		t.Errorf("plugins %v, layouts %v; assembly %+v", p, l, generatedInfo)
	}
	if hasFeature("newsite") && !sameIDs(tp, generatedInfo.Templates) {
		t.Errorf("templates %v; assembly %+v", tp, generatedInfo)
	}
	if hasFeature("reader") {
		if pg, err := z.Page("README.md"); err != nil || pg.HTML == "" {
			t.Errorf("Page: %v", err)
		}
	}
	noGaps(t, menuSpecFor(true, generatedInfo.Features))
}
