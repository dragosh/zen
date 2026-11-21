package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
)

// csp is the Content-Security-Policy of every page: nothing but the site's
// own files and https images. Portable pages open from file://, where some
// browsers (Firefox, Safari) do not match 'self' to local files, so file: is
// listed too, plus blob: for the module loader.
func csp(portable bool) string {
	self := "'self'"
	script := "script-src 'self'"
	if portable {
		self = "'self' file:"
		script = "script-src 'self' file: blob:"
	}
	return "default-src " + self + "; " + script + "; style-src " + self + " 'unsafe-inline'; img-src " + self + " https:; font-src " + self + "; object-src 'none'; base-uri 'none'"
}

type navItem struct {
	Title    string
	Href     string
	Current  bool
	Open     bool
	Children []navItem
}

type crumbItem struct{ Title, Href string }

type shellData struct {
	Lang, Title, CSP     string
	CSS                  []string
	Items                []navItem
	Crumbs               []crumbItem
	ArticleClass, Layout string
	Body                 template.HTML
	Data                 template.HTML // the page data and portable modules
	Script               string
}

var layoutIDRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var langRE = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

var shellTmpl = template.Must(template.New("shell").Parse(`{{define "node"}}{{if .Children}}<li><details class="zen-site-group"{{if .Open}} open{{end}}><summary>{{if .Href}}<a href="{{.Href}}"{{if .Current}} aria-current="page"{{end}}>{{.Title}}</a>{{else}}{{.Title}}{{end}}</summary><ul>{{range .Children}}{{template "node" .}}{{end}}</ul></details></li>{{else}}<li>{{if .Href}}<a href="{{.Href}}"{{if .Current}} aria-current="page"{{end}}>{{.Title}}</a>{{else}}<span>{{.Title}}</span>{{end}}</li>{{end}}{{end}}` +
	`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="{{.CSP}}">
<title>{{.Title}}</title>
{{range .CSS}}<link rel="stylesheet" href="{{.}}">
{{end}}</head>
<body class="zen-site">
<a class="zen-skip" href="#main">Skip to content</a>
<div class="zen-site-frame">
{{if .Items}}<aside class="zen-site-side" id="zen-site-side"><details class="zen-site-menu"><summary>Contents</summary><nav aria-label="Pages" class="zen-site-nav"><ul>{{range .Items}}{{template "node" .}}{{end}}</ul></nav></details></aside>
{{end}}<div class="zen-site-body">
<header class="zen-top zen-site-top"><div class="zen-top-controls">{{if .Items}}<button type="button" class="ghost zen-site-sidebar-toggle" aria-label="Hide sidebar" title="Hide sidebar (⌘\)" aria-expanded="true" aria-controls="zen-site-side" hidden></button>{{end}}<button type="button" class="ghost zen-site-back" aria-label="Back" title="Back (⌘[)" disabled hidden>‹</button><button type="button" class="ghost zen-site-forward" aria-label="Forward" title="Forward (⌘])" disabled hidden>›</button></div>{{if .Crumbs}}<nav aria-label="Breadcrumb" class="zen-site-crumbs"><ol>{{range .Crumbs}}<li>{{if .Href}}<a href="{{.Href}}">{{.Title}}</a>{{else}}{{.Title}}{{end}}</li>{{end}}</ol></nav>{{end}}</header>
<main id="main">
<article class="{{.ArticleClass}}"{{if .Layout}} data-layout="{{.Layout}}"{{end}}>{{.Body}}</article>
</main>
</div>
</div>
{{.Data}}{{if .Script}}<script src="{{.Script}}" defer></script>
{{end}}</body>
</html>
`))

var plainTmpl = template.Must(template.New("plain").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="{{.CSP}}">
{{if .Redirect}}<meta http-equiv="refresh" content="0; url={{.Redirect}}">
{{end}}<title>{{.Title}}</title>
<link rel="stylesheet" href="_zen/zen.css">
</head>
<body class="zen-site">
<div class="zen-site-frame"><div class="zen-site-body">
<main id="main">
<article class="zen-page">{{if .Redirect}}<p><a href="{{.Redirect}}">{{.Title}}</a></p>{{else}}<h1>No pages yet</h1>
<p>This folder has no markdown pages to show.</p>{{end}}</article>
</main>
</div></div>
</body>
</html>
`))

// jsonScript is a <script type="application/json"> element. The text is
// escaped by encoding/json (<, > and & become \u00xx), so it cannot close the
// tag.
func jsonScript(attrs string, v any) (template.HTML, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return template.HTML(fmt.Sprintf(`<script type="application/json" %s>%s</script>`+"\n", attrs, b)), nil
}

// pageData is the page's `zen-page` element, read by the static runtime.
type pageData struct {
	Path     string    `json:"path"`
	SiteID   string    `json:"siteId"`
	Root     string    `json:"root"`
	Assets   pageAsset `json:"assets"`
	Portable bool      `json:"portable"`
}

type pageAsset struct {
	CSS []string `json:"css"`
	JS  []string `json:"js"`
}

// navItems builds the sidebar for the page written to fromOut.
func (s *site) navItems(nodes []NavNode, current, fromOut string) []navItem {
	var out []navItem
	for _, n := range nodes {
		children := s.navItems(n.Children, current, fromOut)
		it := navItem{Title: n.Title, Children: children}
		if n.Kind == "page" {
			if b, ok := s.pages[n.Path]; ok {
				it.Href = escapePath(relFile(fromOut, b.out))
				it.Current = n.Path == current
			} else if len(children) == 0 {
				continue // left out (draft, excluded): not in the sidebar
			}
		} else if len(children) == 0 {
			continue
		}
		it.Open = it.Current
		for _, c := range children {
			it.Open = it.Open || c.Open || c.Current
		}
		out = append(out, it)
	}
	return out
}

func render(t *template.Template, data any) ([]byte, error) {
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (s *site) writeIndex(home string) error {
	if s.hasIndex {
		return nil // a root index.md, or the root home written as index.html
	}
	data := struct {
		CSP, Title, Redirect string
	}{CSP: csp(s.o.Portable), Title: "Home"}
	target := ""
	if b, ok := s.pages[home]; ok {
		target = b.out
	} else if len(s.order) > 0 {
		target = s.pageFirst()
	}
	if target != "" {
		data.Redirect = escapePath(target)
		for _, b := range s.pages {
			if b.out == target {
				data.Title = b.page.Title
			}
		}
	}
	out, err := render(plainTmpl, data)
	if err != nil {
		return err
	}
	return s.w.write("index.html", out)
}

// pageFirst is the output of the first page of the sidebar (or of the sort order).
func (s *site) pageFirst() string {
	var first func(nodes []NavNode) string
	first = func(nodes []NavNode) string {
		for _, n := range nodes {
			if b, ok := s.pages[n.Path]; ok && n.Kind == "page" {
				return b.out
			}
			if f := first(n.Children); f != "" {
				return f
			}
		}
		return ""
	}
	if f := first(s.in.Tree()); f != "" {
		return f
	}
	return s.pages[s.order[0]].out
}

// jsonText is v as JSON text with <, > and & escaped (encoding/json does it).
func jsonText(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}
