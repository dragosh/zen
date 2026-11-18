package scriptplugins

import (
	"syscall"
	"time"
)

// SetRSSLimit lowers the resident-size limit and the polling period (tests).
func SetRSSLimit(kb int64, poll time.Duration) (restore func()) {
	oldKB, oldPoll := rssLimitKB.Load(), rssPoll.Load()
	rssLimitKB.Store(kb)
	rssPoll.Store(int64(poll))
	return func() { rssLimitKB.Store(oldKB); rssPoll.Store(oldPoll) }
}

// CrashHelpers kills every helper with SIGKILL, as a crash would.
func CrashHelpers() {
	defaultHelpers.mu.Lock()
	var pids []int
	for h := range defaultHelpers.all {
		pids = append(pids, h.pid)
	}
	defaultHelpers.mu.Unlock()
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
