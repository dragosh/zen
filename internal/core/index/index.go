// Package index builds the sidebar navigation tree and breadcrumbs from the
// links between the markdown files of a workspace. It performs no I/O.
package index

import (
	"path"
	"reflect"
	"sort"
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
	tree   []Node
	crumbs map[string][]Crumb // lower-cased path -> breadcrumb
}

type tnode struct {
	path     string
	children []int
}

// Build builds the navigation tree. links(path) returns the internal link
// targets of a page in document order; titles(path) its title.
func Build(files []string, home string, links func(path string) []string, titles func(path string) string) *Index {
	canon := make(map[string]string, len(files))
	for _, f := range files {
		k := strings.ToLower(f)
		if _, ok := canon[k]; !ok {
			canon[k] = f
		}
	}
	placed := make(map[string]bool, len(files))
	var nodes []tnode
	var parent []int
	var roots []int
	add := func(p string, par int) int {
		placed[strings.ToLower(p)] = true
		nodes = append(nodes, tnode{path: p})
		parent = append(parent, par)
		id := len(nodes) - 1
		if par >= 0 {
			nodes[par].children = append(nodes[par].children, id)
		}
		return id
	}

	var seeds []string
	if h, ok := canon[strings.ToLower(home)]; ok && home != "" {
		seeds = []string{h}
	} else {
		for _, f := range files {
			if !strings.Contains(f, "/") && !placed[strings.ToLower(f)] {
				placed[strings.ToLower(f)] = true
				seeds = append(seeds, f)
			}
		}
		sort.Strings(seeds)
	}
	queue := make([]int, 0, len(files))
	for _, s := range seeds {
		id := add(s, -1)
		roots = append(roots, id)
		queue = append(queue, id)
	}
	for q := 0; q < len(queue); q++ {
		id := queue[q]
		for _, l := range links(nodes[id].path) {
			c, ok := canon[strings.ToLower(l)]
			if !ok || placed[strings.ToLower(c)] {
				continue
			}
			queue = append(queue, add(c, id))
		}
	}

	ix := &Index{crumbs: make(map[string][]Crumb, len(files))}
	titleOf := make([]string, len(nodes))
	for id, n := range nodes {
		titleOf[id] = titles(n.path)
		var bc []Crumb
		if parent[id] >= 0 {
			pc := ix.crumbs[strings.ToLower(nodes[parent[id]].path)]
			bc = make([]Crumb, len(pc), len(pc)+1)
			copy(bc, pc)
		}
		ix.crumbs[strings.ToLower(n.path)] = append(bc, Crumb{titleOf[id], n.path})
	}
	// Convert iteratively in reverse BFS order: children always have larger ids.
	built := make([]Node, len(nodes))
	for id := len(nodes) - 1; id >= 0; id-- {
		n := Node{Title: titleOf[id], Path: nodes[id].path, Kind: "page"}
		for _, c := range nodes[id].children {
			n.Children = append(n.Children, built[c])
		}
		built[id] = n
	}
	for _, r := range roots {
		ix.tree = append(ix.tree, built[r])
	}

	var rest []string
	for _, f := range files {
		k := strings.ToLower(f)
		if !placed[k] && canon[k] == f {
			placed[k] = true
			rest = append(rest, f)
		}
	}
	if len(rest) > 0 {
		sort.Strings(rest)
		other := Node{Title: "Other pages", Kind: "group"}
		for _, f := range rest {
			name := path.Base(f)
			t := strings.TrimSuffix(name, path.Ext(name))
			insert(&other, strings.Split(f, "/"), 0, f, t)
			ix.crumbs[strings.ToLower(f)] = []Crumb{{t, f}}
		}
		ix.tree = append(ix.tree, other)
	}
	return ix
}

func insert(n *Node, parts []string, depth int, full, title string) {
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
	n.Children = append(n.Children, Node{Title: title, Path: full, Kind: "page"})
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
