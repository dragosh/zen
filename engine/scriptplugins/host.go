package scriptplugins

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/d5/tengo/v2"
)

// HostArg is the hidden command-line mode that makes a zen binary serve as a
// script helper: `zen __script-host`.
const HostArg = "__script-host"

// helperMemory is the Go memory limit of a helper (a soft limit that makes
// the collector work harder; the hard stop is zen's kill at 512 MB).
const helperMemory = 256 << 20

// host is the helper side: it holds the compiled scripts and serves calls.
type host struct {
	exit  func() // ends the process (after a call that left a runaway script behind)
	out   io.Writer
	wmu   sync.Mutex
	mu    sync.Mutex
	files map[string]*compiled // by hash
	wait  map[int64]chan *msg  // callbacks waiting for a reply
	cb    int64
	calls sync.WaitGroup // calls still running
}

// drainTimeout bounds how long the helper waits, once its input has ended,
// for calls still running to send their answers (each call has its own,
// shorter time limit).
const drainTimeout = 3 * time.Second

// drain waits for running calls to finish, drainTimeout at most.
func (h *host) drain() {
	done := make(chan struct{})
	go func() { h.calls.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(drainTimeout):
	}
}

// ServeHost runs the helper: it reads messages from in and writes answers to
// out until in ends. It never reads files, starts programs or writes
// anything else to out. Lines that are not valid messages are ignored.
func ServeHost(in io.Reader, out io.Writer) { serve(in, out, func() {}) }

func serve(in io.Reader, out io.Writer, exit func()) {
	debug.SetMemoryLimit(helperMemory)
	h := &host{exit: exit, out: out, files: map[string]*compiled{}, wait: map[int64]chan *msg{}}
	r := bufio.NewReaderSize(in, 64<<10)
	for {
		line, err := readLine(r, maxLine)
		if err != nil {
			// EOF (zen is gone or closed us) or a line too long: answer the
			// calls already running, then stop.
			h.drain()
			return
		}
		var m msg
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		switch m.T {
		case tLoad:
			h.load(&m)
		case tCall:
			h.calls.Add(1)
			go func(m msg) {
				defer h.calls.Done()
				h.call(&m)
			}(m)
		case tReply:
			h.mu.Lock()
			ch := h.wait[m.CB]
			delete(h.wait, m.CB)
			h.mu.Unlock()
			if ch != nil {
				ch <- &m
			}
		}
	}
}

func (h *host) send(m *msg) {
	line, err := encodeLine(m)
	if err != nil {
		if m.T == tResult {
			line, _ = encodeLine(&msg{T: tResult, Seq: m.Seq, Error: "the output is too large"})
		} else {
			return
		}
	}
	h.wmu.Lock()
	defer h.wmu.Unlock()
	_, _ = h.out.Write(line)
}

func (h *host) load(m *msg) {
	sum := sha256.Sum256([]byte(m.Source))
	if hex.EncodeToString(sum[:]) != m.Hash {
		h.send(&msg{T: tLoaded, Hash: m.Hash, Error: "the script does not match its hash"})
		return
	}
	c, err := compile([]byte(m.Source))
	if err != nil {
		h.send(&msg{T: tLoaded, Hash: m.Hash, Error: err.Error()})
		return
	}
	h.mu.Lock()
	if len(h.files) >= 64 {
		h.files = map[string]*compiled{}
	}
	h.files[m.Hash] = c
	h.mu.Unlock()
	h.send(&msg{T: tLoaded, Hash: m.Hash})
}

// ask sends a callback and waits for zen's reply or the end of the call.
func (h *host) ask(seq int64) func(ctx context.Context, fn string, args ...any) (json.RawMessage, error) {
	return func(ctx context.Context, fn string, args ...any) (json.RawMessage, error) {
		raw := make([]json.RawMessage, len(args))
		for i, a := range args {
			b, err := json.Marshal(a)
			if err != nil {
				return nil, err
			}
			raw[i] = b
		}
		ch := make(chan *msg, 1)
		h.mu.Lock()
		h.cb++
		id := h.cb
		h.wait[id] = ch
		h.mu.Unlock()
		h.send(&msg{T: tCallback, Seq: seq, CB: id, Fn: fn, Args: raw})
		select {
		case r := <-ch:
			if r.Error != "" {
				return nil, errors.New(r.Error)
			}
			return r.Value, nil
		case <-ctx.Done():
			h.mu.Lock()
			delete(h.wait, id)
			h.mu.Unlock()
			return nil, ctx.Err()
		}
	}
}

func (h *host) call(m *msg) {
	res := &msg{T: tResult, Seq: m.Seq}
	defer func() {
		if r := recover(); r != nil {
			res = &msg{T: tResult, Seq: m.Seq, Error: "internal error in the script helper"}
		}
		h.send(res)
		if res.Kill {
			h.exit()
		}
	}()
	h.mu.Lock()
	c := h.files[m.Hash]
	h.mu.Unlock()
	if c == nil {
		res.Error = "the script is not loaded"
		return
	}
	g, lim := m.Globals, m.Limits
	if g == nil || lim == nil {
		res.Error = "bad call"
		return
	}
	timeout := time.Duration(lim.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > maxTimeout {
		timeout = defaultTimeout
	}
	maxOut := lim.MaxOutput
	if maxOut <= 0 || maxOut > maxOutput {
		maxOut = maxOutput
	}
	in := input{hook: m.Hook, name: g.Name, body: g.Body, attrs: g.Attrs}
	in.page.path, in.page.title = g.Path, g.Title
	if len(g.Config) > 0 {
		v, err := decodeJSON(g.Config)
		if err != nil {
			res.Error = "bad call"
			return
		}
		if cfg, ok := v.(map[string]any); ok {
			in.config = cfg
		}
	}
	if len(g.Payload) > 0 {
		in.payload = []byte(g.Payload)
	}
	z := &zenCall{call: h.ask(m.Seq)}
	run, err := c.exec(nil, timeout, in, z)
	if err != nil {
		res.Error = err.Error()
		// A script stopped by the deadline or the memory limit may still be
		// running in the background (Tengo cannot interrupt a builtin): this
		// helper must not serve another call.
		res.Kill = err == errMemory || strings.HasPrefix(res.Error, "timed out")
		return
	}
	out := run.Get("out").Object()
	switch m.Hook {
	case "":
		plug := run.Get("plugin").Object()
		res.Type = "undefined"
		res.Kind = kindNone
		if plug != nil && plug != tengo.UndefinedValue {
			res.Type = plug.TypeName()
			switch plug.(type) {
			case *tengo.Map, *tengo.ImmutableMap:
				v, err := toGo(plug)
				if err != nil {
					res.Error = "plugin: " + err.Error()
					return
				}
				b, err := json.Marshal(v)
				if err != nil {
					res.Error = "plugin: " + err.Error()
					return
				}
				res.Kind, res.JSON = kindMap, b
			}
		}
	case "action":
		data, err := outJSON(out, maxOut)
		if err != nil {
			res.Error = err.Error()
			return
		}
		res.Kind = kindNone
		if data != nil {
			res.Kind, res.JSON = kindJSON, data
		}
	default:
		s, set, err := outString(out, maxOut)
		if err != nil {
			res.Error = err.Error()
			return
		}
		res.Kind = kindNone
		if set {
			res.Kind, res.Text = kindText, s
		}
	}
}

func decodeJSON(b []byte) (any, error) {
	dec := json.NewDecoder(bytesReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("bad json: %w", err)
	}
	return v, nil
}

// HostMain is `zen __script-host`: it serves the helper protocol on stdin and
// stdout and returns the exit code. It refuses to run on a terminal. The
// protocol uses a private copy of stdout; fd 1 itself is pointed at stderr so
// that nothing else can write into the protocol.
func HostMain(stdin, stdout, stderr *os.File) int {
	if fi, err := stdin.Stat(); err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprintln(stderr, "zenmd: "+HostArg+" is for zen's own use and reads from a pipe")
		return 2
	}
	fd, err := syscall.Dup(int(stdout.Fd()))
	if err != nil {
		fmt.Fprintln(stderr, "zenmd: cannot start the script helper")
		return 1
	}
	proto := os.NewFile(uintptr(fd), "protocol")
	_ = syscall.Dup2(int(stderr.Fd()), int(stdout.Fd()))
	serve(stdin, proto, func() { os.Exit(0) })
	return 0
}
