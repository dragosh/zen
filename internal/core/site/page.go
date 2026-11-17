package site

import (
	"html/template"
	"sort"
	"strings"
)

// writePage writes one page: the rewritten body in the shell, plus the
// plugin and layout files it uses.
func (s *site) writePage(b *built) error {
	pg := b.page
	rw := &rewriter{
		from: pg.Path, fromOut: b.out, edit: s.o.EditURL,
		page: func(p string) (string, bool) {
			if x, ok := s.pages[p]; ok {
				return x.out, true
			}
			return "", false
		},
		slug: s.in.Slug,
		file: s.copyWorkspace,
	}
	body := rw.html(pg.HTML)
	for _, w := range rw.warns {
		s.warn(pg.Path, w)
	}
	root := rootPrefix(b.out)

	// Plugin and layout files the page uses.
	css := []string{root + zenDir + "/zen.css"}
	seen := map[string]bool{}
	for _, u := range pg.CSS {
		a, ok := parseAssetURL(u)
		if !ok {
			s.warn(pg.Path, "stylesheet "+shorten(u)+" is not a plugin or layout file; left out")
			continue
		}
		if seen[a.url] {
			continue
		}
		seen[a.url] = true
		if err := s.copyAsset(a); err != nil {
			s.warn(pg.Path, a.url+": "+err.Error()+"; left out")
			continue
		}
		css = append(css, escapePath(root+a.outFile()))
	}

	var js []string
	modules := map[string]map[string]string{} // portable: module -> file -> text
	rejected := map[string]bool{}
	for _, u := range pg.JS {
		a, ok := parseAssetURL(u)
		if !ok {
			s.warn(pg.Path, "script "+shorten(u)+" is not a plugin or layout file; left out")
			continue
		}
		if seen[a.url] {
			continue
		}
		seen[a.url] = true
		if s.o.Portable {
			src, err := s.readAsset(a)
			if err != nil {
				s.warn(pg.Path, a.url+": "+err.Error()+"; left out")
				continue
			}
			if usesImports(src) {
				rejected[a.module()] = true
				s.warn(pg.Path, a.url+": uses imports; not supported in portable mode; left out")
				continue
			}
			if modules[a.module()] == nil {
				modules[a.module()] = map[string]string{}
			}
			modules[a.module()][a.file] = string(src)
		} else if err := s.copyAsset(a); err != nil {
			s.warn(pg.Path, a.url+": "+err.Error()+"; left out")
			continue
		}
		js = append(js, a.url)
	}
	// A module with one script that imports is left out as a whole.
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

	var data template.HTML
	script := ""
	if len(js) > 0 || rw.needsJS {
		cssURLs := []string{}
		for _, u := range pg.CSS {
			if a, ok := parseAssetURL(u); ok {
				cssURLs = append(cssURLs, a.url)
			}
		}
		sort.Strings(js)
		if js == nil {
			js = []string{}
		}
		d, err := jsonScript(`id="zen-page"`, pageData{Path: pg.Path, SiteID: s.o.SiteID, Root: root, Assets: pageAsset{CSS: cssURLs, JS: js}, Portable: s.o.Portable})
		if err != nil {
			return err
		}
		data += d
		for _, m := range sortedModules(modules) {
			mod, err := jsonScript(`data-zen-module="`+m.key+`"`, m.files)
			if err != nil {
				return err
			}
			data += mod
		}
		script = root + zenDir + "/zen.js"
	}

	article := "zen-page"
	layout := ""
	if l := pg.Layout; l != "" && l != "article" && layoutIDRE.MatchString(l) {
		article += " zen-layout-" + l
		layout = l
	}
	lang := "en"
	if langRE.MatchString(pg.Lang) {
		lang = pg.Lang
	}
	var crumbs []crumbItem
	if len(pg.Breadcrumb) > 1 {
		for _, c := range pg.Breadcrumb {
			it := crumbItem{Title: c.Title}
			if x, ok := s.pages[c.Path]; ok && c.Path != pg.Path {
				it.Href = escapePath(relFile(b.out, x.out))
			}
			crumbs = append(crumbs, it)
		}
	}
	title := pg.Title
	if strings.TrimSpace(title) == "" {
		title = pg.Path
	}
	out, err := render(shellTmpl, shellData{
		Lang: lang, Title: title, CSP: csp(s.o.Portable), CSS: css,
		Items:  s.navItems(s.in.Tree(), pg.Path, b.out),
		Crumbs: crumbs, ArticleClass: article, Layout: layout,
		Body: template.HTML(body), Data: data, Script: script,
	})
	if err != nil {
		return err
	}
	if err := s.w.write(b.out, out); err != nil {
		return err
	}
	if b.out == "index.html" {
		s.hasIndex = true
	}
	if pg.Path == s.home && !strings.Contains(b.out, "/") && b.out != "index.html" {
		s.hasIndex = true
		return s.w.write("index.html", out)
	}
	return nil
}
