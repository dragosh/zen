package main

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	tpl := "<string>{{PRODUCT_NAME}}</string><string>{{COPYRIGHT}}</string><string>{{SHORT_VERSION}}</string>"
	got, err := render(tpl, map[string]string{"PRODUCT_NAME": "Zen", "COPYRIGHT": "© <me> & \"co\"", "SHORT_VERSION": "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	want := "<string>Zen</string><string>© &lt;me&gt; &amp; &#34;co&#34;</string><string>1.2.3</string>"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if _, err := render("{{NOPE}}", map[string]string{}); err == nil || !strings.Contains(err.Error(), "NOPE") {
		t.Errorf("unknown placeholder: %v", err)
	}
}
