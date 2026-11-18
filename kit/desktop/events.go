package desktop

import (
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Sink emits events to the running application's windows; events before
// Run started the application are dropped. It implements session.Events.
type Sink struct {
	app atomic.Pointer[application.App]
}

// NewSink returns a Sink that is not attached yet; Run attaches it.
func NewSink() *Sink { return &Sink{} }

// Emit sends an event to every window.
func (s *Sink) Emit(name string, data any) {
	if a := s.app.Load(); a != nil {
		a.Event.Emit(name, data)
	}
}

func (s *Sink) attach(a *application.App) { s.app.Store(a) }
