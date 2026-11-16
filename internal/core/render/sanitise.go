package render

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/microcosm-cc/bluemonday"
)

var (
	basePolicy = newPolicy(nil)
	policies   sync.Map // allow-rule key -> *bluemonday.Policy (immutable once built)
)

// forbidden elements and attributes no Allow rule can re-enable.
var forbiddenElements = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true, "embed": true,
	"frame": true, "frameset": true, "base": true, "meta": true, "link": true, "form": true,
}

func forbiddenAttr(a string) bool {
	a = strings.ToLower(a)
	return strings.HasPrefix(a, "on") || a == "style" || a == "srcdoc" || a == "formaction"
}

func newPolicy(allow []plugins.AllowRule) *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.RequireNoFollowOnLinks(false)
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6", "li", "sup")
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("code", "pre", "div", "span", "li", "ul", "ol", "section", "sup", "a", "strong", "b", "i")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	p.AllowElements("input", "section")
	p.AllowAttrs("data-zen-link").Matching(regexp.MustCompile(`^(internal|external|broken)$`)).OnElements("a")
	p.AllowAttrs("data-path", "data-anchor").OnElements("a")
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`^noopener noreferrer$`)).OnElements("a")
	name := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	p.AllowAttrs("data-zen-plugin", "data-zen-name", "data-zen-layout").Matching(name).OnElements("div", "span")
	p.AllowAttrs("data-zen-attrs").OnElements("div", "span")
	for _, r := range allow {
		el := strings.ToLower(strings.TrimSpace(r.Element))
		if el == "" || forbiddenElements[el] {
			continue
		}
		p.AllowElements(el)
		var attrs []string
		for _, a := range r.Attrs {
			if a != "" && !forbiddenAttr(a) {
				attrs = append(attrs, a)
			}
		}
		if len(attrs) > 0 {
			p.AllowAttrs(attrs...).OnElements(el)
		}
	}
	return p
}

// policyFor returns the base policy extended by allow, cached per rule set.
func policyFor(allow []plugins.AllowRule) *bluemonday.Policy {
	if len(allow) == 0 {
		return basePolicy
	}
	parts := make([]string, len(allow))
	for i, r := range allow {
		parts[i] = r.Element + ":" + strings.Join(r.Attrs, ",")
	}
	sort.Strings(parts)
	key := strings.Join(parts, ";")
	if p, ok := policies.Load(key); ok {
		return p.(*bluemonday.Policy)
	}
	p, _ := policies.LoadOrStore(key, newPolicy(allow))
	return p.(*bluemonday.Policy)
}

// addInternalHref gives internal links href="#" after sanitising: bluemonday
// drops "#" as an empty URL. Adding a constant attribute cannot introduce
// markup, and the sanitiser has already normalised every tag.
func addInternalHref(s string) string {
	return strings.ReplaceAll(s, `<a data-zen-link="internal"`, `<a href="#" data-zen-link="internal"`)
}
