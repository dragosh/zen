package app

import (
	"encoding/json"
	"testing"
)

// The JSON the frontend receives is a contract (docs/PLAN.md "Service API").
// Types may move between packages, their JSON may not change.
func TestPayloadShapes(t *testing.T) {
	site := SiteTemplate{ID: "docs", Name: "Docs", Description: "d", Version: "1", Builtin: true, Update: "2", Error: ""}
	site.Ask = append(site.Ask, struct {
		Key     string `json:"key"`
		Label   string `json:"label"`
		Default string `json:"default"`
	}{"title", "Title", "My site"})
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"Startup", Startup{Workspace: &Workspace{Root: "/r", Name: "r", Home: "README.md", Tree: []NavNode{{Title: "Home", Path: "README.md", Kind: "page", Children: []NavNode{}}}}, Page: "README.md", Mode: "read"},
			`{"workspace":{"root":"/r","name":"r","home":"README.md","title":"","logo":"","tree":[{"title":"Home","path":"README.md","kind":"page","children":[]}]},"page":"README.md","mode":"read"}`},
		{"StartupError", Startup{Mode: "read", Error: "boom"},
			`{"workspace":null,"page":"","mode":"read","error":"boom"}`},
		{"Page", Page{Layout: PageLayout{Name: "article", Source: "builtin"}, Path: "a.md", Title: "A", HTML: "<p>x</p>", Frontmatter: map[string]any{"k": 1}, Breadcrumb: []Crumb{{Title: "A", Path: "a.md"}}, Assets: PageAssets{CSS: []string{"/c.css"}, JS: []string{}}, Warnings: []string{}},
			`{"layout":{"name":"article","source":"builtin"},"path":"a.md","title":"A","html":"\u003cp\u003ex\u003c/p\u003e","frontmatter":{"k":1},"breadcrumb":[{"title":"A","path":"a.md"}],"assets":{"css":["/c.css"],"js":[]},"warnings":[]}`},
		{"PluginInfo", PluginInfo{ID: "p", Version: "1", Description: "d", Source: "builtin", Status: "on", Runs: []string{}, Kind: "builtin", Help: PluginHelp{Summary: "s", Snippets: []PluginSnippet{{Title: "t", Markdown: "m"}}}, Blocks: []string{"b"}, Inlines: []string{}, Options: []PluginOption{{Name: "n", Type: "string", Default: "x", Description: "d"}}},
			`{"id":"p","version":"1","description":"d","source":"builtin","status":"on","runs":[],"problem":"","kind":"builtin","help":{"summary":"s","snippets":[{"title":"t","markdown":"m"}]},"blocks":["b"],"inlines":[],"options":[{"name":"n","type":"string","default":"x","description":"d"}]}`},
		{"LayoutInfo", LayoutInfo{ID: "l", Version: "1", Description: "d", Source: "folder", Status: "on", Kind: "layout", HasTemplate: true, Fields: []LayoutField{{Name: "f", Type: "string", Description: "d"}}, Options: []LayoutOption{}, Help: PluginHelp{Snippets: []PluginSnippet{}}},
			`{"id":"l","version":"1","description":"d","source":"folder","status":"on","kind":"layout","hasTemplate":true,"hasScript":false,"fields":[{"name":"f","type":"string","description":"d"}],"options":[],"help":{"summary":"","snippets":[]},"problem":""}`},
		{"BuildOptions", BuildOptions{Out: "/o", Portable: true, Print: true},
			`{"out":"/o","portable":true,"drafts":false,"clean":false,"print":true}`},
		{"BuildResult", BuildResult{Out: "/o", Pages: 2, Files: 5, Warnings: []BuildWarning{{Path: "a.md", Message: "m"}}, Millis: 7},
			`{"out":"/o","pages":2,"files":5,"warnings":[{"path":"a.md","message":"m"}],"millis":7,"cancelled":false}`},
		{"SiteTemplate", site,
			`{"id":"docs","name":"Docs","description":"d","version":"1","ask":[{"key":"title","label":"Title","default":"My site"}],"builtin":true,"update":"2","error":""}`},
		{"OpenProgress", OpenProgress{Name: "n", Phase: "reading", Done: 3},
			`{"name":"n","phase":"reading","done":3,"total":0}`},
		{"BuildProgress", BuildProgress{Done: 1, Total: 2}, `{"done":1,"total":2}`},
		{"PageChanged", PageChanged{Path: "a.md"}, `{"path":"a.md"}`},
		{"LintResult", LintResult{Available: true, Alerts: []LintAlert{{Line: 1, Col: 2, EndCol: 3, Severity: "warning", Check: "c", Message: "m", Match: "x"}}},
			`{"available":true,"alerts":[{"line":1,"col":2,"endCol":3,"severity":"warning","check":"c","message":"m","match":"x","description":"","link":""}]}`},
		{"PluginStreamEvent", PluginStreamEvent{Call: "c", Chunk: "x", Done: true}, `{"call":"c","chunk":"x","done":true,"error":""}`},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if string(b) != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, b, c.want)
		}
	}
}
