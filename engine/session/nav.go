package session

import "errors"

func (s *Session) ReorderNav(parent string, paths []string) error { return errors.New("todo") }
func (s *Session) SetNavHidden(path string, hidden bool) error    { return errors.New("todo") }
func (s *Session) AbsPath(rel string) (string, error)             { return "", errors.New("todo") }
