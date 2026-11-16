package scriptplugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/token"

	"github.com/dragosh/zen/internal/core/extplugins"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/render"
	"github.com/dragosh/zen/internal/core/workspace"
)

const (
	maxReadBytes = 1 << 20 // zen.read: largest file
	maxChildren  = 1 << 16 // zen.html: children in one call
)

// htmlObj is the value zen.html and zen.markdown return: HTML that zen.html
// takes as it is, where a plain string child is escaped. It prints as its
// HTML, so "<p>" + zen.html(...) and out = zen.html(...) work as expected.
type htmlObj struct {
	tengo.ObjectImpl
	s string
}

func (h *htmlObj) TypeName() string { return "html" }
func (h *htmlObj) String() string   { return h.s }
func (h *htmlObj) IsFalsy() bool    { return h.s == "" }
func (h *htmlObj) Copy() tengo.Object {
	return &htmlObj{s: h.s}
}
func (h *htmlObj) Equals(o tengo.Object) bool {
	switch o := o.(type) {
	case *htmlObj:
		return h.s == o.s
	case *tengo.String:
		return h.s == o.Value
	}
	return false
}
func (h *htmlObj) BinaryOp(op token.Token, rhs tengo.Object) (tengo.Object, error) {
	if op == token.Add {
		r, _ := tengo.ToString(rhs)
		s := h.s + r
		if len(s) > tengo.MaxStringLen {
			return nil, tengo.ErrStringLimit
		}
		return &tengo.String{Value: s}, nil
	}
	return nil, tengo.ErrInvalidOperator
}

var (
	tagRE  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)
	attrRE = regexp.MustCompile(`^[A-Za-z_:][A-Za-z0-9_:.-]*$`)
	void   = map[string]bool{"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
		"img": true, "input": true, "link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true}
	// Files zen.read opens: markdown and plain text.
	readable = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".text": true, ".csv": true,
		".tsv": true, ".json": true, ".yml": true, ".yaml": true}
)

// zenCall is the `zen` module of one call: a Go-built map of user functions
// bound to that call's context.
type zenCall struct {
	p    *Plugin
	ctx  plugins.Context
	rt   Runtime
	root string // resolved folder root, "" when there is none
	read bool   // the "read" permission
}

func fn(name string, f tengo.CallableFunc) *tengo.UserFunction {
	return &tengo.UserFunction{Name: name, Value: f}
}

func (z *zenCall) module() *tengo.ImmutableMap {
	return &tengo.ImmutableMap{Value: map[string]tengo.Object{
		"escape":   fn("escape", z.escape),
		"html":     fn("html", z.html),
		"markdown": fn("markdown", z.markdown),
		"read":     fn("read", z.readFile),
		"pages":    fn("pages", z.pages),
		"storage": &tengo.ImmutableMap{Value: map[string]tengo.Object{
			"get":  fn("get", z.storageGet),
			"set":  fn("set", z.storageSet),
			"keys": fn("keys", z.storageKeys),
		}},
	}}
}

func str(o tengo.Object) string {
	s, _ := tengo.ToString(o) // undefined is ""
	return s
}

func (z *zenCall) escape(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	return &tengo.String{Value: html.EscapeString(str(args[0]))}, nil
}

// children appends the HTML of one child: text is escaped, html values are
// kept, arrays are flattened, undefined is nothing.
func children(b *strings.Builder, o tengo.Object, depth int, count *int) error {
	if depth > 16 {
		return errors.New("zen.html: children nested too deeply")
	}
	if *count++; *count > maxChildren {
		return errors.New("zen.html: too many children")
	}
	switch o := o.(type) {
	case *htmlObj:
		b.WriteString(o.s)
	case *tengo.String:
		b.WriteString(html.EscapeString(o.Value))
	case *tengo.Int, *tengo.Float, *tengo.Bool, *tengo.Char:
		val, _ := tengo.ToString(o)
		b.WriteString(html.EscapeString(val))
	case *tengo.Array:
		for _, e := range o.Value {
			if err := children(b, e, depth+1, count); err != nil {
				return err
			}
		}
	case *tengo.ImmutableArray:
		for _, e := range o.Value {
			if err := children(b, e, depth+1, count); err != nil {
				return err
			}
		}
	default:
		if o == tengo.UndefinedValue {
			return nil
		}
		return fmt.Errorf("zen.html: a %s cannot be a child (use strings, numbers, zen.html results or lists of them)", o.TypeName())
	}
	if b.Len() > tengo.MaxStringLen {
		return tengo.ErrStringLimit
	}
	return nil
}

func (z *zenCall) html(args ...tengo.Object) (tengo.Object, error) {
	if len(args) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	tagObj, ok := args[0].(*tengo.String)
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "tag", Expected: "string", Found: args[0].TypeName()}
	}
	tag := tagObj.Value
	if !tagRE.MatchString(tag) {
		return nil, fmt.Errorf("zen.html: %q is not a tag name", tag)
	}
	var attrs map[string]tengo.Object
	rest := []tengo.Object(nil)
	if len(args) >= 2 {
		switch a := args[1].(type) {
		case *tengo.Map:
			attrs = a.Value
		case *tengo.ImmutableMap:
			attrs = a.Value
		default:
			if args[1] != tengo.UndefinedValue {
				return nil, tengo.ErrInvalidArgumentType{Name: "attrs", Expected: "map", Found: args[1].TypeName()}
			}
		}
		rest = args[2:]
	}
	var b strings.Builder
	b.WriteString("<" + tag)
	names := make([]string, 0, len(attrs))
	for k := range attrs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if !attrRE.MatchString(k) {
			return nil, fmt.Errorf("zen.html: %q is not an attribute name", k)
		}
		switch v := attrs[k].(type) {
		case *tengo.Bool:
			if !v.IsFalsy() {
				b.WriteString(" " + k)
			}
		case *tengo.String, *tengo.Int, *tengo.Float, *tengo.Char, *htmlObj:
			val, _ := tengo.ToString(v)
			b.WriteString(" " + k + `="` + html.EscapeString(val) + `"`)
		default:
			if v == tengo.UndefinedValue {
				continue
			}
			return nil, fmt.Errorf("zen.html: the value of attribute %q is a %s (want a string, number or bool)", k, v.TypeName())
		}
	}
	b.WriteString(">")
	if void[strings.ToLower(tag)] {
		if len(rest) > 0 {
			return nil, fmt.Errorf("zen.html: <%s> cannot have children", tag)
		}
		return &htmlObj{s: b.String()}, nil
	}
	count := 0
	for _, c := range rest {
		if err := children(&b, c, 0, &count); err != nil {
			return nil, err
		}
	}
	b.WriteString("</" + tag + ">")
	if b.Len() > tengo.MaxStringLen {
		return nil, tengo.ErrStringLimit
	}
	return &htmlObj{s: b.String()}, nil
}

func (z *zenCall) markdown(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	if z.ctx.Markdown == nil {
		return nil, errors.New("zen.markdown is not available here")
	}
	out, err := z.ctx.Markdown(str(args[0]))
	if err != nil {
		return nil, err
	}
	if len(out) > tengo.MaxStringLen {
		return nil, tengo.ErrStringLimit
	}
	return &htmlObj{s: out}, nil
}

func (z *zenCall) store() (Storage, error) {
	if z.rt.Storage == nil {
		return nil, errors.New("zen.storage is not available here")
	}
	return z.rt.Storage, nil
}

func keyArg(args []tengo.Object, i int) (string, error) {
	s, ok := args[i].(*tengo.String)
	if !ok {
		return "", tengo.ErrInvalidArgumentType{Name: "key", Expected: "string", Found: args[i].TypeName()}
	}
	return s.Value, nil
}

func (z *zenCall) storageGet(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	key, err := keyArg(args, 0)
	if err != nil {
		return nil, err
	}
	st, err := z.store()
	if err != nil {
		return nil, err
	}
	raw, err := st.Get(key)
	if err != nil {
		return nil, err
	}
	if raw == "" || raw == "null" {
		return tengo.UndefinedValue, nil
	}
	return fromJSON([]byte(raw))
}

func (z *zenCall) storageSet(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 2 {
		return nil, tengo.ErrWrongNumArguments
	}
	key, err := keyArg(args, 0)
	if err != nil {
		return nil, err
	}
	st, err := z.store()
	if err != nil {
		return nil, err
	}
	v, err := toGo(args[1])
	if err != nil {
		return nil, fmt.Errorf("zen.storage.set: %v", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("zen.storage.set: %v", err)
	}
	if err := st.Set(key, string(data)); err != nil {
		return nil, err
	}
	return tengo.UndefinedValue, nil
}

func (z *zenCall) storageKeys(args ...tengo.Object) (tengo.Object, error) {
	prefix := ""
	switch len(args) {
	case 0:
	case 1:
		prefix = str(args[0])
	default:
		return nil, tengo.ErrWrongNumArguments
	}
	st, err := z.store()
	if err != nil {
		return nil, err
	}
	keys, err := st.Keys(prefix)
	if err != nil {
		return nil, err
	}
	return fromGo(keys)
}

func (z *zenCall) needRead(what string) error {
	if !z.read {
		return fmt.Errorf(`%s needs the "read" permission (plugin := {permissions: ["read"], …})`, what)
	}
	if z.root == "" {
		return errors.New(what + ": no folder is open")
	}
	return nil
}

// readText reads the text file rel of the folder root: a markdown or text
// file (the extension is checked when checkExt is set), a regular file
// reached without leaving the folder (symlinks are resolved first), at most
// 1 MB of valid UTF-8. exists is false for a file that is not there.
func readText(root, rel string, checkExt bool) (text string, exists bool, err error) {
	if rel == "" {
		return "", false, errors.New("zen.read needs a file path inside the folder")
	}
	if strings.ContainsRune(rel, 0) || strings.ContainsAny(rel, "\\") {
		return "", false, errors.New("the path is not valid")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("%q is outside the folder", rel)
	}
	if checkExt && !readable[strings.ToLower(filepath.Ext(clean))] {
		return "", false, errors.New("only markdown and text files can be read (.md, .txt, .csv, .json, .yml)")
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, clean))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%q cannot be read", rel)
	}
	if !extplugins.InsideRel(root, full) {
		return "", false, fmt.Errorf("%q is outside the folder", rel)
	}
	if checkExt && !readable[strings.ToLower(filepath.Ext(full))] {
		return "", false, errors.New("only markdown and text files can be read (.md, .txt, .csv, .json, .yml)")
	}
	fi, err := os.Stat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false, fmt.Errorf("%q is not a file", rel)
	}
	data, err := extplugins.ReadRegular(full, maxReadBytes)
	if err != nil {
		if err.Error() == "too large" {
			return "", false, fmt.Errorf("%q is larger than 1 MB", rel)
		}
		return "", false, fmt.Errorf("%q cannot be read", rel)
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return "", false, fmt.Errorf("%q is not UTF-8 text", rel)
	}
	return string(data), true, nil
}

func (z *zenCall) readFile(args ...tengo.Object) (tengo.Object, error) {
	if err := z.needRead("zen.read"); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	rel, ok := args[0].(*tengo.String)
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "path", Expected: "string", Found: args[0].TypeName()}
	}
	text, exists, err := readText(z.root, rel.Value, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return tengo.UndefinedValue, nil
	}
	return &tengo.String{Value: text}, nil
}

// titles caches page titles by path, validated by size and modification time.
type titleEntry struct {
	size  int64
	mtime int64
	title string
}

var titler = render.New(nil, nil, nil) // no plugins: the title is the frontmatter's, the first heading or the file name

// titleOf is the title of the page rel in the folder (the file name without
// extension when the page cannot be read).
func (p *Plugin) titleOf(root, rel string) string {
	stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	if root == "" || rel == "" {
		return stem
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || !extplugins.InsideRel(root, full) {
		return stem
	}
	fi, err := os.Stat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return stem
	}
	if v, ok := p.titles.Load(rel); ok {
		if e := v.(titleEntry); e.size == fi.Size() && e.mtime == fi.ModTime().UnixNano() {
			return e.title
		}
	}
	text, exists, err := readText(root, rel, false)
	if err != nil || !exists {
		return stem
	}
	title := titler.Title(rel, []byte(text))
	if p.titleCount.Add(1) > 4096 { // a bounded cache
		p.titles.Range(func(k, _ any) bool { p.titles.Delete(k); return true })
		p.titleCount.Store(0)
	}
	p.titles.Store(rel, titleEntry{size: fi.Size(), mtime: fi.ModTime().UnixNano(), title: title})
	return title
}

func (z *zenCall) pages(args ...tengo.Object) (tengo.Object, error) {
	if err := z.needRead("zen.pages"); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, tengo.ErrWrongNumArguments
	}
	ws, err := workspace.Open(z.root)
	if err != nil {
		return nil, errors.New("zen.pages: the folder cannot be read")
	}
	files, err := ws.Files()
	if err != nil {
		return nil, errors.New("zen.pages: the folder has too many files")
	}
	out := make([]tengo.Object, len(files))
	for i, f := range files {
		out[i] = &tengo.ImmutableMap{Value: map[string]tengo.Object{
			"path":  &tengo.String{Value: f},
			"title": &tengo.String{Value: z.p.titleOf(z.root, f)},
		}}
	}
	return &tengo.ImmutableArray{Value: out}, nil
}
