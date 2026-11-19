package session

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/engine/lint"
	"github.com/zenmark-org/zenmd/engine/workspace"
)

// LintAlert and LintResult are the JSON shapes of Lint (ui/lint).
type LintAlert struct {
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	EndCol   int    `json:"endCol"`
	Severity string `json:"severity"`
	Check    string `json:"check"`
	Message  string `json:"message"`
	Match    string `json:"match"`
	// Why the check flagged this, and where to read more (https or "").
	Description string `json:"description"`
	Link        string `json:"link"`
}

// LintResult is what Lint returns: whether the linter is available and its alerts.
type LintResult struct {
	Available bool        `json:"available"`
	Alerts    []LintAlert `json:"alerts"`
}

// TextLinter is what Lint asks for alerts: *lint.Linter (Vale), or a fake in tests.
type TextLinter interface {
	Lint(ctx context.Context, root, path string, content []byte) ([]lint.Alert, error)
}

// The one linter of the process. An add-on installs a finder with SetLinter;
// the finder runs once, on first use. Sharing the linter keeps core/lint's
// limit of concurrent vale processes for all windows. Without a finder (an app
// built without the vale add-on) there is no linter and nothing is looked up.
var linters struct {
	mu    sync.Mutex
	find  func() TextLinter
	found bool
	l     TextLinter
}

func currentLinter() TextLinter {
	linters.mu.Lock()
	defer linters.mu.Unlock()
	if linters.find == nil {
		return nil
	}
	if !linters.found {
		linters.l, linters.found = linters.find(), true
	}
	return linters.l
}

// SetLinter installs the finder of the text linter (a nil finder: no linter,
// Lint is unavailable) and returns a func that puts the previous one back.
// The finder is called once, when the first Lint needs it; it returns nil when
// there is no linter (no vale binary). Add-ons call it from their init
// (addons/vale); nothing else looks for a linter.
func SetLinter(find func() TextLinter) (restore func()) {
	linters.mu.Lock()
	defer linters.mu.Unlock()
	oldFind, oldFound, oldL := linters.find, linters.found, linters.l
	linters.find, linters.found, linters.l = find, false, nil
	return func() {
		linters.mu.Lock()
		defer linters.mu.Unlock()
		linters.find, linters.found, linters.l = oldFind, oldFound, oldL
	}
}

func lintOff() LintResult { return LintResult{Alerts: []LintAlert{}} }

// Lint returns Vale's alerts for a page: content, or the file on disk when
// content is "". Unavailable without the linter add-on, without vale or with `lint: false`.
func (s *Session) Lint(path, content string) (LintResult, error) {
	if _, ok := configWhich(path); ok {
		return lintOff(), nil // settings files are not prose
	}
	return s.lint(currentLinter(), path, content)
}

// lint checks path (a workspace markdown page) and lints content, or the
// file on disk when content is "". Without vale, or with the `lint` plugin
// switch off for the page, the result is unavailable.
func (s *Session) lint(l TextLinter, path, content string) (LintResult, error) {
	if l == nil {
		return lintOff(), nil
	}
	if !isMarkdownPath(path) {
		return LintResult{}, s.cleanErr(path, workspace.ErrNotMarkdown)
	}
	s.mu.Lock()
	if s.ws == nil {
		s.mu.Unlock()
		return LintResult{}, errNoWorkspace
	}
	var src []byte
	if content == "" {
		b, err := s.ws.Read(path)
		if err != nil {
			err = s.cleanErr(path, err)
			s.mu.Unlock()
			return LintResult{}, err
		}
		src = b
	} else {
		if _, err := s.ws.Resolve(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			err = s.cleanErr(path, err)
			s.mu.Unlock()
			return LintResult{}, err
		}
		src = []byte(content)
	}
	root, cfg := s.ws.Root(), s.cfg
	s.mu.Unlock()

	fm, _, _ := config.SplitFrontmatter(src)
	if on, _, mentioned := cfg.ForPage(fm).Plugin("lint"); mentioned && !on {
		return lintOff(), nil
	}

	alerts, err := l.Lint(context.Background(), root, path, src)
	if errors.Is(err, lint.ErrUnavailable) {
		return lintOff(), nil
	}
	if err != nil {
		msg := strings.ReplaceAll(err.Error(), root+string(os.PathSeparator), "")
		return LintResult{}, errors.New(strings.ReplaceAll(msg, root, "."))
	}
	out := LintResult{Available: true, Alerts: make([]LintAlert, 0, len(alerts))}
	for _, a := range alerts {
		out.Alerts = append(out.Alerts, LintAlert{
			Line: a.Line, Col: a.Col, EndCol: a.EndCol, Severity: a.Severity,
			Check: a.Check, Message: a.Message, Match: a.Match,
			Description: a.Description, Link: a.Link,
		})
	}
	return out, nil
}
