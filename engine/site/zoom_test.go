package site

import "testing"

// Image zoom (specs/ui/image-zoom) is in zen.js: a page with an image loads it,
// a page with only text still has no script at all.
func TestPageWithImageLoadsTheRuntime(t *testing.T) {
	f := newFake()
	f.home = "README.md"
	f.page("README.md", "Home", "<p>text only</p>")
	f.page("shots.md", "Shots", `<p><img src="shot.png" alt="A shot"></p>`)
	_, out := build(t, f, Options{})
	contains(t, "image page", read(t, out, "shots.html"), `<script src="_zen/zen.js" defer></script>`)
	// Every page loads zen.js now (the top bar's buttons need it); a text page has no modules.
	contains(t, "text page", read(t, out, "README.html"), `<script src="_zen/zen.js" defer></script>`)
}
