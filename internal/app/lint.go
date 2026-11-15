package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/lint"
	"github.com/dragosh/zen/internal/core/workspace"
)

// LintAlert and LintResult are the JSON shapes of Zen.Lint (ui/lint).
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

type LintResult struct {
	Available bool        `json:"available"`
	Alerts    []LintAlert `json:"alerts"`
}

// textLinter is *lint.Linter; tests use a fake.
type textLinter interface {
	Lint(ctx context.Context, root, path string, content []byte) ([]lint.Alert, error)
}

// The one linter of the process, found on first use. Sharing it keeps
// core/lint's limit of concurrent vale processes for all windows.
var linters struct {
	mu    sync.Mutex
	found bool
	l     textLinter
}

func findLinter() textLinter {
	p, err := lint.Find()
	if err != nil {
		return nil
	}
	return lint.New(p)
}

func currentLinter() textLinter {
	linters.mu.Lock()
	defer linters.mu.Unlock()
	if !linters.found {
		linters.l, linters.found = findLinter(), true
	}
	return linters.l
}

// setLinter replaces the linter (nil: no vale) and returns a restore func.
func setLinter(l textLinter) (restore func()) {
	linters.mu.Lock()
	defer linters.mu.Unlock()
	oldFound, oldL := linters.found, linters.l
	linters.found, linters.l = true, l
	return func() {
		linters.mu.Lock()
		defer linters.mu.Unlock()
		linters.found, linters.l = oldFound, oldL
	}
}

func lintOff() LintResult { return LintResult{Alerts: []LintAlert{}} }

// lint checks path (a workspace markdown page) and lints content, or the
// file on disk when content is "". Without vale, or with the `lint` plugin
// switch off for the page, the result is unavailable.
func (s *state) lint(l textLinter, path, content string) (LintResult, error) {
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
