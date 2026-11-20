package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// LogCapacity is how many entries the app keeps for the Developer panel.
const LogCapacity = 2000

// MaxLogMessage is the longest message kept, in bytes; longer ones are cut.
const MaxLogMessage = 2000

// LogEntry is one line of the app's log, as the Developer panel shows it.
type LogEntry struct {
	Seq     int    `json:"seq"`     // 1, 2, 3, ... for the life of the ring
	Time    int64  `json:"time"`    // Unix milliseconds
	Level   string `json:"level"`   // debug | info | warn | error
	Source  string `json:"source"`  // app | plugin
	Message string `json:"message"` // never page contents or secrets: callers log paths, ids and error text
}

// Emitter delivers an event to the windows; *Sink is one.
type Emitter interface {
	Emit(name string, data any)
}

// LogRing is a bounded, concurrency-safe in-memory log: the newest capacity
// entries, each with a sequence number.
type LogRing struct {
	mu      sync.Mutex
	buf     []LogEntry // circular, len == capacity once full
	cap     int
	last    int // sequence number of the newest entry
	waiters map[chan struct{}]struct{}
}

// NewLogRing returns a ring that keeps the newest capacity entries.
func NewLogRing(capacity int) *LogRing {
	if capacity < 1 {
		capacity = 1
	}
	return &LogRing{cap: capacity, waiters: map[chan struct{}]struct{}{}}
}

func cleanLevel(l string) string {
	switch l {
	case "debug", "info", "warn", "error":
		return l
	}
	return "info"
}

func truncate(s string) string {
	if len(s) <= MaxLogMessage {
		return s
	}
	s = s[:MaxLogMessage]
	for !utf8.ValidString(s) { // do not cut a character in half
		s = s[:len(s)-1]
	}
	return s + "…"
}

// Add appends an entry and returns it. An unknown level is info and an empty
// source is "app".
func (r *LogRing) Add(level, source, message string) LogEntry {
	if source == "" {
		source = "app"
	}
	r.mu.Lock()
	r.last++
	e := LogEntry{Seq: r.last, Time: time.Now().UnixMilli(), Level: cleanLevel(level), Source: source, Message: truncate(message)}
	if len(r.buf) < r.cap {
		r.buf = append(r.buf, e)
	} else {
		r.buf[(e.Seq-1)%r.cap] = e
	}
	for w := range r.waiters {
		select {
		case w <- struct{}{}:
		default:
		}
	}
	r.mu.Unlock()
	return e
}

// Since returns the kept entries with a sequence number above seq, oldest first.
func (r *LogRing) Since(seq int) []LogEntry { return r.since(seq, 0) }

func (r *LogRing) since(seq, max int) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	oldest := r.last - len(r.buf) + 1 // sequence number of the oldest kept entry
	from := seq + 1
	if from < oldest {
		from = oldest
	}
	n := r.last - from + 1
	if n <= 0 {
		return []LogEntry{}
	}
	if max > 0 && n > max {
		n = max
	}
	out := make([]LogEntry, 0, n)
	for s := from; s < from+n; s++ {
		out = append(out, r.buf[(s-1)%r.cap])
	}
	return out
}

// Stream sends new entries to sink as event, in batches of at most batch
// entries and at most one event per every, so a chatty source cannot flood
// the windows (the ring still holds everything it has room for). It returns a
// function that stops it.
func (r *LogRing) Stream(sink Emitter, event string, every time.Duration, batch int) (stop func()) {
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	r.mu.Lock()
	r.waiters[wake] = struct{}{}
	sent := r.last // only entries added from now on
	r.mu.Unlock()
	var once sync.Once
	go func() {
		for {
			select {
			case <-done:
				return
			case <-wake:
			}
			for {
				entries := r.since(sent, batch)
				if len(entries) == 0 {
					break
				}
				sent = entries[len(entries)-1].Seq
				sink.Emit(event, entries)
				select {
				case <-done:
					return
				case <-time.After(every):
				}
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
			r.mu.Lock()
			delete(r.waiters, wake)
			r.mu.Unlock()
		})
	}
}

// logHandler writes every record to the ring and, when it is enabled there,
// to tee (stderr).
type logHandler struct {
	ring  *LogRing
	tee   slog.Handler
	attrs []slog.Attr
	lim   *debugLimiter // shared by every handler derived from this one
}

// DebugPerSecond is how many debug entries of one kind (one message) the log
// keeps each second; the rest are counted in a single "… N more" entry.
const DebugPerSecond = 20

// debugLimiter keeps a busy debug source (a file event per file, a render per
// page) from filling the ring.
type debugLimiter struct {
	mu    sync.Mutex
	kinds map[string]*debugBucket
}

type debugBucket struct {
	start   time.Time
	n       int
	dropped int
	timer   *time.Timer
}

// allow reports whether a debug entry of this kind may be kept now. The first
// entry dropped in a window schedules the "… N more" entry for its end.
func (l *debugLimiter) allow(ring *LogRing, kind string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.kinds[kind]
	if b == nil {
		b = &debugBucket{start: now}
		l.kinds[kind] = b
	}
	if now.Sub(b.start) >= time.Second {
		b.start, b.n = now, 0
	}
	if b.n < DebugPerSecond {
		b.n++
		return true
	}
	b.dropped++
	if b.timer == nil {
		b.timer = time.AfterFunc(time.Second-now.Sub(b.start), func() {
			l.mu.Lock()
			n := b.dropped
			b.dropped, b.timer = 0, nil
			l.mu.Unlock()
			if n > 0 {
				ring.Add("debug", "app", fmt.Sprintf("… %d more %s", n, kind))
			}
		})
	}
	return false
}

// LogApp logs a message from the app itself (source "app") at level, with
// key, value attributes that show after the message. It goes through
// log/slog, so it reaches the Console and stderr like any other record.
// Callers pass folder-relative paths and never file contents or option values.
func LogApp(level slog.Level, msg string, attrs ...any) {
	slog.Log(context.Background(), level, msg, append(attrs[:len(attrs):len(attrs)], "source", "app")...)
}

// NewLogHandler returns a slog handler that keeps every record (debug and up)
// in r and passes the ones tee accepts on to it. A "source" attribute ("app",
// "plugin") picks the entry's source; it is not repeated in the message.
func NewLogHandler(r *LogRing, tee slog.Handler) slog.Handler {
	return &logHandler{ring: r, tee: tee, lim: &debugLimiter{kinds: map[string]*debugBucket{}}}
}

func (h *logHandler) Enabled(context.Context, slog.Level) bool { return true }

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	}
	return "debug"
}

func (h *logHandler) Handle(ctx context.Context, rec slog.Record) error {
	if rec.Level < slog.LevelInfo && !h.lim.allow(h.ring, rec.Message) {
		return nil
	}
	source := "app"
	var b strings.Builder
	b.WriteString(rec.Message)
	add := func(a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Key == "source" {
			// The Console has three Go-side sources; the folder's own notices are the app's.
			if source = a.Value.String(); source == "workspace" {
				source = "app"
			}
			return
		}
		if a.Equal(slog.Attr{}) {
			return
		}
		b.WriteString(" " + a.Key + "=" + a.Value.String())
	}
	for _, a := range h.attrs {
		add(a)
	}
	rec.Attrs(func(a slog.Attr) bool { add(a); return true })
	h.ring.Add(levelName(rec.Level), source, b.String())
	if h.tee != nil && h.tee.Enabled(ctx, rec.Level) {
		return h.tee.Handle(ctx, rec)
	}
	return nil
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	if h.tee != nil {
		n.tee = h.tee.WithAttrs(attrs)
	}
	return &n
}

// WithGroup is ignored for the ring (attribute keys stay flat); the tee gets it.
func (h *logHandler) WithGroup(name string) slog.Handler {
	n := *h
	if h.tee != nil {
		n.tee = h.tee.WithGroup(name)
	}
	return &n
}
