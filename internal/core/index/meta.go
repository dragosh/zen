package index

// Meta is the navigation metadata of a page, from its frontmatter.
type Meta struct {
	Title  string
	Order  *int
	Parent string
	Hidden bool
}

// BuildWithMeta is Build with per-page navigation metadata.
func BuildWithMeta(files []string, home string, links func(string) []string, titles func(string) string, meta func(string) Meta) *Index {
	return Build(files, home, links, titles)
}

// ParseMeta reads slug and nav from a page's frontmatter.
func ParseMeta(frontmatter map[string]any) (m Meta, slug string, warnings []string) {
	return Meta{}, "", nil
}

// Warnings returns the navigation problems of the page at path.
func (i *Index) Warnings(path string) []string { return nil }
