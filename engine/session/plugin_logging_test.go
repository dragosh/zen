package session

import (
	"log/slog"
	"regexp"
	"strings"
	"testing"
)

// Every action call and every stream of any plugin logs at debug, with the op
// and the duration; the payload and the answer never appear.
func TestPluginCallsAreLogged(t *testing.T) {
	h := &recHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := pluginTree(t, pluginDocs())
	z, ev := openActing(t, dir, t.TempDir(), newActor("act"))

	if _, err := z.PluginInvoke("act", "README.md", `{"op":"check-1","secret":"TOPSECRET-PAYLOAD"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginInvoke("act", "README.md", `{"op":"has space"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginInvoke("act", "README.md", `{"op":42}`); err != nil {
		t.Fatal(err)
	}
	if _, err := z.PluginInvoke("act", "README.md", `"TOPSECRET-STRING"`); err != nil {
		t.Fatal(err)
	}
	call, err := z.PluginStream("act", "README.md", `"three"`)
	if err != nil {
		t.Fatal(err)
	}
	finish(t, ev, call)

	got := h.all()
	for _, re := range []string{
		`(?m)^DEBUG act: action check-1 \(\d+ ms\) source=plugin$`,
		`(?m)^DEBUG act: action \(\d+ ms\) source=plugin$`,
		`(?m)^DEBUG act: stream started source=plugin$`,
		`(?m)^DEBUG act: stream ended \(\d+ ms\) source=plugin$`,
	} {
		if !regexp.MustCompile(re).MatchString(got) {
			t.Errorf("log lacks %s:\n%s", re, got)
		}
	}
	if n := strings.Count(got, "act: action ("); n != 3 {
		t.Errorf("%d op-less action entries, want 3:\n%s", n, got)
	}
	for _, bad := range []string{"TOPSECRET", "has space", "three", "payload"} {
		if strings.Contains(got, bad) {
			t.Errorf("the log has %q:\n%s", bad, got)
		}
	}
}

// A failing action keeps its warn entry and still logs the call.
func TestPluginFailedCallsStillLogged(t *testing.T) {
	h := &recHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := pluginTree(t, pluginDocs())
	z, _ := openActing(t, dir, t.TempDir(), newActor("act"))
	if _, err := z.PluginInvoke("act", "README.md", `"fail"`); err == nil {
		t.Fatal("want an error")
	}
	got := h.all()
	if !strings.Contains(got, "WARN plugin act: action failed source=plugin") {
		t.Errorf("the warn entry changed:\n%s", got)
	}
	if !regexp.MustCompile(`DEBUG act: action \(\d+ ms\)`).MatchString(got) {
		t.Errorf("no debug entry:\n%s", got)
	}
}
