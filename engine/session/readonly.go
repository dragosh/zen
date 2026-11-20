package session

import "errors"

// ErrReadOnly is what every write of a read-only session returns (an app built
// without the editor feature, specs/kit/features). It reaches the frontend as
// a plain error message.
var ErrReadOnly = errors.New("this app is read-only")

// writable is the guard at the top of every method that changes the open
// folder or the settings files: nil, or ErrReadOnly when the session was
// created with Options.ReadOnly. It lives here, not in the app, so a button, a
// script in a page and the CLI binary are all covered.
func (s *Session) writable() error {
	if s.readOnly {
		return ErrReadOnly
	}
	return nil
}
