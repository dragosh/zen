package layouts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"html/template"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/dragosh/zenmd/engine/plugins"
)

// maxOutput caps what a template may write.
const maxOutput = 4 << 20

// Crumb is one step of the breadcrumb.
type Crumb struct{ Title, Path string }

// Link points at another page (Prev and Next).
type Link struct{ Title, Path string }

// Child is a page under this one in the sidebar.
type Child struct{ Title, Path, Summary string }

// TOCItem is a heading of the page (h2 to h4) that has an id.
type TOCItem struct {
	Level    int
	ID, Text string
}

// Site answers questions about where the page sits in the folder. A nil Site
// knows nothing: no breadcrumb, neighbours or children.
type Site interface {
	Breadcrumb(path string) []Crumb
	// Neighbours are the pages before and after path in the sidebar order.
	Neighbours(path string) (prev, next *Link)
	// Children are the pages under path in the sidebar, in order.
	Children(path string) []Child
}

// Input is a page to be laid out.
type Input struct {
	Title, Path string
	Content     string         // the rendered page body
	Frontmatter map[string]any // the page's frontmatter
	Options     map[string]any // the layout options the page gave
	Site        Site
	// Markdown renders markdown through the page's own pipeline; nil renders
	// the text as plain text.
	Markdown func(src string) (string, error)
	// Image resolves an image field: a workspace path or an https address to
	// the URL the page may show; false when it is neither.
	Image func(value string) (string, bool)
}

// Result is a page inside its layout.
type Result struct {
	HTML     string            // the wrapper element with the layout's output inside
	Attrs    map[string]string // the options the layout's script reads (data-zen-attrs)
	Warnings []string
}

// Data is what a template sees. Fields are plain data; Field, Option and
// Children are methods so only what a template asks for is worked out.
type Data struct {
	Title, Path string
	Content     template.HTML
	Breadcrumb  []Crumb
	TOC         []TOCItem
	Prev, Next  *Link
	Words       int
	ReadingTime int // minutes, at least 1

	fields   map[string]any
	options  map[string]any
	site     Site
	children []Child
	got      bool
}

// Field returns the frontmatter field name, typed by its declaration in
// layout.yml: string and text and image give a string, int an int, bool a
// bool, date a *time.Time (nil when the page gives none) and list a []string.
// A missing field gives the empty value. A field the layout does not declare
// is an error.
func (d *Data) Field(name string) (any, error) {
	v, ok := d.fields[name]
	if !ok {
		return nil, fmt.Errorf("field %q is not declared in layout.yml", name)
	}
	return v, nil
}

// Option returns the layout option name, typed by its declaration, with the
// page's value or the default.
func (d *Data) Option(name string) (any, error) {
	v, ok := d.options[name]
	if !ok {
		return nil, fmt.Errorf("option %q is not declared in layout.yml", name)
	}
	return v, nil
}

// Children are the pages under this one in the sidebar.
func (d *Data) Children() []Child {
	if !d.got {
		d.got = true
		if d.site != nil {
			d.children = d.site.Children(d.Path)
		}
	}
	return d.children
}

func (l *Layout) warnf(res *Result, format string, args ...any) {
	res.Warnings = append(res.Warnings, fmt.Sprintf("layout %q: ", l.man.ID)+fmt.Sprintf(format, args...))
}

// Apply lays the page out: it works out the options and fields, runs the
// template (when there is one) and wraps the result in the layout's element.
// An error means the template failed; the caller falls back to the article.
func (l *Layout) Apply(in Input) (Result, error) {
	res := Result{Attrs: map[string]string{}}
	opts := l.options(in.Options, &res)
	inner := in.Content
	if l.tmpl != nil {
		d := &Data{Title: in.Title, Path: in.Path, Content: template.HTML(in.Content), site: in.Site, options: opts}
		d.fields = l.fields(in, &res)
		if in.Site != nil {
			d.Breadcrumb = in.Site.Breadcrumb(in.Path)
			d.Prev, d.Next = in.Site.Neighbours(in.Path)
		}
		d.TOC = tableOfContents(in.Content)
		d.Words = countWords(in.Content)
		d.ReadingTime = max(1, int(math.Ceil(float64(d.Words)/200)))
		out, err := l.execute(d, in)
		if err != nil {
			return Result{}, err
		}
		inner = out
	}
	for k, v := range opts {
		res.Attrs[k] = scalar(v)
	}
	if len(l.man.Options) == 0 {
		for k, v := range in.Options {
			res.Attrs[k] = scalar(v)
		}
	}
	res.HTML = `<div data-zen-layout="` + stdhtml.EscapeString(l.man.ID) + `" data-zen-layout-source="` + stdhtml.EscapeString(l.source) + `"` +
		attrsAttr(res.Attrs) + ">" + inner + "</div>"
	return res, nil
}

func attrsAttr(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(attrs) != nil {
		return ""
	}
	return ` data-zen-attrs="` + stdhtml.EscapeString(strings.TrimSpace(b.String())) + `"`
}

func scalar(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// options returns the typed options of the layout: the declared ones with
// the page's value or the default. With none declared, the page's options pass
// through as they are; with some declared, others are dropped with a warning.
func (l *Layout) options(given map[string]any, res *Result) map[string]any {
	out := map[string]any{}
	declared := map[string]Option{}
	for _, o := range l.man.Options {
		declared[o.Name] = o
		out[o.Name] = cloneValue(o.Default)
	}
	keys := make([]string, 0, len(given))
	for k := range given {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		o, ok := declared[k]
		if !ok {
			if len(declared) > 0 {
				l.warnf(res, "unknown option %q", k)
			}
			continue
		}
		v, ok := coerceOption(o.Type, given[k])
		if !ok {
			l.warnf(res, "option %q must be of type %s", k, o.Type)
			continue
		}
		out[k] = v
	}
	if len(declared) == 0 {
		return map[string]any{}
	}
	return out
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]string:
		m := make(map[string]string, len(t))
		for k, x := range t {
			m[k] = x
		}
		return m
	case []string:
		return append([]string{}, t...)
	}
	return v
}

// coerceOption turns a frontmatter value into the option's type. Scalars may
// be written as text ("true", "3").
func coerceOption(typ string, v any) (any, bool) {
	switch typ {
	case "string":
		switch v.(type) {
		case string, bool, int, int64, float64:
			return scalar(v), true
		}
		return nil, false
	case "bool":
		return toBool(v)
	case "int":
		return toInt(v)
	}
	return plugins.CoerceOption(typ, v)
}

func toBool(v any) (any, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "on":
			return true, true
		case "false", "no", "off":
			return false, true
		}
	}
	return nil, false
}

func toInt(v any) (any, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1<<53 {
			return int(t), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n, true
		}
	}
	return nil, false
}

// fields types the frontmatter fields the layout declares.
func (l *Layout) fields(in Input, res *Result) map[string]any {
	out := make(map[string]any, len(l.man.Fields))
	for _, f := range l.man.Fields {
		raw, present := in.Frontmatter[f.Name]
		switch f.Type {
		case "string", "text":
			out[f.Name] = ""
		case "image":
			out[f.Name] = ""
		case "int":
			out[f.Name] = 0
		case "bool":
			out[f.Name] = false
		case "date":
			out[f.Name] = (*time.Time)(nil)
		case "list":
			out[f.Name] = []string{}
		}
		if !present || raw == nil {
			continue
		}
		bad := func(what string) { l.warnf(res, "field %q must be %s", f.Name, what) }
		switch f.Type {
		case "string", "text":
			switch t := raw.(type) {
			case time.Time:
				out[f.Name] = t.Format("2006-01-02")
			case string, bool, int, int64, float64:
				out[f.Name] = scalar(t)
			default:
				bad("text")
			}
		case "int":
			if n, ok := toInt(raw); ok {
				out[f.Name] = n
			} else {
				bad("a whole number")
			}
		case "bool":
			if b, ok := toBool(raw); ok {
				out[f.Name] = b
			} else {
				bad("true or false")
			}
		case "date":
			if t, ok := toDate(raw); ok {
				out[f.Name] = &t
			} else {
				bad("a date such as 2026-11-17")
			}
		case "list":
			switch t := raw.(type) {
			case string:
				out[f.Name] = []string{t}
			case []any:
				items := make([]string, 0, len(t))
				ok := true
				for _, x := range t {
					switch x.(type) {
					case string, bool, int, int64, float64:
						items = append(items, scalar(x))
					default:
						ok = false
					}
				}
				if ok {
					out[f.Name] = items
				} else {
					bad("a list of text")
				}
			case []string:
				out[f.Name] = append([]string{}, t...)
			default:
				bad("a list")
			}
		case "image":
			s, ok := raw.(string)
			if !ok {
				bad("an image path or address")
				break
			}
			if in.Image != nil {
				if u, ok := in.Image(s); ok {
					out[f.Name] = u
					break
				}
			}
			l.warnf(res, "field %q: image %q is not an image of this folder or an https address", f.Name, s)
		}
	}
	return out
}

var dateLayouts = []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04", "2006-01-02T15:04:05", "2 January 2006", "January 2, 2006"}

func toDate(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		s := strings.TrimSpace(t)
		for _, lay := range dateLayouts {
			if d, err := time.Parse(lay, s); err == nil {
				return d, true
			}
		}
	}
	return time.Time{}, false
}

// ---- template -------------------------------------------------------------

func parseTemplate(m Manifest, src string) (*template.Template, error) {
	t, err := template.New("layout").Funcs(funcs(nil)).Parse(src)
	if err != nil {
		return nil, fmt.Errorf("template %s: %s", m.Template, cleanTemplateErr(err))
	}
	return t, nil
}

func cleanTemplateErr(err error) string {
	msg := strings.TrimPrefix(err.Error(), "template: ")
	return strings.TrimPrefix(msg, "layout:")
}

func funcs(md func(string) (template.HTML, error)) template.FuncMap {
	if md == nil {
		md = func(string) (template.HTML, error) { return "", errors.New("markdown is not available") }
	}
	return template.FuncMap{
		"date": func(v any, layout ...string) (string, error) {
			lay := "2 January 2006"
			if len(layout) > 0 {
				lay = layout[0]
			}
			switch t := v.(type) {
			case nil:
				return "", nil
			case *time.Time:
				if t == nil || t.IsZero() {
					return "", nil
				}
				return t.Format(lay), nil
			case time.Time:
				if t.IsZero() {
					return "", nil
				}
				return t.Format(lay), nil
			case string:
				if d, ok := toDate(t); ok {
					return d.Format(lay), nil
				}
				return "", nil
			}
			return "", fmt.Errorf("date: %T is not a date", v)
		},
		"upper": func(v any) string { return strings.ToUpper(textOf(v)) },
		"lower": func(v any) string { return strings.ToLower(textOf(v)) },
		"join": func(list any, sep string) (string, error) {
			switch t := list.(type) {
			case []string:
				return strings.Join(t, sep), nil
			case []any:
				parts := make([]string, len(t))
				for i, x := range t {
					parts[i] = textOf(x)
				}
				return strings.Join(parts, sep), nil
			case nil:
				return "", nil
			}
			return "", fmt.Errorf("join: %T is not a list", list)
		},
		"markdown": md,
	}
}

func textOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case template.HTML:
		return string(t)
	}
	return fmt.Sprint(v)
}

type limitWriter struct {
	buf bytes.Buffer
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > maxOutput {
		return 0, errors.New("the template wrote more than 4 MB")
	}
	return w.buf.Write(p)
}

var _ io.Writer = (*limitWriter)(nil)

func (l *Layout) execute(d *Data, in Input) (out string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("template failed: %v", p)
		}
	}()
	md := func(src string) (template.HTML, error) {
		if in.Markdown == nil {
			return template.HTML(stdhtml.EscapeString(src)), nil
		}
		h, err := in.Markdown(src)
		if err != nil {
			return "", fmt.Errorf("markdown: %v", err)
		}
		return template.HTML(h), nil
	}
	t, err := l.tmpl.Clone()
	if err != nil {
		return "", fmt.Errorf("template: %v", err)
	}
	t = t.Funcs(funcs(md))
	w := &limitWriter{}
	if err := t.Execute(w, d); err != nil {
		return "", fmt.Errorf("template %s: %s", l.man.Template, cleanExecErr(err))
	}
	return w.buf.String(), nil
}

func cleanExecErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); strings.HasPrefix(msg, "template: ") && i >= 0 {
		msg = msg[i+2:]
	}
	return msg
}

// ---- page analysis --------------------------------------------------------

// tableOfContents lists the h2 to h4 headings that have an id.
func tableOfContents(content string) []TOCItem {
	var items []TOCItem
	z := html.NewTokenizer(strings.NewReader(content))
	level, id := 0, ""
	var text strings.Builder
	for {
		switch z.Next() {
		case html.ErrorToken:
			return items
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			if len(name) == 2 && name[0] == 'h' && name[1] >= '2' && name[1] <= '4' && level == 0 {
				level, id = int(name[1]-'0'), ""
				text.Reset()
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					if string(k) == "id" {
						id = string(v)
					}
				}
			}
		case html.TextToken:
			if level != 0 {
				text.Write(z.Text())
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if level != 0 && len(name) == 2 && name[0] == 'h' && int(name[1]-'0') == level {
				if t := strings.Join(strings.Fields(text.String()), " "); id != "" && t != "" {
					items = append(items, TOCItem{Level: level, ID: id, Text: t})
				}
				level = 0
			}
		}
	}
}

// countWords counts the words of the text of an HTML fragment.
func countWords(content string) int {
	n := 0
	z := html.NewTokenizer(strings.NewReader(content))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return n
		case html.TextToken:
			n += len(strings.Fields(string(z.Text())))
		}
	}
}
