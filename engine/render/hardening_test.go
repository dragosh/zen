package render

import (
	"strings"
	"testing"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

// svgPlugin renders an svg with URL-carrying parts and allows whatever it asks.
type svgPlugin struct {
	m   plugins.Manifest
	out string
}

func (s svgPlugin) Manifest() plugins.Manifest { return s.m }
func (s svgPlugin) RenderBlock(plugins.Context, plugins.Block) (string, error) {
	return s.out, nil
}

// M7: a plugin's allow rules cannot allow script-carrying parts, and the
// rules are applied only inside that plugin's own wrapper (the reviewer's probe).
func TestM7AllowRulesAreScopedAndCannotAllowDangerousParts(t *testing.T) {
	m := xm("p", []string{"p"}, nil, nil)
	m.Allow = []plugins.AllowRule{{Element: "svg", Attrs: []string{"viewBox"}}, {Element: "a", Attrs: []string{"href"}},
		{Element: "animate", Attrs: []string{"attributeName", "values", "href"}}, {Element: "set"}, {Element: "use", Attrs: []string{"href", "xlink:href"}},
		{Element: "foreignObject"}, {Element: "div", Attrs: []string{"data-zen-plugin", "class", "id", "rel", "style", "onclick"}},
		{Element: "img", Attrs: []string{"srcset", "from", "to", "by"}}}
	inner := `<svg viewBox="0 0 1 1"><a href="https://x.test/"><animate attributeName="href" values="javascript:alert(1)"/><text>x</text></a>` +
		`<use xlink:href="data:image/svg+xml,AAA"/><set to="1"/><foreignObject><b>f</b></foreignObject></svg>` +
		`<img src="https://a/b.png" srcset="javascript:alert(1) 1x, http://evil/x 2x" from="1" to="2" by="3">`
	src := "```p\nx\n```\n\n<svg viewBox=\"0 0 2 2\"><text>outside</text></svg>\n\n<div data-zen-plugin=\"Bad Value!\" id=\"q\" onclick=\"x\" style=\"a:b\">y</div>\n"
	out := xrender(t, src, svgPlugin{m, inner}).HTML
	for _, bad := range []string{"animate", "<use", "xlink:href", "srcset", "<set", "foreignObject", "values=", "javascript:", "onclick", "style=", " from=", " to=", " by=", "Bad Value"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q survived: %s", bad, out)
		}
	}
	if !strings.Contains(out, `<svg viewbox="0 0 1 1">`) {
		t.Errorf("the plugin's own svg was removed: %s", out)
	}
	if strings.Contains(out, `viewbox="0 0 2 2"`) {
		t.Errorf("the rule applied outside the plugin's wrapper: %s", out)
	}
}

// M7: the attribute-injection probe stays inert.
func TestM7AttrsJSONInjectionIsEscaped(t *testing.T) {
	out := xrender(t, "<div data-zen-plugin=\"x\" data-zen-attrs='{\"a\":\"\\u003c/div\\u003e\\\" onclick=x'>t</div>\n").HTML
	if strings.Contains(out, " onclick") && !strings.Contains(out, "&#34; onclick") && !strings.Contains(out, "&quot; onclick") {
		t.Errorf("injected attribute: %s", out)
	}
}

// M7: rules of nested wrappers apply to their own content only; an unclosed
// wrapper gets no extra rules.
func TestM7NestedScopes(t *testing.T) {
	rules := map[string][]plugins.AllowRule{"p": {{Element: "svg"}}, "q": {{Element: "canvas", Attrs: []string{"width"}}}}
	in := `<div data-zen-plugin="p" data-zen-name="p"><svg></svg><canvas width="1"></canvas>` +
		`<div data-zen-plugin="q" data-zen-name="q"><canvas width="2"></canvas><svg></svg></div></div><svg></svg>`
	out := scopedSanitise(in, rules)
	want := `<div data-zen-plugin="p" data-zen-name="p"><svg></svg><div data-zen-plugin="q" data-zen-name="q"><canvas width="2"></canvas></div></div>`
	if out != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
	if got := scopedSanitise(`<div data-zen-plugin="p" data-zen-name="p"><div><svg></svg>`, rules); strings.Contains(got, "<svg") {
		t.Errorf("an unclosed wrapper widened the policy: %s", got)
	}
}
