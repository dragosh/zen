package render

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var (
	basePolicy = newPolicy(nil)
	// layoutPolicy is the base policy for a page laid out by a template layout:
	// the same, plus the structural elements and class names a template needs.
	layoutPolicy = newLayoutPolicy()
	policies     sync.Map // allow-rule key -> *bluemonday.Policy (immutable once built)
)

// forbidden elements and attributes no Allow rule can re-enable.
var forbiddenElements = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true, "embed": true,
	"frame": true, "frameset": true, "base": true, "meta": true, "link": true, "form": true,
	// SVG elements that start scripts or load documents from attribute values.
	"animate": true, "animatemotion": true, "animatetransform": true, "set": true, "use": true, "foreignobject": true,
}

// forbiddenAttr: attributes no Allow rule can enable on element el. They carry
// URLs or script, or are the ones zen's own markup depends on.
func forbiddenAttr(el, a string) bool {
	a = strings.ToLower(a)
	switch {
	case strings.HasPrefix(a, "on"), strings.HasPrefix(a, "data-zen-"):
		return true
	case a == "href":
		return el != "a"
	}
	switch a {
	case "style", "srcdoc", "formaction", "values", "from", "to", "by", "srcset", "xlink:href",
		"attributename", "class", "rel", "id":
		return true
	}
	return false
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
	p.AllowAttrs("data-zen-layout-source").Matching(regexp.MustCompile(`^(builtin|folder|user)$`)).OnElements("div")
	for _, r := range allow {
		el := strings.ToLower(strings.TrimSpace(r.Element))
		if el == "" || forbiddenElements[el] {
			continue
		}
		p.AllowElements(el)
		var attrs []string
		for _, a := range r.Attrs {
			if a != "" && !forbiddenAttr(el, a) {
				attrs = append(attrs, a)
			}
		}
		if len(attrs) > 0 {
			p.AllowAttrs(attrs...).OnElements(el)
		}
	}
	return p
}

// newLayoutPolicy extends the base policy for template layouts: page structure
// elements, class names (styling hooks for the layout's CSS) on every element
// a template draws, and the one action attribute the frontend acts on.
func newLayoutPolicy() *bluemonday.Policy {
	p := newPolicy(nil)
	p.AllowElements("header", "footer", "nav", "article", "aside", "figure", "figcaption", "time", "address", "hgroup")
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements(
		"header", "footer", "nav", "article", "aside", "figure", "figcaption", "time", "address", "hgroup",
		"p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "blockquote", "table", "td", "th", "tr", "em", "small", "dl", "dt", "dd")
	p.AllowAttrs("datetime").Matching(regexp.MustCompile(`^[0-9T:+\-. Z]{1,40}$`)).OnElements("time")
	p.AllowAttrs("aria-label").Matching(regexp.MustCompile(`^[^<>]{1,80}$`)).OnElements("nav", "a", "section", "aside", "div")
	p.AllowAttrs("data-zen-action").Matching(regexp.MustCompile(`^(edit|print)$`)).OnElements("a")
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

// scopedSanitise sanitises the page: everything with the base policy, and the
// content of each plugin wrapper (an element with data-zen-plugin="<id>")
// with the base policy plus that plugin's own allow rules, so a rule never
// widens what the rest of the page may contain. The wrapper's content is
// sanitised on its own and put back after the page has been sanitised; a
// wrapper that is not closed is left to the base policy.
func scopedSanitise(in string, rules map[string][]plugins.AllowRule) string {
	return scopedSanitiseWith(in, rules, basePolicy)
}

// scopedSanitiseWith is scopedSanitise with base as the policy of the page.
func scopedSanitiseWith(in string, rules map[string][]plugins.AllowRule, base *bluemonday.Policy) string {
	if len(rules) == 0 || !strings.Contains(in, "data-zen-plugin") {
		return base.Sanitize(in)
	}
	sc := &scoper{rules: rules, prefix: "zensan" + newNonce() + "n"}
	out := base.Sanitize(sc.scope(in))
	for round := 0; round <= len(sc.frags) && strings.Contains(out, sc.prefix); round++ {
		pairs := make([]string, 0, 2*len(sc.frags))
		for i, f := range sc.frags {
			pairs = append(pairs, sc.token(i), f)
		}
		out = strings.NewReplacer(pairs...).Replace(out)
	}
	return out
}

type scoper struct {
	rules  map[string][]plugins.AllowRule
	prefix string
	frags  []string // sanitised content of the wrappers, innermost first
}

func (sc *scoper) token(i int) string { return fmt.Sprintf("%s%dz", sc.prefix, i) }

// scope returns in with the content of every wrapper of a plugin that has
// rules replaced by a token for its sanitised content.
func (sc *scoper) scope(in string) string {
	z := html.NewTokenizer(strings.NewReader(in))
	var out strings.Builder
	pos, last := 0, 0 // last: what of in is already in out
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			out.WriteString(in[last:])
			return out.String()
		}
		pos += len(z.Raw())
		if tt != html.StartTagToken {
			continue
		}
		name, hasAttr := z.TagName()
		tag := string(name)
		if tag != "div" && tag != "span" {
			continue
		}
		var rs []plugins.AllowRule
		for hasAttr {
			var k, v []byte
			k, v, hasAttr = z.TagAttr()
			if string(k) == "data-zen-plugin" {
				rs = sc.rules[string(v)]
			}
		}
		if len(rs) == 0 {
			continue
		}
		innerStart, depth := pos, 1
		innerEnd := -1
		for depth > 0 {
			t2 := z.Next()
			if t2 == html.ErrorToken {
				break
			}
			rawStart := pos
			pos += len(z.Raw())
			switch t2 {
			case html.StartTagToken, html.EndTagToken:
				if n, _ := z.TagName(); string(n) == tag {
					if t2 == html.StartTagToken {
						depth++
					} else if depth--; depth == 0 {
						innerEnd = rawStart
					}
				}
			}
		}
		if innerEnd < 0 { // not closed: the rest is sanitised by the base policy
			out.WriteString(in[last:])
			return out.String()
		}
		inner := sc.scope(in[innerStart:innerEnd])
		sc.frags = append(sc.frags, policyFor(rs).Sanitize(inner))
		out.WriteString(in[last:innerStart])
		out.WriteString(sc.token(len(sc.frags) - 1))
		last = innerEnd
	}
}

// addInternalHref gives internal links href="#" after sanitising: bluemonday
// drops "#" as an empty URL. Adding a constant attribute cannot introduce
// markup, and the sanitiser has already normalised every tag.
func addInternalHref(s string) string {
	return strings.ReplaceAll(s, `<a data-zen-link="internal"`, `<a href="#" data-zen-link="internal"`)
}
