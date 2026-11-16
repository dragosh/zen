package scriptplugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/d5/tengo/v2"
)

const (
	maxDepth = 32      // nesting of values converted to or from JSON
	maxNodes = 1 << 20 // values converted at once
)

// fromGo converts a Go value (string, bool, numbers, nil, slices and maps of
// those, json.Number) to a Tengo object. nil is undefined.
func fromGo(v any) (tengo.Object, error) {
	n := 0
	return fromGoDepth(v, 0, &n)
}

func fromGoDepth(v any, depth int, nodes *int) (tengo.Object, error) {
	if depth > maxDepth {
		return nil, errors.New("value nested too deeply")
	}
	if *nodes++; *nodes > maxNodes {
		return nil, errors.New("value too large")
	}
	switch v := v.(type) {
	case nil:
		return tengo.UndefinedValue, nil
	case string:
		return &tengo.String{Value: v}, nil
	case bool:
		if v {
			return tengo.TrueValue, nil
		}
		return tengo.FalseValue, nil
	case int:
		return &tengo.Int{Value: int64(v)}, nil
	case int64:
		return &tengo.Int{Value: v}, nil
	case float64:
		return &tengo.Float{Value: v}, nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return &tengo.Int{Value: i}, nil
		}
		f, err := v.Float64()
		if err != nil {
			return nil, err
		}
		return &tengo.Float{Value: f}, nil
	case []string:
		arr := make([]tengo.Object, len(v))
		for i, s := range v {
			arr[i] = &tengo.String{Value: s}
		}
		return &tengo.Array{Value: arr}, nil
	case []any:
		arr := make([]tengo.Object, len(v))
		for i, e := range v {
			o, err := fromGoDepth(e, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			arr[i] = o
		}
		return &tengo.Array{Value: arr}, nil
	case map[string]string:
		m := make(map[string]tengo.Object, len(v))
		for k, s := range v {
			m[k] = &tengo.String{Value: s}
		}
		return &tengo.Map{Value: m}, nil
	case map[string]any:
		m := make(map[string]tengo.Object, len(v))
		for k, e := range v {
			o, err := fromGoDepth(e, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			m[k] = o
		}
		return &tengo.Map{Value: m}, nil
	}
	return nil, fmt.Errorf("cannot pass a %T to a script", v)
}

// fromJSON decodes JSON text into a Tengo object (null is undefined; numbers
// without a fraction are ints).
func fromJSON(data []byte) (tengo.Object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, errors.New("not valid JSON")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("not valid JSON")
	}
	return fromGo(v)
}

// toGo converts a Tengo object to a value json.Marshal accepts: undefined is
// nil, strings, numbers and bools as they are, arrays and maps recursively.
// Anything else (functions, bytes, errors) is refused, and so is a value
// nested more than maxDepth levels deep (which includes any cycle).
func toGo(o tengo.Object) (any, error) {
	n := 0
	return toGoDepth(o, 0, &n)
}

func toGoDepth(o tengo.Object, depth int, nodes *int) (any, error) {
	if depth > maxDepth {
		return nil, errors.New("value nested too deeply (or it contains itself)")
	}
	if *nodes++; *nodes > maxNodes {
		return nil, errors.New("value too large")
	}
	switch o := o.(type) {
	case nil:
		return nil, nil
	case *tengo.String:
		return o.Value, nil
	case *htmlObj:
		return o.s, nil
	case *tengo.Int:
		return o.Value, nil
	case *tengo.Float:
		return o.Value, nil
	case *tengo.Bool:
		return !o.IsFalsy(), nil
	case *tengo.Char:
		return string(o.Value), nil
	case *tengo.Time:
		return o.Value.Format(time.RFC3339Nano), nil
	case *tengo.Array:
		return toGoList(o.Value, depth, nodes)
	case *tengo.ImmutableArray:
		return toGoList(o.Value, depth, nodes)
	case *tengo.Map:
		return toGoMap(o.Value, depth, nodes)
	case *tengo.ImmutableMap:
		return toGoMap(o.Value, depth, nodes)
	}
	if o == tengo.UndefinedValue {
		return nil, nil
	}
	return nil, fmt.Errorf("a %s cannot be turned into JSON", o.TypeName())
}

func toGoList(items []tengo.Object, depth int, nodes *int) (any, error) {
	out := make([]any, len(items))
	for i, e := range items {
		v, err := toGoDepth(e, depth+1, nodes)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func toGoMap(items map[string]tengo.Object, depth int, nodes *int) (any, error) {
	out := make(map[string]any, len(items))
	for k, e := range items {
		v, err := toGoDepth(e, depth+1, nodes)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}
