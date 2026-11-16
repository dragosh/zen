package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

// EventPluginStream carries the output of a plugin stream to the frontend:
// payload PluginStreamEvent. A chunk has Chunk set; the last event of a call
// has Done true and Error "" or the reason it failed ("cancelled",
// "output limit", the plugin's message).
const EventPluginStream = "plugin:stream"

// PluginStreamEvent is the payload of EventPluginStream.
type PluginStreamEvent struct {
	Call  string `json:"call"`
	Chunk string `json:"chunk"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}

const maxStreams = 4 // streams running at once, per app

// builtinStreamTimeout ends a built-in plugin's stream (external plugins have
// their manifest's timeout, which is never longer).
var builtinStreamTimeout = 10 * time.Minute

// streamCall is one running stream.
type streamCall struct {
	cancel    context.CancelFunc
	cancelled bool // guarded by streams.mu
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

// PluginStream starts the plugin's stream hook with a JSON payload and
// returns the id of the call. Output arrives as EventPluginStream events.
func (z *Zen) PluginStream(id, payload string) (string, error) {
	return z.state.startStream(id, payload)
}

// PluginCancel cancels a running stream; it ends with error "cancelled". A
// finished or unknown call is not an error.
func (z *Zen) PluginCancel(call string) error {
	z.state.streams.cancel(call)
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

func (s *state) startStream(id, payload string) (string, error) {
	if !json.Valid([]byte(payload)) {
		return "", errors.New("the payload is not valid JSON")
	}
	t, err := s.target(id, true)
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
	sc := &streamCall{cancel: cancel}
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
		total, limited := 0, false
		emit := func(b []byte) {
			if c.Err() != nil || limited {
				return
			}
			if total+len(b) > plugins.MaxStreamOutput {
				limited = true
				cancel()
				return
			}
			total += len(b)
			s.events.Emit(EventPluginStream, PluginStreamEvent{Call: call, Chunk: string(b)})
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
		s.events.Emit(EventPluginStream, PluginStreamEvent{Call: call, Done: true, Error: streamError(err, c, wasCancelled, limited)})
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
