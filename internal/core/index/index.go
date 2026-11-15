// Package index builds the sidebar navigation tree and breadcrumbs from the
// links between the markdown files of a workspace. It performs no I/O.
package index

import (
	"reflect"
	"strings"
)

// Node is a navigation tree node. Kind is "page" or "group".
type Node struct {
	Title, Path, Kind string
	Children          []Node
}

// Crumb is one breadcrumb entry.
type Crumb struct{ Title, Path string }

// Index holds a built navigation tree.
type Index struct {
	tree     []Node
	crumbs   map[string][]Crumb // lower-cased path -> breadcrumb
	warnings map[string][]string
}

// Build builds the navigation tree. links(path) returns the internal link
// targets of a page in document order; titles(path) its title.
func Build(files []string, home string, links func(path string) []string, titles func(path string) string) *Index {
	return BuildWithMeta(files, home, links, titles, nil)
}

// insert places page under n, in groups named after the folders of parts.
func insert(n *Node, parts []string, depth int, page Node) {
	for depth < len(parts)-1 {
		dir := strings.Join(parts[:depth+1], "/")
		var next *Node
		for i := range n.Children {
			if c := &n.Children[i]; c.Kind == "group" && c.Path == dir {
				next = c
			}
		}
		if next == nil {
			n.Children = append(n.Children, Node{Title: parts[depth], Path: dir, Kind: "group"})
			next = &n.Children[len(n.Children)-1]
		}
		n, depth = next, depth+1
	}
	n.Children = append(n.Children, page)
}

// Tree returns the navigation tree.
func (i *Index) Tree() []Node { return i.tree }

// Breadcrumb returns the tree path from the top-level node to path
// (inclusive). Unknown paths return an empty slice.
func (i *Index) Breadcrumb(path string) []Crumb {
	bc := i.crumbs[strings.ToLower(path)]
	if bc == nil {
		return []Crumb{}
	}
	return append([]Crumb(nil), bc...)
}

// Equal reports whether both indexes have deeply equal trees.
func (i *Index) Equal(other *Index) bool {
	if i == nil || other == nil {
		return i == other
	}
	return reflect.DeepEqual(i.tree, other.tree)
}
