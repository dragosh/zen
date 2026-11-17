package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

// EventPluginStream carries the output of a plugin stream to the frontend:
// payload PluginStreamEvent. A chunk has Chunk set; the last event of a call
// has Done true and Error "" or the reason it failed ("cancelled",
// "output limit", the plugin's message).
const EventPluginStream = "plugin:stream"

// PluginStreamEvent is the payload of EventPluginStream. Output is batched:
// at most one chunk event per streamBatch per call, its Chunk the lines
// produced since the last event joined with "\n" (no trailing newline; a
// chunk is split on "\n" to get its lines, an empty line is an empty piece).
type PluginStreamEvent struct {
	Call  string `json:"call"`
	Chunk string `json:"chunk"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}

const (
	maxStreams  = 4                     // streams running at once, per session
	streamBatch = 50 * time.Millisecond // at most one chunk event per stream in this time
)

// builtinStreamTimeout ends a built-in plugin's stream (external plugins have
// their manifest's timeout, which is never longer).
var builtinStreamTimeout = 10 * time.Minute

// streamCall is one running stream.
type streamCall struct {
	cancel    context.CancelFunc
	cancelled bool   // guarded by streams.mu
	plugin    string // id of the plugin that runs it
	page      string // page it was started for
}

type streams struct {
	mu    sync.Mutex
	calls map[string]*streamCall
	wg    sync.WaitGroup
}

func newCallID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errors.New("could not start the stream")
	}
	return hex.EncodeToString(b[:]), nil
}

// PluginCancel cancels a running stream; it ends with error "cancelled". A
// finished or unknown call is not an error.
func (s *Session) PluginCancel(call string) error {
	s.streams.cancel(call)
	return nil
}

func (st *streams) cancel(call string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if c := st.calls[call]; c != nil {
		c.cancelled = true
		c.cancel()
	}
}

// cancelPlugin cancels the running streams of plugin id.
func (st *streams) cancelPlugin(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, c := range st.calls {
		if c.plugin == id {
			c.cancelled = true
			c.cancel()
		}
	}
}

// reconcileStreams cancels the streams whose plugin is no longer allowed, or
// no longer switched on for the page it was started for.
func (s *Session) reconcileStreams() {
	type run struct{ call, plugin, page string }
	st := &s.streams
	st.mu.Lock()
	var runs []run
	for id, c := range st.calls {
		runs = append(runs, run{id, c.plugin, c.page})
	}
	st.mu.Unlock()
	for _, r := range runs {
		if _, err := s.target(r.plugin, r.page, true); err != nil {
			st.cancel(r.call)
		}
	}
}

// cancelAll cancels every running stream (the folder changed, the app quits).
func (st *streams) cancelAll() {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, c := range st.calls {
		c.cancelled = true
		c.cancel()
	}
}

// wait blocks until every stream ended, at most d.
func (st *streams) wait(d time.Duration) {
	done := make(chan struct{})
	go func() { st.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// PluginStream starts the plugin's stream hook with a JSON payload and
// returns the id of the call. Output arrives as EventPluginStream events.
//
// page is the workspace path of the page the block is on (see PluginInvoke).
func (s *Session) PluginStream(id, page, payload string) (string, error) {
	if !json.Valid([]byte(payload)) {
		return "", errors.New("the payload is not valid JSON")
	}
	t, err := s.target(id, page, true)
	if err != nil {
		return "", err
	}
	h, ok := plugins.StreamHookOf(t.p)
	if !ok {
		return "", fmt.Errorf("plugin %q does not declare the stream hook", id)
	}
	call, err := newCallID()
	if err != nil {
		return "", err
	}
	c, cancel := context.WithTimeout(context.Background(), builtinStreamTimeout)
	sc := &streamCall{cancel: cancel, plugin: id, page: page}
	st := &s.streams
	st.mu.Lock()
	if len(st.calls) >= maxStreams {
		st.mu.Unlock()
		cancel()
		return "", fmt.Errorf("at most %d streams run at once", maxStreams)
	}
	if st.calls == nil {
		st.calls = map[string]*streamCall{}
	}
	st.calls[call] = sc
	st.wg.Add(1)
	st.mu.Unlock()

	go func() {
		defer st.wg.Done()
		defer cancel()
		b := &batcher{call: call, send: s.emit, c: c}
		emit := func(line []byte) {
			if !b.add(line) {
				cancel() // over the output limit
			}
		}
		err := func() (err error) {
			defer func() {
				if recover() != nil {
					err = errors.New("the plugin failed")
				}
			}()
			return h.Stream(c, t.ctx, []byte(payload), emit)
		}()
		st.mu.Lock()
		wasCancelled := sc.cancelled
		delete(st.calls, call) // free the slot before the frontend hears about the end
		st.mu.Unlock()
		b.finish(!wasCancelled) // the last lines go out before the end of the stream
		s.events.Emit(EventPluginStream, PluginStreamEvent{Call: call, Done: true, Error: streamError(err, c, wasCancelled, b.limited())})
	}()
	return call, nil
}

// streamError is the text of a stream's ending, "" for success.
func streamError(err error, c context.Context, cancelled, limited bool) string {
	switch {
	case cancelled:
		return "cancelled"
	case limited || errors.Is(err, plugins.ErrOutputLimit):
		return plugins.ErrOutputLimit.Error()
	case errors.Is(c.Err(), context.DeadlineExceeded) && (err == nil || errors.Is(err, context.DeadlineExceeded)):
		return fmt.Sprintf("timed out after %s", builtinStreamTimeout)
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case err != nil:
		return err.Error()
	}
	return ""
}

// batcher turns a stream's lines into at most one event per streamBatch. The
// output limit counts every line once, its newline included (empty lines too).
type batcher struct {
	call string
	send func(name string, data any)
	c    context.Context

	mu      sync.Mutex
	lines   []string
	total   int
	over    bool
	armed   bool
	stopped bool
}

// add queues a line; it reports false once the output limit is passed.
func (b *batcher) add(line []byte) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped || b.over || b.c.Err() != nil {
		return !b.over
	}
	b.total += len(line) + 1
	if b.total > plugins.MaxStreamOutput {
		b.over = true
		return false
	}
	b.lines = append(b.lines, string(line))
	if !b.armed {
		b.armed = true
		time.AfterFunc(streamBatch, b.flush)
	}
	return true
}

func (b *batcher) flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.armed = false
	b.flushLocked()
}

func (b *batcher) flushLocked() {
	if len(b.lines) == 0 || b.stopped {
		return
	}
	chunk := strings.Join(b.lines, "\n")
	b.lines = b.lines[:0]
	b.send(EventPluginStream, PluginStreamEvent{Call: b.call, Chunk: chunk})
}

// finish sends what is queued (unless the stream was cancelled) and stops the batcher: nothing is sent after.
func (b *batcher) finish(flush bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if flush {
		b.flushLocked()
	}
	b.stopped = true
}

func (b *batcher) limited() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.over
}
