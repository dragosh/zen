// Package index builds the navigation tree and breadcrumbs from page links.
package index

type Node struct {
	Title, Path, Kind string
	Children          []Node
} // Kind: "page" | "group"
type Crumb struct{ Title, Path string }

type Index struct{}

func Build(files []string, home string, links func(path string) []string, titles func(path string) string) *Index {
	return &Index{}
}
func (i *Index) Tree() []Node                   { return nil }
func (i *Index) Breadcrumb(path string) []Crumb { return nil }
func (i *Index) Equal(other *Index) bool        { return false }
