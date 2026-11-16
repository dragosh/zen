package extplugins

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/dragosh/zen/internal/core/plugins"
)

// Stream runs the stream hook (ZEN_HOOK=stream): the payload goes to stdin and
// each line of stdout is handed to emit as it arrives (without its newline; a
// last line without one too). It returns nil when the program exits 0.
//
// Cancelling c kills the program's process group and returns c.Err(). The
// manifest's timeout (60s unless it gave one, at most 10m) ends it with a
// "timed out" error, an exit code other than 0 with "exit code N" and the
// first lines of stderr, and more than plugins.MaxStreamOutput bytes of
// output with plugins.ErrOutputLimit. emit is never called after Stream
// returned. Streams do not take one of the four backend slots of renders: the
// caller limits how many run.
func (p *Plugin) Stream(c context.Context, ctx plugins.Context, payload []byte, emit func(chunk []byte)) error {
	if err := p.declared("stream"); err != nil {
		return err
	}
	if err := c.Err(); err != nil {
		return err
	}
	sp, err := p.spawn(call{hook: "stream", ctx: ctx, stdin: payload})
	if err != nil {
		return err
	}
	cmd := sp.cmd
	errc := &collector{limit: maxStderr, done: make(chan struct{})}
	go errc.run(sp.errR)

	var (
		mu     sync.Mutex // guards closed and emit
		closed bool
		over   bool
	)
	send := func(b []byte) {
		mu.Lock()
		defer mu.Unlock()
		if !closed {
			emit(b)
		}
	}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		total := 0
		var pending []byte
		buf := make([]byte, 32<<10)
		for {
			n, err := sp.outR.Read(buf)
			for _, b := range buf[:n] {
				total++
				if total > plugins.MaxStreamOutput {
					mu.Lock()
					over = true
					mu.Unlock()
					killGroup(cmd)
					return
				}
				if b == '\n' {
					send(append([]byte(nil), pending...))
					pending = pending[:0]
				} else {
					pending = append(pending, b)
				}
			}
			if err != nil {
				if len(pending) > 0 {
					send(append([]byte(nil), pending...))
				}
				return
			}
		}
	}()

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	timer := time.NewTimer(p.streamTimeout())
	defer timer.Stop()
	var waitErr error
	timedOut, cancelled := false, false
	select {
	case waitErr = <-waited:
	case <-c.Done():
		cancelled = true
		killGroup(cmd)
		waitErr = <-waited
	case <-timer.C:
		timedOut = true
		killGroup(cmd)
		waitErr = <-waited
	}
	killGroup(cmd) // children left behind do not outlive the stream
	grace := time.NewTimer(pipeDrainWait)
	for _, d := range []chan struct{}{readDone, errc.done} {
		select {
		case <-d:
		case <-grace.C:
		}
	}
	grace.Stop()
	mu.Lock()
	closed = true
	hitLimit := over
	mu.Unlock()
	sp.outR.Close()
	sp.errR.Close()
	sp.inW.Close()

	switch {
	case cancelled || c.Err() != nil:
		return c.Err()
	case timedOut:
		return fmt.Errorf("%w after %s", errTimeout, p.streamTimeout())
	case hitLimit:
		return plugins.ErrOutputLimit
	case waitErr != nil:
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			return fmt.Errorf("exit code %d%s", ee.ExitCode(), stderrSummary(errc.buf))
		}
		return errors.New("the plugin did not finish")
	}
	return nil
}
