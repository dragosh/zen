package session

import (
	"net/url"
	"sort"
	"strings"

	"github.com/zenmark-org/zenmd/engine/workspace"
)

// slugResolver resolves links like the workspace does and, for links that
// would otherwise be broken, by frontmatter slug. It is immutable: reindex
// builds a new one when the slugs change.
type slugResolver struct {
	ws    *workspace.Workspace
	slugs map[string]string // slug -> workspace path
}

func (r *slugResolver) ResolveLink(from, href string) workspace.Link {
	l := r.ws.ResolveLink(from, href)
	if l.Kind != "broken" || len(r.slugs) == 0 {
		return l
	}
	target, _, _ := strings.Cut(href, "#")
	target, _, _ = strings.Cut(target, "?")
	if t, err := url.PathUnescape(target); err == nil {
		target = t
	}
	if p, ok := r.slugs[strings.TrimPrefix(target, "/")]; ok {
		return workspace.Link{Path: p, Anchor: l.Anchor, Kind: "internal"}
	}
	return l
}

// buildSlugs maps each slug to its page. slugOf holds the valid slug of each
// page; when several pages claim one, the first by path keeps it and the
// others get a warning (keyed by path).
func buildSlugs(slugOf map[string]string) (map[string]string, map[string][]string) {
	paths := make([]string, 0, len(slugOf))
	for p := range slugOf {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	slugs := make(map[string]string, len(paths))
	warns := map[string][]string{}
	for _, p := range paths {
		s := slugOf[p]
		if first, ok := slugs[s]; ok {
			warns[p] = append(warns[p], "frontmatter: slug "+s+" is already used by "+first)
			continue
		}
		slugs[s] = p
	}
	return slugs, warns
}
