package app

import (
	"log/slog"
	"testing"
	"time"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

func TestLogsReadsTheRing(t *testing.T) {
	z, _, _, _ := newTestZen(t, tree(t, docs), "", "read")
	if got := z.Logs(0); got == nil || len(got) != 0 {
		t.Fatalf("without a ring Logs = %#v, want an empty list (JSON [])", got)
	}
	z.logs = desktop.NewLogRing(5)
	z.logs.Add("warn", "plugin", "one")
	z.logs.Add("info", "app", "two")
	if got := z.Logs(0); len(got) != 2 || got[0].Message != "one" {
		t.Errorf("Logs(0) = %+v", got)
	}
	if got := z.Logs(1); len(got) != 1 || got[0].Seq != 2 {
		t.Errorf("Logs(1) = %+v", got)
	}
}

func TestInstallLoggingTeesSlogAndEmits(t *testing.T) {
	ev := &sink{}
	ring, undo := installLogging(ev)
	slog.Info("hello", "k", "v")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range ev.all() {
			if batch, ok := e.Data.([]LogEntry); ok && e.Name == EventLogEntry && len(batch) > 0 && batch[0].Message == "hello k=v" {
				undo()
				if got := ring.Since(0); len(got) != 1 {
					t.Errorf("ring = %+v", got)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	undo()
	t.Fatal("no log:entry event for the slog record")
}
