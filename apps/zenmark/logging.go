package app

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/zenmark-org/zenmd/kit/desktop"
)

// logStartup is the first line of the Console: what started.
func logStartup() {
	b := buildInfo()
	desktop.LogApp(slog.LevelInfo, Name+" started", "version", b.Version, "channel", desktop.Channel(), "features", strings.Join(b.Features, ","))
}

// scrub removes what a log must not show from text: the open folder's path
// becomes ".", the home folder "~".
func (z *Zen) scrub(text string) string {
	if ws := z.s.Workspace(); ws != nil && ws.Root != "" {
		text = strings.ReplaceAll(text, ws.Root+string(filepath.Separator), "")
		text = strings.ReplaceAll(text, ws.Root, ".")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		text = strings.ReplaceAll(text, home, "~")
	}
	return text
}

// logCall logs, at error level, a service method that returned an error to
// the UI: the method and the message the UI got. Every method that returns
// an error defers it with a pointer to its error result. A cancelled open is
// not a failure.
func (z *Zen) logCall(method string, err *error) {
	if *err == nil || errors.Is(*err, ErrOpenCancelled) {
		return
	}
	desktop.LogApp(slog.LevelError, "service call failed", "method", method, "error", z.scrub((*err).Error()))
}

// logHandlerPanic logs a panic in an HTTP handler; the caller panics again.
func logHandlerPanic(v any) {
	desktop.LogApp(slog.LevelError, "panic in handler", "panic", toText(v))
}

func toText(v any) string {
	switch x := v.(type) {
	case error:
		return x.Error()
	case string:
		return x
	}
	return "unexpected value"
}

// recovering logs a panic in next's handlers and lets it go on, as net/http
// would have handled it without this.
func recovering(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				logHandlerPanic(v)
				panic(v)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
