package app

import (
	"log/slog"
	"os"
	"time"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

// LogEntry is one line of the log the Developer panel shows.
type LogEntry = desktop.LogEntry

// EventLogEntry carries a batch ([]LogEntry) of new log entries.
const EventLogEntry = "log:entry"

const (
	logEvery = 250 * time.Millisecond // at most one log:entry event per interval
	logBatch = 100                    // and at most this many entries in it
)

// Logs returns the log entries after sequence number since, oldest first;
// Logs(0) is everything kept (the newest desktop.LogCapacity).
func (z *Zen) Logs(since int) []LogEntry {
	if z.logs == nil {
		return []LogEntry{}
	}
	return z.logs.Since(since)
}

// installLogging makes log/slog (and the standard log package) write to
// stderr, as before, and to a ring the Developer panel reads, and sends new
// entries to the windows. It returns the ring and a function that undoes it.
func installLogging(sink EventSink) (*desktop.LogRing, func()) {
	ring := desktop.NewLogRing(desktop.LogCapacity)
	prev := slog.Default()
	slog.SetDefault(slog.New(desktop.NewLogHandler(ring, slog.NewTextHandler(os.Stderr, nil))))
	stop := ring.Stream(sink, EventLogEntry, logEvery, logBatch)
	return ring, func() {
		stop()
		slog.SetDefault(prev)
	}
}
