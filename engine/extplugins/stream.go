package extplugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/dragosh/zenmd/engine/plugins"
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
	var (
		mu     sync.Mutex // guards closed, over and emit
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
	readOut := func(r io.Reader) {
		total := 0
		var pending []byte
		buf := make([]byte, 32<<10)
		for {
			n, err := r.Read(buf)
			for _, b := range buf[:n] {
				total++
				if total > plugins.MaxStreamOutput {
					mu.Lock()
					over = true
					mu.Unlock()
					killGroup(sp.cmd)
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
	}
	res := spawnWait(c, sp, p.streamTimeout(), readOut)
	mu.Lock()
	closed = true
	hitLimit := over
	mu.Unlock()

	switch {
	case res.cancelled || c.Err() != nil:
		return c.Err()
	case res.timedOut:
		return fmt.Errorf("%w after %s", errTimeout, p.streamTimeout())
	case hitLimit:
		return plugins.ErrOutputLimit
	case res.waitErr != nil:
		var ee *exec.ExitError
		if errors.As(res.waitErr, &ee) {
			return fmt.Errorf("exit code %d%s", ee.ExitCode(), stderrSummary(res.stderr))
		}
		return errors.New("the plugin did not finish")
	}
	return nil
}
