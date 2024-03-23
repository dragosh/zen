package web

// #cgo pkg-config: gtk+-3.0 webkit2gtk-4.0
// #include <gtk/gtk.h>
// #include <webkit2/webkit2.h>

import (
	"embed"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"

	"github.com/dragosh/zen/internal/errors"
	httpServer "github.com/dragosh/zen/internal/server/http"
	"github.com/dragosh/zen/internal/watcher"
	"github.com/dragosh/zen/pkg/api"
	webview "github.com/jchv/go-webview-selector"
)

import (
	"runtime"
	/*
		#cgo darwin LDFLAGS: -framework CoreGraphics
		#cgo linux pkg-config: x11
		#if defined(__APPLE__)
		#include <CoreGraphics/CGDisplayConfiguration.h>
		int display_width() {
				return CGDisplayPixelsWide(CGMainDisplayID());
		}
		int display_height() {
				return CGDisplayPixelsHigh(CGMainDisplayID());
		}
		#endif

		#if defined(_WIN32)
		int show_window(void* hWndPtr, int nCmdShow) {
			HWND hWnd = (HWND)(hWndPtr);
			return ShowWindow(hWnd, nCmdShow);
		}
		#endif
	*/
	"C"
)

var (
	mux    = &sync.RWMutex{}
	pCount = 0
)

var title string = "Title"

//go:embed apps
var webApps embed.FS

// // go:embed apps/default/index.tpl.html
// var defaultIndex embed.FS

//go:embed apps/markdown/index.tpl.html
var markdownIndex embed.FS

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	errors.Handle(err)
}

func initJS(jsWindowInitObj string) string {
	const WEB_NAMESPACE = api.GLOBAL_NAMESPACE
	js := fmt.Sprintf(`window.%s = %s;`, WEB_NAMESPACE, jsWindowInitObj)
	api.Log.Debugf("[WEBVIEW] init window with: %s", js)
	return js
}

func bindFunctions(wv webview.WebView) {
	wv.Bind("quit", func() {
		wv.Terminate()
	})

	wv.Bind("open", func(url string) {
		log.Println("Open browser to: ", url)
		openBrowser(url)
	})
}

// func startCounting(wv webview.WebView) {
// 	for {
// 		mux.RLock()
// 		js := fmt.Sprintf(`document.getElementById("counter").innerHTML = %d;`, pCount)
// 		wv.Dispatch(func() {
// 			wv.Eval(js)
// 		})
// 		mux.RUnlock()
// 		pCount += 1
// 		time.Sleep(1 * time.Second)
// 	}
// }

func runWebView(ln net.Listener, statics api.EmbededApp, entry api.DocPreviewEntry) {

	url := "http://" + ln.Addr().String()
	//url := startServer()

	wv := webview.New(api.LOG_LEVEL == "debug") //debug

	width := 1024 // default
	height := 768 // default

	if runtime.GOOS == "darwin" {
		width = int(C.display_width())
		height = int(C.display_height())
	}
	wv.SetSize(width, height, webview.HintNone)

	// enhance it
	jsWindowInitObj := fmt.Sprintf("{webviewWidth: %d, webviewHeight : %d}", width, height)

	wv.Init(initJS(jsWindowInitObj))

	go func() {
		httpServer.Start(ln, statics, entry)
	}()

	wv.Navigate(url)
	wv.Dispatch(func() {
		wv.SetTitle(entry.FileName)
	})

	// Watcher
	go func() {
		// todo ignore specific folders as default relative to root path
		//  `node_modules` `.hidden_folders` etc
		api.Log.Debugf("[WATCH] looking up files in: %s", entry.FileRoot)
		watcher.Start(entry.FileRoot, entry.FileName, func(path string) {
			// @todo - do action based on path/extension
			wv.Eval("reload(true)")
			api.Log.Debugf("[WATCH] file changed: %s", path)
		})
	}()
	defer wv.Destroy()

	bindFunctions(wv)
	// go startCounting(wv)
	wv.Run()
}

func runBrowser(ln net.Listener, entry api.DocPreviewEntry) {
	// httpServer.Start(ln, staticFiles, defaultIndex, entry)
}

func Start(entry api.DocPreviewEntry) error {
	var err error
	var ln net.Listener
	var statics api.EmbededApp
	statics.StaticFiles = webApps
	switch entry.StaticAppName {
	case "markdown":
		statics.DefaultIndex = markdownIndex
		statics.DefaultPath = "apps/markdown/"
	case "default":
		// statics.DefaultIndex = defaultIndex
		// statics.DefaultPath = "apps/default/"
	default:
		api.Log.Fatalf("Unable to render application layout: '%s'", entry.StaticAppName)
	}

	addr := "127.0.0.1:0"
	ln, err = net.Listen("tcp", addr)

	if err != nil {
		return err
	}
	defer ln.Close()

	// go func() {
	// 	grpcServer.Start()
	// }()

	if api.DEV_MODE {
		runBrowser(ln, entry)
	} else {
		runWebView(ln, statics, entry)
	}

	return nil
}
