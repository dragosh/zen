package session

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// slowIndex is how long indexing a folder may take before it is logged as a warning.
const slowIndex = 2 * time.Second

// logApp logs a message of the app itself (source "app"). The Console shows
// the attributes after the message. Callers pass folder-relative paths, never
// file contents or option values (specs/app/logging).
func logApp(level slog.Level, msg string, attrs ...any) {
	slog.Log(context.Background(), level, msg, append(attrs[:len(attrs):len(attrs)], "source", "app")...)
}

// relPath is abs as a path relative to root, with forward slashes. A path
// outside root is only its file name, so a log never has a full path.
func relPath(root, abs string) string {
	if root != "" && filepath.IsAbs(abs) {
		if r, err := filepath.Rel(root, abs); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(r)
		}
		return filepath.Base(abs)
	}
	return filepath.ToSlash(abs)
}

// scrubRoot removes the folder's absolute path from text a log line shows.
func scrubRoot(root, text string) string {
	if root == "" {
		return text
	}
	text = strings.ReplaceAll(text, root+string(filepath.Separator), "")
	return strings.ReplaceAll(text, root, ".")
}

// logRendered logs a page render at debug, and each warning the page got at warn
// (the config notice has its own entry).
func logRendered(root string, p Page, took time.Duration, notice string) {
	logApp(slog.LevelDebug, "page rendered", "path", p.Path, "ms", took.Milliseconds())
	for _, w := range p.Warnings {
		if w == notice {
			continue // logged once, when the folder opened
		}
		logApp(slog.LevelWarn, "render warning", "path", p.Path, "warning", scrubRoot(root, w))
	}
}

// logTrust logs the user's decision about a plugin or layout.
func logTrust(kind, id string, allow bool) {
	verb := "blocked"
	if allow {
		verb = "allowed"
	}
	logApp(slog.LevelInfo, kind+" "+verb, "id", id)
}

// logFail logs, at error level, a method the service calls that returned an
// error to the UI: its name and the message the UI got (already without
// absolute paths; the folder's own is removed from the rest). Methods defer it
// first, with a pointer to their error result, so it runs after they unlock.
func (s *Session) logFail(method string, err *error) {
	if *err == nil || errors.Is(*err, ErrOpenCancelled) {
		return
	}
	root := ""
	s.mu.Lock()
	if s.ws != nil {
		root = s.ws.Root()
	}
	s.mu.Unlock()
	msg := scrubRoot(root, (*err).Error())
	if home, herr := os.UserHomeDir(); herr == nil && home != "" && home != "/" {
		msg = strings.ReplaceAll(msg, home, "~")
	}
	logApp(slog.LevelError, "service call failed", "method", method, "error", msg)
}
