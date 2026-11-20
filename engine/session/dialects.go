package session

import "errors"

// DialectInfo is one markdown dialect of the build as Settings shows it.
type DialectInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	On      bool   `json:"on"`
}

func (s *Session) Dialects() []DialectInfo             { return nil }
func (s *Session) DialectHint() string                 { return "" }
func (s *Session) SetDialect(id string, on bool) error { return errors.New("todo") }
func (s *Session) DismissDialectHint(id string) error  { return errors.New("todo") }
