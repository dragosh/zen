// Package lint runs the Vale prose linter on markdown content.
package lint

import (
	"context"
	"errors"
	"time"
)

// ErrUnavailable is returned when no vale binary can be found or run.
var ErrUnavailable = errors.New("lint: vale is not available")

// MaxContent is the largest content Lint passes to vale.
const MaxContent = 1 << 20

// Alert is one finding.
type Alert struct {
	Line, Col, EndCol int
	Severity          string
	Check, Message    string
	Match             string
}

// Linter runs vale.
type Linter struct {
	path    string
	timeout time.Duration
	sem     chan struct{}
}

func Find() (string, error) { return "", ErrUnavailable }

func find(exe string, getenv func(string) string, lookPath func(string) (string, error)) (string, error) {
	return "", ErrUnavailable
}

func isExec(p string) bool { return false }

func New(valePath string) *Linter { return &Linter{path: valePath} }

func (l *Linter) Lint(ctx context.Context, root, path string, content []byte) ([]Alert, error) {
	return nil, errors.New("not implemented")
}

func parse(data, content []byte) ([]Alert, error) { return nil, errors.New("not implemented") }
