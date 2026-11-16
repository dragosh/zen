package scriptplugins

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime/metrics"
	"strings"
	"sync"
	"time"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"

	"github.com/dragosh/zen/internal/core/plugins"
)

// Limits of a script plugin.
const (
	maxScript      = 256 << 10 // bytes of a script file
	maxOutput      = 2 << 20   // bytes of output of one call
	maxAllocs      = 500_000   // objects one call may allocate
	defaultTimeout = 200 * time.Millisecond
	maxTimeout     = 2 * time.Second
	maxCSS         = 64 << 10 // bytes of inline css

	// A script can grow memory faster than it allocates objects (doubling an
	// array is one allocation): a call that makes the heap grow by memLimit is
	// stopped. The heap is sampled every memTick, from memDelay into the call.
	memLimit = 256 << 20
	memDelay = 10 * time.Millisecond
	memTick  = 2 * time.Millisecond
)

func init() {
	// One string or byte value never needs to be larger than the output may
	// be, plus room for what zen.read brings in. (Tengo's default is 2 GB.)
	tengo.MaxStringLen = 4 << 20
	tengo.MaxBytesLen = 4 << 20
}

// The modules a script may import: pure computation only.
var allowedModules = []string{"text", "math", "times", "fmt", "json", "enum", "base64", "hex", "rand"}

// Attributes left out of a module because they would block or write to zen's
// own output: times.sleep cannot be interrupted, fmt.print* write to stdout.
var withheld = map[string]map[string]bool{
	"times": {"sleep": true},
	"fmt":   {"print": true, "println": true, "printf": true},
}

func moduleMap() *tengo.ModuleMap {
	m := tengo.NewModuleMap()
	for _, name := range allowedModules {
		if mod := stdlib.BuiltinModules[name]; mod != nil {
			attrs := make(map[string]tengo.Object, len(mod))
			for k, v := range mod {
				if !withheld[name][k] {
					attrs[k] = v
				}
			}
			m.AddBuiltinModule(name, attrs)
		}
		if src := stdlib.SourceModules[name]; src != "" {
			m.AddSourceModule(name, []byte(src))
		}
	}
	return m
}

// The globals of a script: inputs and the output.
var globalNames = []string{"hook", "name", "body", "attrs", "config", "page", "payload", "out", "zen"}

// compiled is a script compiled once; every call runs a clone of base.
type compiled struct{ base *tengo.Compiled }

var (
	cacheMu sync.Mutex
	cache   = map[[32]byte]*compiled{} // by the hash of the source: one compile per content
)

func compile(src []byte) (*compiled, error) {
	key := sha256.Sum256(src)
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if c, ok := cache[key]; ok {
		return c, nil
	}
	s := tengo.NewScript(src)
	s.SetImports(moduleMap())
	s.SetMaxAllocs(maxAllocs)
	for _, g := range globalNames {
		if err := s.Add(g, tengo.UndefinedValue); err != nil {
			return nil, err
		}
	}
	base, err := s.Compile()
	if err != nil {
		return nil, errors.New(cleanErr(err))
	}
	if len(cache) >= 64 { // edits make new content: keep the cache small
		cache = map[[32]byte]*compiled{}
	}
	c := &compiled{base: base}
	cache[key] = c
	return c, nil
}

var (
	posRE    = regexp.MustCompile(`\(?main\)?:(\d+):\d+`)
	prefixRE = regexp.MustCompile(`^(Runtime|Compile|Parse) Error: `)
)

// cleanErr turns a Tengo error into one line: "line N: message", without the
// stack of positions or any path.
func cleanErr(err error) string {
	msg := err.Error()
	line := ""
	if m := posRE.FindStringSubmatch(msg); m != nil {
		line = m[1]
	}
	first := strings.TrimSpace(strings.SplitN(msg, "\n", 2)[0])
	first = prefixRE.ReplaceAllString(first, "")
	if i := strings.Index(first, " at "); i > 0 && posRE.MatchString(first[i:]) {
		first = first[:i]
	}
	switch {
	case strings.Contains(first, tengo.ErrObjectAllocLimit.Error()):
		first = fmt.Sprintf("allocation limit exceeded (%d objects)", maxAllocs)
	case strings.Contains(first, tengo.ErrStringLimit.Error()), strings.Contains(first, tengo.ErrBytesLimit.Error()):
		first = fmt.Sprintf("a string or bytes value is larger than %d MB", tengo.MaxStringLen>>20)
	}
	if line != "" {
		return "line " + line + ": " + first
	}
	return first
}

var (
	errCancelled = errors.New("cancelled")
	errMemory    = errors.New("memory limit exceeded")
)

// watchMemory cancels the call when the heap grows by memLimit. The returned
// function stops the watcher.
func watchMemory(cancel context.CancelCauseFunc) (stop func()) {
	done := make(chan struct{})
	t := time.AfterFunc(memDelay, func() {
		sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
		read := func() uint64 {
			metrics.Read(sample)
			if sample[0].Value.Kind() != metrics.KindUint64 {
				return 0
			}
			return sample[0].Value.Uint64()
		}
		base := read()
		tk := time.NewTicker(memTick)
		defer tk.Stop()
		for {
			select {
			case <-done:
				return
			case <-tk.C:
				if cur := read(); cur > base && cur-base > memLimit {
					cancel(errMemory)
					return
				}
			}
		}
	})
	var once sync.Once
	return func() { once.Do(func() { close(done); t.Stop() }) }
}

// input is what a call hands the script.
type input struct {
	hook, name, body string
	attrs            map[string]string
	config           map[string]any
	page             struct{ path, title string }
	payload          []byte // JSON, "" for none
}

// exec runs one call: a clone of the compiled script with fresh globals, the
// zen module bound to z, under the time, allocation and memory limits. It
// returns the clone, from which the caller reads out (and plugin).
func (c *compiled) exec(parent context.Context, timeout time.Duration, in input, z *zenCall) (*tengo.Compiled, error) {
	if parent == nil {
		parent = context.Background()
	}
	if parent.Err() != nil {
		return nil, errCancelled
	}
	run := c.base.Clone()
	set := func(name string, v any) error { return run.Set(name, v) }
	attrs := map[string]tengo.Object{}
	for k, v := range in.attrs {
		attrs[k] = &tengo.String{Value: v}
	}
	cfg, err := fromGo(in.config)
	if err != nil {
		return nil, err
	}
	if in.config == nil {
		cfg = &tengo.Map{Value: map[string]tengo.Object{}}
	}
	payload := tengo.Object(tengo.UndefinedValue)
	if in.payload != nil {
		if payload, err = fromJSON(in.payload); err != nil {
			return nil, errors.New("the payload is not valid JSON")
		}
	}
	for name, v := range map[string]tengo.Object{
		"hook":    &tengo.String{Value: in.hook},
		"name":    &tengo.String{Value: in.name},
		"body":    &tengo.String{Value: in.body},
		"attrs":   &tengo.Map{Value: attrs},
		"config":  cfg,
		"page":    &tengo.Map{Value: map[string]tengo.Object{"path": &tengo.String{Value: in.page.path}, "title": &tengo.String{Value: in.page.title}}},
		"payload": payload,
		"zen":     z.module(),
	} {
		if err := set(name, v); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	stop := watchMemory(cancel)
	defer stop()
	tctx, tcancel := context.WithTimeout(ctx, timeout)
	defer tcancel()
	err = run.RunContext(tctx)
	switch {
	case err == nil:
		return run, nil
	case parent.Err() != nil:
		return nil, errCancelled
	case context.Cause(ctx) == errMemory:
		return nil, errMemory
	case errors.Is(err, context.DeadlineExceeded):
		return nil, fmt.Errorf("timed out after %s", timeout)
	}
	return nil, errors.New(cleanErr(err))
}

// outString reads the value of out for a hook that returns text.
func outString(o tengo.Object) (string, bool, error) {
	var s string
	switch o := o.(type) {
	case *tengo.String:
		s = o.Value
	case *htmlObj:
		s = o.s
	case *tengo.Bytes:
		s = string(o.Value)
	default:
		if o == nil || o == tengo.UndefinedValue {
			return "", false, nil
		}
		return "", false, fmt.Errorf("out must be a string (or zen.html result), not a %s", o.TypeName())
	}
	if len(s) > maxOutput {
		return "", false, errors.New("the output is larger than 2 MB")
	}
	return s, true, nil
}

// outJSON reads the value of out for an action: any value, as JSON.
func outJSON(o tengo.Object) ([]byte, error) {
	v, err := toGo(o)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(data) > maxOutput {
		return nil, errors.New("the output is larger than 2 MB")
	}
	return data, nil
}

var _ = plugins.APIVersion
