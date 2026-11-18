package app

import (
	"reflect"
	"sort"
	"testing"
)

// serviceMethods are the methods the frontend calls through the generated
// bindings (frontend/bindings/.../zen.ts). Names and JSON payloads are a
// contract: moving the engine out of this package must not change them.
var serviceMethods = []string{
	"BuildOptions", "BuildWebsite", "CancelBuild", "CancelOpen", "CreatePage", "CreateSite",
	"LayoutStorageGet", "LayoutStorageKeys", "LayoutStorageSet", "LayoutTrust", "Layouts", "Lint", "Logs",
	"OpenBuiltSite", "OpenConfig", "OpenEditor", "OpenExternal", "Page", "PickBuildFolder",
	"PickFolder", "PickSiteFolder", "PluginCancel", "PluginInvoke", "PluginStorageGet",
	"PluginStorageKeys", "PluginStorageSet", "PluginStream", "PluginTrust", "Plugins", "Preview",
	"PrintPage", "PrintSite", "PrintWindow", "RevealBuiltSite", "RevealTemplates", "Save", "SaveBuildDefaults",
	"SiteTemplates", "Source", "Startup", "TemplatesDir",
}

func TestServiceMethodsUnchanged(t *testing.T) {
	typ := reflect.TypeOf(&Zen{})
	var got []string
	for i := 0; i < typ.NumMethod(); i++ {
		got = append(got, typ.Method(i).Name)
	}
	want := append([]string(nil), serviceMethods...)
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the exported methods of Zen changed:\n got %v\nwant %v", got, want)
	}
}
