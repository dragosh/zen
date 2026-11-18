package scriptplugins

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Pool and limits of the helper processes (see the spec, "Isolation").
const (
	defaultPool = 2
	maxPool     = 4
	hardCap     = maxPool + 12 // helpers parked in callbacks do not count against the pool

	killGrace    = 300 * time.Millisecond // after the call's timeout, before zen kills the helper
	acquireLimit = 10 * time.Second
	crashLimit   = 3
	crashWindow  = time.Minute
)

var (
	// rssLimitKB is the resident size at which zen kills a helper; rssPoll
	// (nanoseconds) is how often it is looked at.
	rssLimitKB atomic.Int64
	rssPoll    atomic.Int64
)

func init() {
	rssLimitKB.Store(512 << 10)
	rssPoll.Store(int64(250 * time.Millisecond))
}

// Crash messages (page warnings and error boxes show them with the plugin's
// name).
var (
	errCrashed = errors.New("the script plugin crashed")
	errBusy    = errors.New("the script plugin helpers are busy")
)

type helper struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	dir     string // private working directory
	loaded  map[string]bool
	retired bool         // discard after the current call
	reason  atomic.Value // string: why zen killed it ("" = it died by itself)
	dead    atomic.Bool
	pid     int
}

type pool struct {
	mu      sync.Mutex
	size    int
	cmdFn   func() (string, []string)
	idle    []*helper
	total   int
	parked  int
	changed chan struct{}
	crashes map[string][]time.Time
	off     map[string]bool
	all     map[*helper]bool
}

var defaultHelpers = newPool()

func newPool() *pool {
	return &pool{size: defaultPool, changed: make(chan struct{}), crashes: map[string][]time.Time{},
		off: map[string]bool{}, all: map[*helper]bool{}}
}

// SetHelperCommand sets the program that serves as the helper (default: the
// running binary with HostArg). Tests use it to start the test binary or a
// fake. It stops the running helpers.
func SetHelperCommand(path string, args ...string) {
	defaultHelpers.mu.Lock()
	defaultHelpers.cmdFn = func() (string, []string) { return path, args }
	defaultHelpers.mu.Unlock()
	Reset()
}

// SetPoolSize sets how many helpers compute at once (1 to 4).
func SetPoolSize(n int) {
	if n < 1 {
		n = 1
	}
	if n > maxPool {
		n = maxPool
	}
	defaultHelpers.mu.Lock()
	defaultHelpers.size = n
	defaultHelpers.mu.Unlock()
}

// Reset ends the idle helpers and retires the busy ones (they end after the
// call they are in): zen calls it when the folder changes.
func Reset() { defaultHelpers.stop(false) }

// Shutdown kills every helper: zen calls it on quit.
func Shutdown() { defaultHelpers.stop(true) }

// HelperCount is the number of helper processes alive (for tests).
func HelperCount() int {
	defaultHelpers.mu.Lock()
	defer defaultHelpers.mu.Unlock()
	return len(defaultHelpers.all)
}

// ResetCrashes forgets the crash counts and the switched-off plugins (tests).
func ResetCrashes() {
	defaultHelpers.mu.Lock()
	defer defaultHelpers.mu.Unlock()
	defaultHelpers.crashes = map[string][]time.Time{}
	defaultHelpers.off = map[string]bool{}
}

func (p *pool) notifyLocked() {
	close(p.changed)
	p.changed = make(chan struct{})
}

func (p *pool) stop(all bool) {
	p.mu.Lock()
	var kill []*helper
	for _, h := range p.idle {
		kill = append(kill, h)
	}
	p.idle = nil
	for h := range p.all {
		if all {
			kill = append(kill, h)
		} else {
			h.retired = true
		}
	}
	p.mu.Unlock()
	for _, h := range kill {
		p.discard(h, "stop")
	}
}

// discard kills h and forgets it.
func (p *pool) discard(h *helper, reason string) {
	h.kill(reason)
	p.mu.Lock()
	if p.all[h] {
		delete(p.all, h)
		p.total--
		p.notifyLocked()
	}
	for i, x := range p.idle {
		if x == h {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
}

func (h *helper) kill(reason string) {
	if h.dead.Swap(true) {
		return
	}
	h.reason.CompareAndSwap(nil, reason)
	if h.cmd != nil && h.cmd.Process != nil {
		_ = syscall.Kill(-h.pid, syscall.SIGKILL)
		_ = h.cmd.Process.Kill()
		go func() { _ = h.cmd.Wait() }()
	}
	h.stdin.Close()
	if h.dir != "" {
		_ = os.RemoveAll(h.dir)
	}
}

func (h *helper) why() string {
	if s, ok := h.reason.Load().(string); ok {
		return s
	}
	return ""
}

func (p *pool) command() (string, []string, error) {
	p.mu.Lock()
	fn := p.cmdFn
	p.mu.Unlock()
	if fn != nil {
		path, args := fn()
		return path, args, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", nil, errors.New("the script helper could not be found")
	}
	return exe, []string{HostArg}, nil
}

// helperEnv is all a helper gets of the environment.
func helperEnv() []string {
	var env []string
	for _, k := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (p *pool) start() (*helper, error) {
	path, args, err := p.command()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "zen-script-")
	if err != nil {
		return nil, errors.New("the script helper could not be started")
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = helperEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	in, err := cmd.StdinPipe()
	if err != nil {
		os.RemoveAll(dir)
		return nil, errors.New("the script helper could not be started")
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		os.RemoveAll(dir)
		return nil, errors.New("the script helper could not be started")
	}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, errors.New("the script helper could not be started")
	}
	return &helper{cmd: cmd, stdin: in, stdout: bufio.NewReaderSize(out, 64<<10), dir: dir,
		loaded: map[string]bool{}, pid: cmd.Process.Pid}, nil
}

func (p *pool) acquire(ctx context.Context) (*helper, error) {
	deadline := time.NewTimer(acquireLimit)
	defer deadline.Stop()
	for {
		p.mu.Lock()
		if n := len(p.idle); n > 0 {
			h := p.idle[n-1]
			p.idle = p.idle[:n-1]
			p.mu.Unlock()
			return h, nil
		}
		if p.total < hardCap && p.total-p.parked < p.size {
			p.total++
			p.mu.Unlock()
			h, err := p.start()
			p.mu.Lock()
			if err != nil {
				p.total--
				p.notifyLocked()
				p.mu.Unlock()
				return nil, err
			}
			p.all[h] = true
			p.mu.Unlock()
			return h, nil
		}
		ch := p.changed
		p.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, errCancelled
		case <-deadline.C:
			return nil, errBusy
		}
	}
}

func (p *pool) release(h *helper, healthy bool) {
	p.mu.Lock()
	excess := p.total > p.size
	retired := h.retired
	p.mu.Unlock()
	if !healthy || retired || excess || h.dead.Load() {
		p.discard(h, "done")
		return
	}
	p.mu.Lock()
	p.idle = append(p.idle, h)
	p.notifyLocked()
	p.mu.Unlock()
}

func (p *pool) setParked(delta int) {
	p.mu.Lock()
	p.parked += delta
	p.notifyLocked()
	p.mu.Unlock()
}

// recordCrash notes a crash of the plugin key and reports whether it is now
// switched off.
func (p *pool) recordCrash(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	list := p.crashes[key][:0]
	for _, t := range p.crashes[key] {
		if now.Sub(t) < crashWindow {
			list = append(list, t)
		}
	}
	list = append(list, now)
	p.crashes[key] = list
	if len(list) >= crashLimit {
		p.off[key] = true
	}
	return p.off[key]
}

func (p *pool) isOff(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.off[key]
}

// request is one call to a helper.
type request struct {
	key     string // the plugin, for crash counts
	id      string
	hash    string
	source  string
	hook    string
	globals *wireGlobals
	timeout time.Duration
	ctx     context.Context // ends the call (folder change, quit)
	handle  func(fn string, args []json.RawMessage) (any, error)
}

var seqCounter atomic.Int64

func errSwitchedOff() error {
	return fmt.Errorf("the script plugin was switched off after %d crashes in a minute (edit the script or restart zen to try again)", crashLimit)
}

func (p *pool) do(rq request) (*msg, error) {
	if rq.ctx == nil {
		rq.ctx = context.Background()
	}
	if rq.ctx.Err() != nil {
		return nil, errCancelled
	}
	if p.isOff(rq.key) {
		return nil, errSwitchedOff()
	}
	h, err := p.acquire(rq.ctx)
	if err != nil {
		return nil, err
	}
	res, healthy, err := p.run(h, rq)
	p.release(h, healthy)
	if err == errCrashed && p.recordCrash(rq.key) {
		return nil, errSwitchedOff()
	}
	return res, err
}

func (h *helper) write(m *msg) error {
	line, err := encodeLine(m)
	if err != nil {
		return err
	}
	_, err = h.stdin.Write(line)
	return err
}

// run does one call on h. healthy says whether h may serve another call.
func (p *pool) run(h *helper, rq request) (res *msg, healthy bool, err error) {
	seq := seqCounter.Add(1)
	// A cancelled context kills the helper (the call is over).
	stopWatch := context.AfterFunc(rq.ctx, func() { h.kill("cancel") })
	defer stopWatch()

	fail := func(e error) (*msg, bool, error) {
		switch h.why() {
		case "timeout":
			return nil, false, fmt.Errorf("timed out after %s", rq.timeout)
		case "cancel", "stop":
			return nil, false, errCancelled
		case "memory":
			return nil, false, errMemory
		}
		if rq.ctx.Err() != nil {
			return nil, false, errCancelled
		}
		h.kill("crash")
		return nil, false, errCrashed
	}

	if !h.loaded[rq.hash] {
		if err := h.write(&msg{T: tLoad, ID: rq.id, Hash: rq.hash, Source: rq.source}); err != nil {
			return fail(err)
		}
	}
	callMsg := &msg{T: tCall, Seq: seq, ID: rq.id, Hash: rq.hash, Hook: rq.hook, Globals: rq.globals,
		Limits: &wireLimits{TimeoutMS: int(rq.timeout / time.Millisecond), MaxOutput: maxOutput}}
	if rq.timeout < time.Millisecond {
		callMsg.Limits.TimeoutMS = 1
	}
	if err := h.write(callMsg); err != nil {
		if errors.Is(err, errLineTooLong) {
			return nil, true, errors.New("the input is too large")
		}
		return fail(err)
	}
	// The wall clock of the call, callbacks included.
	timer := time.AfterFunc(rq.timeout+killGrace, func() { h.kill("timeout") })
	defer timer.Stop()
	stopRSS := make(chan struct{})
	defer close(stopRSS)
	go watchRSS(h, stopRSS)

	var loadErr string
	for {
		line, err := readLine(h.stdout, maxLine)
		if err != nil {
			return fail(err)
		}
		var m msg
		if err := json.Unmarshal(line, &m); err != nil {
			return fail(err) // garbage
		}
		switch m.T {
		case tLoaded:
			if m.Error != "" {
				loadErr = m.Error
			} else {
				h.loaded[m.Hash] = true
			}
		case tResult:
			if m.Seq != seq {
				return fail(errors.New("unexpected result"))
			}
			if loadErr != "" {
				return nil, true, errors.New(loadErr)
			}
			if m.Error != "" {
				return nil, !m.Kill, errors.New(m.Error)
			}
			return &m, true, nil
		case tCallback:
			if m.Seq != seq {
				return fail(errors.New("unexpected callback"))
			}
			p.setParked(1)
			value, herr := rq.handle(m.Fn, m.Args)
			p.setParked(-1)
			reply := &msg{T: tReply, CB: m.CB}
			if herr != nil {
				reply.Error = herr.Error()
			} else if b, merr := json.Marshal(value); merr != nil {
				reply.Error = "the answer cannot be sent"
			} else {
				reply.Value = b
			}
			if err := h.write(reply); err != nil {
				if errors.Is(err, errLineTooLong) {
					reply = &msg{T: tReply, CB: m.CB, Error: "the answer is too large"}
					if err = h.write(reply); err == nil {
						continue
					}
				}
				return fail(err)
			}
		default:
			return fail(errors.New("unexpected message"))
		}
	}
}

// watchRSS kills h when its resident size passes rssLimitKB.
func watchRSS(h *helper, stop <-chan struct{}) {
	t := time.NewTicker(time.Duration(rssPoll.Load()))
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if rssKB(h.pid) > rssLimitKB.Load() {
				h.kill("memory")
				return
			}
		}
	}
}

func rssKB(pid int) int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n
}
