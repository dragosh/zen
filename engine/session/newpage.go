package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/dragosh/zenmd/engine/layouts"
)

// errPageExists is returned by CreatePage when the file is already there.
var errPageExists = errors.New("a page with that name already exists")

// CreatePage creates a markdown page from the template of layout ("" or
// "article" for a plain page, or the id of any layout the folder can use: its
// first help snippet is the starting text) and returns its workspace path. ".md" is
// added when name has no extension; missing folders are created inside the
// workspace. An existing file is never overwritten.
func (s *Session) CreatePage(name, layout string) (string, error) {
	rel, err := newPagePath(name)
	if err != nil {
		return "", err
	}
	text, ok := s.newPageText(layout, titleOf(rel))
	if !ok {
		return "", fmt.Errorf("unknown layout %q", layout)
	}
	return rel, s.create(rel, text)
}

// newPagePath cleans a page name typed by the user into a workspace path.
func newPagePath(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, `\`, "/"))
	name = strings.TrimLeft(name, "/")
	if name == "" || strings.HasSuffix(name, "/") {
		return "", errors.New("give the page a name")
	}
	if strings.ContainsRune(name, 0) {
		return "", errors.New("the name contains a NUL byte")
	}
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".md", ".markdown":
	case "":
		name += ".md"
	default:
		return "", fmt.Errorf("pages end in .md, not %s", ext)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("the page must be inside the open folder")
	}
	for _, part := range strings.Split(clean, "/") {
		if strings.HasPrefix(part, ".") {
			return "", errors.New("names starting with a dot are hidden; pick another")
		}
	}
	return clean, nil
}

// titleOf turns "notes/my-first_deck.md" into "My first deck".
func titleOf(rel string) string {
	base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	t := strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(base))
	if t == "" {
		return "Untitled"
	}
	r := []rune(t)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// newPageText is the starting text of a new page in layout; false when the
// folder has no such layout.
func (s *Session) newPageText(layout, title string) (string, bool) {
	if layout == "" || layout == "article" {
		return pageTemplate("", title), true
	}
	s.mu.Lock()
	var l *layouts.Layout
	if s.lreg != nil {
		l = s.lreg.Get(layout)
	}
	s.mu.Unlock()
	if l == nil {
		return "", false
	}
	switch layout {
	case "slides", "course":
		if l.Source() == layouts.SourceBuiltin {
			return pageTemplate(layout, title), true
		}
	}
	if sn := l.Manifest().Help.Snippets; len(sn) > 0 {
		return snippetPage(sn[0].Markdown, title), true
	}
	return "---\ntitle: " + yamlString(title) + "\nlayout: " + layout + "\n---\n# " + title + "\n\n", true
}

// snippetPage turns a help snippet into a new page: its frontmatter gets the
// page's title and its first heading is replaced by it.
func snippetPage(md, title string) string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	rest := md
	var front []string
	if strings.HasPrefix(md, "---\n") {
		if end := strings.Index(md[4:], "\n---\n"); end >= 0 {
			front = strings.Split(md[4:4+end], "\n")
			rest = md[4+end+5:]
		}
	}
	var kept []string
	for _, l := range front {
		if !strings.HasPrefix(l, "title:") {
			kept = append(kept, l)
		}
	}
	kept = append([]string{"title: " + yamlString(title)}, kept...)
	lines := strings.Split(rest, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if strings.HasPrefix(l, "# ") {
			lines[i] = "# " + title
		}
		break
	}
	return "---\n" + strings.Join(kept, "\n") + "\n---\n" + strings.Join(lines, "\n")
}

// pageTemplate is the starting text of a new page (a plain page, or the
// slides and course decks, which have their own wording).
func pageTemplate(layout, title string) string {
	switch layout {
	case "slides":
		return "---\ntitle: " + yamlString(title) + "\nlayout: slides\nlayoutOptions: { ratio: \"16:9\" }\n---\n" +
			"# " + title + "\n\nA subtitle or your name.\n\n---\n\n" +
			"## First point\n\n- One idea per slide\n- Separate slides with a line of three dashes\n\nNote: Speaker notes start with \"Note:\" and stay hidden.\n\n---\n\n" +
			"## Thank you\n\nPress F for full page, O for the overview.\n"
	case "course":
		return "---\ntitle: " + yamlString(title) + "\nlayout: course\nlayoutOptions: { id: " + yamlString(courseID(title)) + " }\n---\n" +
			"# " + title + "\n\nWhat this course teaches, in one or two sentences.\n\n" +
			"## First step\n\nExplain the step.\n\n:::check\n- [ ] I did the first step\n:::\n\n" +
			"## Second step\n\nEach `##` heading starts a step.\n"
	default:
		return "---\ntitle: " + yamlString(title) + "\n---\n# " + title + "\n\n"
	}
}

// courseID is a stable storage id for a course's progress: "My course" -> "my-course".
func courseID(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	if id := strings.TrimSuffix(b.String(), "-"); id != "" {
		return id
	}
	return "course"
}

// yamlString quotes s when YAML would read it as something else.
func yamlString(s string) string {
	if s != "" && !strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`") && strings.TrimSpace(s) == s {
		return s
	}
	return "\"" + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + "\""
}

// create writes a new page, refusing to replace an existing file.
func (s *Session) create(rel, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws == nil {
		return errNoWorkspace
	}
	p, err := s.ws.Resolve(rel)
	if err != nil {
		return s.cleanErr(rel, err)
	}
	switch _, err := os.Lstat(p); {
	case err == nil:
		return errPageExists
	case !errors.Is(err, fs.ErrNotExist):
		return s.cleanErr(rel, err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return s.cleanErr(rel, err)
	}
	return s.cleanErr(rel, s.ws.Write(rel, []byte(content)))
}
