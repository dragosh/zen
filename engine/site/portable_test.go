package site

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func portableFake(js string) *fake {
	f := newFake()
	f.page("p1.md", "P1", `<div data-zen-plugin="hello">x</div><div data-zen-layout="memo">y</div>`)
	f.with("p1.md", func(p *Page) {
		p.JS = []string{"/_plugins/hello/hello.js?v=abc", "/_layouts/memo/m.js?v=1"}
		p.CSS = []string{"/_plugins/hello/hello.css?v=abc"}
	})
	f.page("sub/p2.md", "P2", "<p>none</p>")
	f.assets["/_plugins/hello/hello.js"] = js
	f.assets["/_plugins/hello/hello.css"] = ".hello{}"
	f.assets["/_layouts/memo/m.js"] = "export default { mount(el) { el.dataset.m = '</script>' } }"
	return f
}

var moduleRE = regexp.MustCompile(`(?s)<script type="application/json" data-zen-module="([^"]+)">(.*?)</script>`)

func TestPortableModules(t *testing.T) {
	src := "export default { mount(el) { el.textContent = 'hi' } }"
	rep, out := build(t, portableFake(src), Options{Portable: true})
	p1 := read(t, out, "p1.html")
	mods := map[string]map[string]string{}
	for _, m := range moduleRE.FindAllStringSubmatch(p1, -1) {
		var files map[string]string
		if err := json.Unmarshal([]byte(m[2]), &files); err != nil {
			t.Fatalf("module %s is not JSON: %v\n%s", m[1], err, m[2])
		}
		mods[m[1]] = files
	}
	if mods["plugin:hello"]["hello.js"] != src {
		t.Errorf("plugin module: %v", mods)
	}
	if mods["layout:memo"]["m.js"] == "" || !strings.Contains(mods["layout:memo"]["m.js"], "</script>") {
		t.Errorf("layout module: %v", mods)
	}
	// A script cannot close its own tag: the text carries no raw "</script>".
	if strings.Count(p1, "</script>") != len(moduleRE.FindAllString(p1, -1))+2 { // modules + page data + zen.js
		t.Errorf("a module text broke out of its tag:\n%s", p1)
	}
	contains(t, "p1", p1, `"portable":true`, `<script src="_zen/zen.js" defer></script>`, `href="_zen/plugins/hello/hello.css"`)
	// Scripts are inline, not files; css is still a file.
	if has(out, "_zen/plugins/hello/hello.js") || has(out, "_zen/layouts/memo/m.js") {
		t.Error("portable build copied script files")
	}
	if !has(out, "_zen/plugins/hello/hello.css") {
		t.Error("css missing")
	}
	// Pages without scripts carry no modules; relative links only.
	p2 := read(t, out, "sub/p2.html")
	lacks(t, "p2", p2, "data-zen-module", "hello")
	for name, body := range listing(t, out) {
		if strings.HasSuffix(name, ".html") {
			lacks(t, name, body, `href="/`, `src="/`, "file://", "http://localhost")
		}
	}
	if len(rep.Warnings) != 0 {
		t.Errorf("warnings %v", rep.Warnings)
	}

	// Hosted: the files are copied and nothing is inlined.
	_, out = build(t, portableFake(src), Options{})
	p1 = read(t, out, "p1.html")
	lacks(t, "hosted", p1, "data-zen-module")
	contains(t, "hosted", p1, `"portable":false`)
	if read(t, out, "_zen/plugins/hello/hello.js") != src {
		t.Error("hosted script not copied")
	}
}

func TestPortableRejectsImports(t *testing.T) {
	for name, src := range map[string]string{
		"static import":  "import x from './lib.js';\nexport default { mount() { x() } }",
		"side effect":    "import './lib.js';\nexport default { mount() {} }",
		"named import":   "import { a } from \"./lib.js\"\nexport default { mount() {} }",
		"dynamic import": "export default { async mount() { await import('./lib.js') } }",
		"re-export":      "export * from './lib.js';\nexport default { mount() {} }",
	} {
		t.Run(name, func(t *testing.T) {
			rep, out := build(t, portableFake(src), Options{Portable: true})
			if !hasWarn(rep, "uses imports; not supported in portable mode", "hello") {
				t.Errorf("no warning: %v", rep.Warnings)
			}
			p1 := read(t, out, "p1.html")
			lacks(t, "p1", p1, `data-zen-module="plugin:hello"`, "lib.js")
			contains(t, "p1", p1, `data-zen-module="layout:memo"`) // the other module still works
			if strings.Contains(p1, "/_plugins/hello/hello.js") {
				t.Errorf("the left-out script is still listed:\n%s", p1)
			}
			// The same script is fine on a hosted site.
			rep, _ = build(t, portableFake(src), Options{})
			if hasWarn(rep, "imports") {
				t.Errorf("hosted build warns: %v", rep.Warnings)
			}
		})
	}
	// Words that only look like imports are fine.
	rep, out := build(t, portableFake("// how to import things\nexport default { mount(el) { el.textContent = 'important: import.meta' } }"), Options{Portable: true})
	if hasWarn(rep, "imports") || !strings.Contains(read(t, out, "p1.html"), `data-zen-module="plugin:hello"`) {
		t.Errorf("false positive: %v", rep.Warnings)
	}
}
