package app

// `zenmd export pdf`: the print document of a folder laid out in a hidden web
// view of a background app (no window shown, no Dock icon, no menu bar), saved
// as a PDF through macOS printing, then the app quits. It does not talk to a
// running Zenmark.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zenmark-org/zenmd/engine/config"
	"github.com/zenmark-org/zenmd/internal/cli"
	"github.com/zenmark-org/zenmd/kit/desktop"
)

// headlessTimeout is how long the page layout may take.
const headlessTimeout = 30 * time.Second

// headlessPlan is what the headless export runs: the output file and the window.
type headlessPlan struct {
	Out     string
	Name    string // the folder's name
	Timeout time.Duration
}

func planHeadless(e cli.ExportPDF, name string) headlessPlan {
	out := e.Out
	if out == "" {
		out = cli.DefaultPDFOut(e.Dir)
	}
	return headlessPlan{Out: out, Name: name, Timeout: headlessTimeout}
}

// windowOptions is the one hidden window, at the print preview's export mode.
func (p headlessPlan) windowOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:   printWindowName,
		Title:  p.Name + " — Print",
		URL:    "/?window=" + printWindowName + "&export=1&page=",
		Width:  printWindowWidth,
		Height: printWindowHeight,
		Hidden: true,
	}
}

// appOptions: an accessory app (no Dock icon, no menu bar) that ends by Quit only.
func (p headlessPlan) appOptions(assets application.AssetOptions, services []application.Service) application.Options {
	return application.Options{
		Name:     Name,
		Assets:   assets,
		Services: services,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	}
}

// headlessAssets serves the frontend (with its CSP) and the session's routes, as the app does.
func headlessAssets(z *Zen, frontend http.Handler) application.AssetOptions {
	return application.AssetOptions{Handler: frontend, Middleware: z.middleware}
}

func headlessServices(z *Zen, h *headlessService) []application.Service {
	return []application.Service{application.NewService(z), application.NewService(h)}
}

func headlessTimeoutError(d time.Duration) error {
	return fmt.Errorf("the page layout did not finish in %d s", int(d/time.Second))
}

// headlessService is the one method the preview calls in export mode.
type headlessService struct {
	save   func(path, title string) (int, error)
	out    string
	title  string
	stdout io.Writer
	stderr io.Writer

	mu    sync.Mutex
	state int // 0 waiting, 1 saving or done, 2 timed out
	done  chan error
}

func newHeadlessService(save func(path, title string) (int, error), out, title string, stdout, stderr io.Writer) *headlessService {
	return &headlessService{save: save, out: out, title: title, stdout: stdout, stderr: stderr, done: make(chan error, 1)}
}

// PrintReady is called once by the preview when the document is laid out; late
// names the blocks that were not ready in time (warnings). It saves the PDF.
func (h *headlessService) PrintReady(late []string) {
	h.mu.Lock()
	if h.state != 0 {
		h.mu.Unlock()
		return
	}
	h.state = 1
	h.mu.Unlock()
	for _, l := range late {
		fmt.Fprintf(h.stderr, "zenmd: warning: %s was not ready in time\n", oneLine(l))
	}
	pages, err := h.save(h.out, h.title)
	if err != nil {
		h.done <- err
		return
	}
	unit := "pages"
	if pages == 1 {
		unit = "page"
	}
	fmt.Fprintf(h.stdout, "Wrote %s (%d %s)\n", h.out, pages, unit)
	h.done <- nil
}

// PrintFailed is called once by the preview when it could not prepare the document; it ends the
// export at once with that message.
func (h *headlessService) PrintFailed(message string) {
	h.mu.Lock()
	if h.state != 0 {
		h.mu.Unlock()
		return
	}
	h.state = 1
	h.mu.Unlock()
	if message = oneLine(message); message == "" {
		message = "the document could not be prepared"
	}
	h.done <- errors.New("export pdf: " + message)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// wait returns when the PDF is written or has failed, or when d has passed.
func (h *headlessService) wait(d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case err := <-h.done:
		return err
	case <-t.C:
		h.mu.Lock()
		if h.state == 1 { // saving has begun: let it finish
			h.mu.Unlock()
			return <-h.done
		}
		h.state = 2
		h.mu.Unlock()
		return headlessTimeoutError(d)
	}
}

// ExportPDF is `zenmd export pdf`: it opens the folder, runs the hidden app and
// writes e.Out. Errors are one line.
func ExportPDF(e cli.ExportPDF, stdout, stderr io.Writer) error {
	if err := requireFeature("export"); err != nil {
		return err
	}
	if err := requireFeature("print"); err != nil {
		return err
	}
	z, err := newZen(zenOptions{
		Registry: builtinPlugins(), GlobalPath: globalConfigPath(), ConfigDir: configDirOf(),
		Dir: e.Dir, Mode: "read", Events: discardSink{},
	})
	if err != nil {
		return oneLineError(err)
	}
	defer z.close()
	ws := z.s.Workspace()
	if ws == nil || len(ws.Tree) == 0 {
		return fmt.Errorf("%s has no pages", filepath.Base(e.Dir))
	}
	if runtime.GOOS != "darwin" {
		return errors.New("export pdf needs macOS")
	}
	if !desktop.HasWindowServer() {
		return errors.New("export pdf needs a logged-in macOS session")
	}
	assets, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		return err
	}
	plan := planHeadless(e, ws.Name)
	var win application.Window
	h := newHeadlessService(func(path, title string) (int, error) {
		return desktop.SavePDF(win, path, title)
	}, plan.Out, plan.Name, stdout, stderr)
	a := application.New(plan.appOptions(headlessAssets(z, application.AssetFileServerFS(assets)), headlessServices(z, h)))
	win = a.Window.NewWithOptions(plan.windowOptions())
	result := make(chan error, 1)
	go func() {
		result <- h.wait(plan.Timeout)
		a.Quit()
	}()
	if err := a.Run(); err != nil {
		return oneLineError(err)
	}
	return <-result
}

func oneLineError(err error) error { return errors.New(oneLine(err.Error())) }

type discardSink struct{}

func (discardSink) Emit(string, any) {}

func globalConfigPath() string {
	p, _ := config.GlobalPath()
	return p
}

func configDirOf() string {
	if p := globalConfigPath(); p != "" {
		return filepath.Dir(p)
	}
	return ""
}
