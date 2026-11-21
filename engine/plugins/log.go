package plugins

import (
	"context"
	"log/slog"
)

// Log writes one entry for the plugin to the app log, with source "plugin" and
// the message prefixed with "<id>: ". Built-ins use it so they need not know
// where the log goes. Callers log ids, keys, counts and durations, never page
// text, code or answers.
func Log(id string, level slog.Level, msg string, attrs ...any) {
	attrs = append(attrs, "source", "plugin")
	slog.Log(context.Background(), level, id+": "+msg, attrs...)
}
