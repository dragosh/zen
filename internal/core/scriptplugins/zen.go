package scriptplugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/token"
)

const (
	maxChildren = 1 << 16 // zen.html: children in one call
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
)

// zenCall is the `zen` module of one call, as the helper runs it: escape and
// html are computed here, everything that needs zen (the renderer, storage,
// files) is a callback to the parent, which keeps the permission checks.
type zenCall struct {
	ctx  context.Context // the call's deadline; set by exec
	call func(ctx context.Context, fn string, args ...any) (json.RawMessage, error)
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

func (z *zenCall) ask(fn string, args ...any) (json.RawMessage, error) {
	if z.call == nil {
		return nil, errors.New("zen." + fn + " is not available here")
	}
	return z.call(z.ctx, fn, args...)
}

func (z *zenCall) markdown(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	raw, err := z.ask("markdown", str(args[0]))
	if err != nil {
		return nil, err
	}
	var out string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, errors.New("zen.markdown: bad answer")
	}
	if len(out) > tengo.MaxStringLen {
		return nil, tengo.ErrStringLimit
	}
	return &htmlObj{s: out}, nil
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
	raw, err := z.ask("storage.get", key)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return tengo.UndefinedValue, nil
	}
	return fromJSON(raw)
}

func (z *zenCall) storageSet(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 2 {
		return nil, tengo.ErrWrongNumArguments
	}
	key, err := keyArg(args, 0)
	if err != nil {
		return nil, err
	}
	v, err := toGo(args[1])
	if err != nil {
		return nil, fmt.Errorf("zen.storage.set: %v", err)
	}
	if _, err := z.ask("storage.set", key, v); err != nil {
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
	raw, err := z.ask("storage.keys", prefix)
	if err != nil {
		return nil, err
	}
	return fromJSON(raw)
}

func (z *zenCall) readFile(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 1 {
		// the parent checks the permission first, as it always did
		if _, err := z.ask("read.check"); err != nil {
			return nil, err
		}
		return nil, tengo.ErrWrongNumArguments
	}
	rel, ok := args[0].(*tengo.String)
	if !ok {
		if _, err := z.ask("read.check"); err != nil {
			return nil, err
		}
		return nil, tengo.ErrInvalidArgumentType{Name: "path", Expected: "string", Found: args[0].TypeName()}
	}
	raw, err := z.ask("read", rel.Value)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return tengo.UndefinedValue, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, errors.New("zen.read: bad answer")
	}
	return &tengo.String{Value: text}, nil
}

func (z *zenCall) pages(args ...tengo.Object) (tengo.Object, error) {
	if len(args) != 0 {
		if _, err := z.ask("read.check"); err != nil {
			return nil, err
		}
		return nil, tengo.ErrWrongNumArguments
	}
	raw, err := z.ask("pages")
	if err != nil {
		return nil, err
	}
	var list []struct{ Path, Title string }
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, errors.New("zen.pages: bad answer")
	}
	out := make([]tengo.Object, len(list))
	for i, f := range list {
		out[i] = &tengo.ImmutableMap{Value: map[string]tengo.Object{
			"path":  &tengo.String{Value: f.Path},
			"title": &tengo.String{Value: f.Title},
		}}
	}
	return &tengo.ImmutableArray{Value: out}, nil
}
