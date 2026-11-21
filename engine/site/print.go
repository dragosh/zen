package site

import (
	"errors"
	"fmt"
	"html/template"
	"path"
	"sort"
	"strconv"
	"strings"
)

// PrintOptions are what PrintDocument makes.
type PrintOptions struct {
	Drafts  bool
	Section string // "" = all; else this page and the pages under it in the tree
	Title   string // cover title; "" = the home page's title or the folder name
	Date    string // shown on the cover; "" = none (keeps the build deterministic)
	Max     int    // 0 = 500
	// SiteID names the storage plugins and layouts use in the document when it is opened
	// outside the app (print.html); "" is "print".
	SiteID string
	// Scheme is "light" or "dark" to force the colours of print.html on screen (the PDF is always light); "" and
	// "auto" follow the reader's system.
	Scheme string
}

// ErrTooManyPages is returned when the document would have more pages than PrintOptions.Max.
var ErrTooManyPages = errors.New("too many pages to print at once; print a section")

// ErrNotASection is returned for a PrintOptions.Section that is not a page of the sidebar.
var ErrNotASection = errors.New("not a page in this folder")

const defaultMaxPages = 500

// PrintDocument is the whole folder as one document: a cover, a table of
// contents, then every page in sidebar order, each in a section that starts a
// new sheet. It is one HTML document for the app's print preview window and
// for print.html of a build; the scripts of the plugins and layouts in use are
// inlined as in portable mode, the runtime is `_zen/zen.js` next to it.
func PrintDocument(in Input, o PrintOptions) (string, Report, error) {
	names, err := in.Pages()
	if err != nil {
		return "", Report{}, err
	}
	sort.Strings(names)
	tree := in.Tree()
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	order := sidebarOrder(tree, have)
	if o.Section != "" {
		allowed, ok := sectionPages(tree, o.Section)
		if !ok {
			return "", Report{}, fmt.Errorf("%s: %w", shorten(o.Section), ErrNotASection)
		}
		kept := order[:0:0]
		for _, p := range order {
			if allowed[p] {
				kept = append(kept, p)
			}
		}
		order = kept
	}
	max := o.Max
	if max <= 0 {
		max = defaultMaxPages
	}

	warns := map[string]bool{}
	warn := func(p, msg string) {
		if p != "" {
			msg = p + ": " + msg
		} else {
			msg = "site: " + msg
		}
		warns[msg] = true
	}
	var items []printItem
	for _, p := range order {
		pg, err := in.Render(p)
		if err != nil {
			warn(p, err.Error())
			items = append(items, printItem{Page: Page{Path: p, Title: p}, Err: err.Error()})
		} else {
			pg.Path = p
			for _, w := range pg.Warnings {
				warn(p, w)
			}
			if pg.Draft && !o.Drafts {
				continue
			}
			items = append(items, printItem{Page: pg})
		}
		if len(items) > max {
			return "", Report{}, ErrTooManyPages
		}
	}
	pc := &printer{in: in, o: o, warn: warn, keepWS: true, portable: true}
	doc, err := pc.document(items, tree)
	if err != nil {
		return "", Report{}, err
	}
	rep := Report{Pages: len(items)}
	for w := range warns {
		rep.Warnings = append(rep.Warnings, w)
	}
	sort.Strings(rep.Warnings)
	return doc, rep, nil
}

// printItem is one page of the document, or the error that kept it from rendering.
type printItem struct {
	Page
	Err string
}

// printer builds the document of the pages it is given. A build gives it a
// file function (the output path of a copied workspace file); the app keeps
// /_ws/ URLs, which it serves itself.
type printer struct {
	in     Input
	o      PrintOptions
	warn   func(p, msg string)
	keepWS bool
	file   func(p string) (string, error)
	// portable inlines the scripts of plugins and layouts; otherwise they are the files a
	// hosted build wrote (a hosted print.html must not allow file: or blob: in its CSP).
	portable bool
}

// sidebarOrder lists the pages in sidebar order; pages the sidebar does not
// show follow, sorted.
func sidebarOrder(tree []NavNode, have map[string]bool) []string {
	var out []string
	seen := map[string]bool{}
	var walk func([]NavNode)
	walk = func(nodes []NavNode) {
		for _, n := range nodes {
			if n.Kind == "page" && have[n.Path] && !seen[n.Path] {
				seen[n.Path] = true
				out = append(out, n.Path)
			}
			walk(n.Children)
		}
	}
	walk(tree)
	var rest []string
	for p := range have {
		if !seen[p] {
			rest = append(rest, p)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// sectionPages is the page at p in the tree and the pages under it.
func sectionPages(tree []NavNode, p string) (map[string]bool, bool) {
	var find func([]NavNode) *NavNode
	find = func(nodes []NavNode) *NavNode {
		for i := range nodes {
			if nodes[i].Kind == "page" && nodes[i].Path == p {
				return &nodes[i]
			}
			if n := find(nodes[i].Children); n != nil {
				return n
			}
		}
		return nil
	}
	n := find(tree)
	if n == nil {
		return nil, false
	}
	out := map[string]bool{}
	var add func(NavNode)
	add = func(n NavNode) {
		if n.Kind == "page" {
			out[n.Path] = true
		}
		for _, c := range n.Children {
			add(c)
		}
	}
	add(*n)
	return out, true
}

type tocItem struct {
	Title, Href string
	Children    []tocItem
}

type printSection struct {
	ID, Path, Class, Layout, Lang, Crumb, Err string
	Title                                     string
	Body                                      template.HTML
	JS                                        string // the page's scripts as JSON, for a runtime per page
}

type printData struct {
	CSP, Title, Meta, Date string
	Theme                  string // data-theme of <html>: light or dark, "" for the reader's system
	Toc                    []tocItem
	Sections               []printSection
	Styles                 []template.CSS
	Data                   template.HTML
	Script                 bool
}

// document assembles the cover, the contents and the sections.
func (c *printer) document(items []printItem, tree []NavNode) (string, error) {
	n := map[string]int{} // page path -> section number
	for i, it := range items {
		n[it.Path] = i + 1
	}
	sectionID := func(i int) string { return "zen-print-" + strconv.Itoa(i) }

	var secs []printSection
	for i, it := range items {
		sec := printSection{ID: sectionID(i + 1), Path: it.Path, Title: it.Title, Crumb: crumbLabel(it.Page)}
		if l := it.Layout; l != "" && l != "article" && layoutIDRE.MatchString(l) {
			sec.Class, sec.Layout = " zen-layout-"+l, l
		}
		if langRE.MatchString(it.Lang) {
			sec.Lang = it.Lang
		}
		if it.Err != "" {
			sec.Err = it.Err
			secs = append(secs, sec)
			continue
		}
		id := sec.ID
		rw := &rewriter{
			from: it.Path, edit: "", slug: c.in.Slug,
			page: func(p string) (string, bool) {
				if k, ok := n[p]; ok {
					return sectionID(k), true
				}
				return "", false
			},
			file: c.fileFor(it.Path),
			print: &printMode{prefix: id, keepWS: c.keepWS, section: func(p string) (string, bool) {
				if k, ok := n[p]; ok {
					return sectionID(k), true
				}
				return "", false
			}},
		}
		sec.Body = template.HTML(rw.html(it.HTML))
		for _, w := range rw.warns {
			c.warn(it.Path, w)
		}
		secs = append(secs, sec)
	}

	// Styles and scripts of the plugins and layouts in use, each once.
	var styles []template.CSS
	cssURLs := []string{}
	var js []string
	seen := map[string]bool{}
	modules := map[string]map[string]string{}
	rejected := map[string]bool{}
	for i, it := range items {
		if it.Err != "" {
			continue
		}
		var pageJS []string
		for _, u := range it.CSS {
			a, ok := parseAssetURL(u)
			if !ok {
				c.warn(it.Path, "stylesheet "+shorten(u)+" is not a plugin or layout file; left out")
				continue
			}
			if seen[a.url] {
				continue
			}
			seen[a.url] = true
			src, err := c.readAsset(a)
			if err != nil {
				c.warn(it.Path, a.url+": "+err.Error()+"; left out")
				continue
			}
			styles = append(styles, template.CSS(strings.ReplaceAll(string(src), "</style", `<\/style`)))
			cssURLs = append(cssURLs, a.url)
		}
		for _, u := range it.JS {
			a, ok := parseAssetURL(u)
			if !ok {
				c.warn(it.Path, "script "+shorten(u)+" is not a plugin or layout file; left out")
				continue
			}
			pageJS = append(pageJS, a.url)
			if seen[a.url] {
				continue
			}
			seen[a.url] = true
			src, err := c.readAsset(a)
			if err != nil {
				c.warn(it.Path, a.url+": "+err.Error()+"; left out")
				continue
			}
			if c.portable {
				if usesImports(src) {
					rejected[a.module()] = true
					c.warn(it.Path, a.url+": uses imports; not supported in portable mode; left out")
					continue
				}
				if modules[a.module()] == nil {
					modules[a.module()] = map[string]string{}
				}
				modules[a.module()][a.file] = string(src)
			}
			js = append(js, a.url)
		}
		if len(pageJS) > 0 {
			b, err := jsonText(pageJS)
			if err != nil {
				return "", err
			}
			secs[i].JS = b
		}
	}
	if len(rejected) > 0 {
		kept := js[:0]
		for _, u := range js {
			if a, _ := parseAssetURL(u); !rejected[a.module()] {
				kept = append(kept, u)
			}
		}
		js = kept
		for m := range rejected {
			delete(modules, m)
		}
	}

	d := printData{CSP: csp(c.portable), Theme: themeAttr(c.o.Scheme), Sections: secs, Styles: styles, Toc: tocOf(tree, n, sectionID)}
	d.Title = c.coverTitle(items)
	d.Date = c.o.Date
	switch len(items) {
	case 1:
		d.Meta = "1 page"
	default:
		d.Meta = strconv.Itoa(len(items)) + " pages"
	}
	if len(js) > 0 {
		sort.Strings(js)
		siteID := c.o.SiteID
		if siteID == "" {
			siteID = "print"
		}
		data, err := jsonScript(`id="zen-page"`, pageData{Path: "print", SiteID: siteID, Root: "", Assets: pageAsset{CSS: cssURLs, JS: js}, Portable: c.portable})
		if err != nil {
			return "", err
		}
		d.Data += data
		for _, m := range sortedModules(modules) {
			mod, err := jsonScript(`data-zen-module="`+m.key+`"`, m.files)
			if err != nil {
				return "", err
			}
			d.Data += mod
		}
		d.Script = true
	}
	out, err := render(printTmpl, d)
	return string(out), err
}

func (c *printer) fileFor(from string) func(string) (string, error) {
	if c.file == nil {
		return func(p string) (string, error) { return p, nil }
	}
	return c.file
}

func (c *printer) readAsset(a assetRef) ([]byte, error) {
	f, err := c.in.Asset(a.url)
	if err != nil {
		return nil, errors.New("not found")
	}
	return readAll(f)
}

// coverTitle is the title option, else the home page's title, else the folder's name.
func (c *printer) coverTitle(items []printItem) string {
	if c.o.Title != "" {
		return c.o.Title
	}
	if h := c.in.Home(); h != "" {
		for _, it := range items {
			if it.Path == h && strings.TrimSpace(it.Title) != "" {
				return it.Title
			}
		}
	}
	if r, ok := c.in.(rooter); ok {
		if b := path.Base(strings.ReplaceAll(r.Root(), "\\", "/")); b != "" && b != "." && b != "/" {
			return b
		}
	}
	return "Documentation"
}

// crumbLabel is the running label above a page's title: the pages above it, or its file.
func crumbLabel(p Page) string {
	cs := p.Breadcrumb
	if n := len(cs); n > 0 && cs[n-1].Path == p.Path {
		cs = cs[:n-1]
	}
	var parts []string
	for _, c := range cs {
		if strings.TrimSpace(c.Title) != "" {
			parts = append(parts, c.Title)
		}
	}
	if len(parts) == 0 {
		return p.Path
	}
	return strings.Join(parts, " › ")
}

// tocOf is the sidebar restricted to the pages in the document, indented as it is.
func tocOf(nodes []NavNode, n map[string]int, id func(int) string) []tocItem {
	var out []tocItem
	listed := map[string]bool{}
	var walk func([]NavNode) []tocItem
	walk = func(nodes []NavNode) []tocItem {
		var res []tocItem
		for _, nd := range nodes {
			kids := walk(nd.Children)
			it := tocItem{Title: nd.Title, Children: kids}
			if nd.Kind == "page" {
				if k, ok := n[nd.Path]; ok && !listed[nd.Path] {
					listed[nd.Path] = true
					it.Href = "#" + id(k)
				} else if len(kids) == 0 {
					continue
				}
			} else if len(kids) == 0 {
				continue
			}
			res = append(res, it)
		}
		return res
	}
	out = walk(nodes)
	// Pages the sidebar does not show come last, in document order.
	var rest []string
	for p := range n {
		if !listed[p] {
			rest = append(rest, p)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return n[rest[i]] < n[rest[j]] })
	for _, p := range rest {
		out = append(out, tocItem{Title: titleOfPath(p), Href: "#" + id(n[p])})
	}
	return out
}

func titleOfPath(p string) string { return p }

var printTmpl = template.Must(template.New("print").Parse(`{{define "toc"}}<ul>{{range .}}<li>{{if .Href}}<a href="{{.Href}}">{{.Title}}</a>{{else}}<span>{{.Title}}</span>{{end}}{{if .Children}}{{template "toc" .Children}}{{end}}</li>{{end}}</ul>{{end}}` +
	`<!doctype html>
<html lang="en"{{if .Theme}} data-theme="{{.Theme}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="{{.CSP}}">
<title>{{.Title}}</title>
<link rel="stylesheet" href="_zen/zen.css">
{{range .Styles}}<style>{{.}}</style>
{{end}}<style>@page { margin: 18mm 16mm; }</style>
</head>
<body class="zen-site zen-print">
<div class="zen-page zen-print-doc" role="document">
<section class="zen-print-cover" aria-label="Cover">
<h1 class="zen-print-title">{{.Title}}</h1>
<p class="zen-print-meta">{{if .Date}}{{.Date}} · {{end}}{{.Meta}}</p>
</section>
<nav class="zen-print-toc" aria-label="Contents">
<h2>Contents</h2>
{{if .Toc}}{{template "toc" .Toc}}{{else}}<p>No pages</p>{{end}}
</nav>
{{range .Sections}}<section id="{{.ID}}" data-path="{{.Path}}" class="zen-print-page{{if .Err}} zen-print-error{{end}}{{.Class}}"{{if .Layout}} data-layout="{{.Layout}}"{{end}}{{if .Lang}} lang="{{.Lang}}"{{end}}{{if .JS}} data-zen-js="{{.JS}}"{{end}}>
<p class="zen-print-crumb">{{.Crumb}}</p>
{{if .Err}}<h1>{{.Title}}</h1>
<p class="zen-print-failed">This page could not be rendered: {{.Err}}</p>
{{else}}{{.Body}}
{{end}}</section>
{{end}}</div>
{{.Data}}{{if .Script}}<script src="_zen/zen.js" defer></script>
{{end}}</body>
</html>
`))

const printFile = "print.html"

// writePrint writes print.html: the pages that were written, as one document.
// It returns its path, "" when there is none.
func (s *site) writePrint() (string, error) {
	if s.o.NoPrint || len(s.order) == 0 {
		return "", nil
	}
	if len(s.order) > defaultMaxPages {
		s.warn("", fmt.Sprintf("%s left out: more than %d pages; print a section from the app", printFile, defaultMaxPages))
		return "", nil
	}
	have := map[string]bool{}
	for _, p := range s.order {
		have[p] = true
	}
	tree := s.in.Tree()
	var items []printItem
	for _, p := range sidebarOrder(tree, have) {
		items = append(items, printItem{Page: s.pages[p].page})
	}
	pc := &printer{in: s.in, o: PrintOptions{SiteID: s.o.SiteID, Scheme: s.o.Scheme}, warn: s.warn, file: s.copyWorkspace, portable: s.o.Portable}
	doc, err := pc.document(items, tree)
	if err != nil {
		return "", err
	}
	if err := s.w.write(printFile, []byte(doc)); err != nil {
		return "", err
	}
	return printFile, nil
}
