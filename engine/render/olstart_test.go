package render

import "testing"

func TestOrderedListStart(t *testing.T) {
	r := plain()
	src := "1. A\n2. B\n\n```\ncode\n```\n\n3. C\n"
	p := render(t, r, src)
	contains(t, p.HTML, `<ol start="3">`)

	// A list starting at 1 stays a plain <ol>.
	p = render(t, r, "1. A\n2. B\n")
	lacks(t, p.HTML, `start=`)

	// Non-numeric start values from raw HTML are dropped.
	p = render(t, r, "<ol start=\"x\"><li>a</li></ol>\n")
	lacks(t, p.HTML, `start=`)
	p = render(t, r, "<ol start=\"3\" onclick=\"x()\"><li>a</li></ol>\n")
	contains(t, p.HTML, `start="3"`)
	lacks(t, p.HTML, `onclick`)
}
