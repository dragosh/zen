/*
* -----------------------------------------------------------
* Http Server
*
* -----------------------------------------------------------
 */

package server

import (
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/dragosh/zen/internal/errors"
	"github.com/dragosh/zen/pkg/api"
)

func close(write http.ResponseWriter, r *http.Request) {
	os.Exit(0)
}

// @todo  https://gist.github.com/hauxe/f2ea1901216177ccf9550a1b8bd59178#file-http_static_correct-go

func Start(ln net.Listener, statics api.EmbededApp, entry api.DocPreviewEntry) {

	var indexFile = "index.tpl.html"
	api.Log.Debug(statics.DefaultPath + indexFile)

	var t *template.Template
	var err error
	// Serve static files
	// http.FS can be used to create a http Filesystem
	if api.DEV_MODE {
		_, thisFile, _, _ := runtime.Caller(1)
		webDir := filepath.Dir(thisFile)
		staticDir := filepath.Join(webDir + "/static")
		fs := http.FileServer(http.Dir(staticDir))
		http.Handle("/static/", http.StripPrefix("/static", fs))
		indexPath := filepath.Join(webDir, "static/"+indexFile)
		api.Log.Debugf("Index file: %s", indexPath)
		api.Log.Debugf("Static directory: %s", staticDir)
		t, err = template.ParseFiles(indexPath)
	} else {
		t, err = template.ParseFS(statics.DefaultIndex, statics.DefaultPath+indexFile)
		http.Handle("/"+statics.DefaultPath, http.FileServer(http.FS(statics.StaticFiles)))
	}

	errors.Handle(err)
	http.HandleFunc("/close/", close)

	// Handle all other requests
	http.HandleFunc("/", func(write http.ResponseWriter, req *http.Request) {

		// lock to main entry fileroot
		var path = entry.FileRoot + req.URL.Path
		User, _ := user.Current()

		api.Log.Debugf("[HTTP] request file path: %s", path)

		if strings.HasSuffix(path, ".md") {

			buf, err := os.ReadFile(path)
			errors.Handle(err)
			write.Header().Add("Content-Type", "text/markdown")
			write.Write(buf)

		} else if strings.HasSuffix(path, ".svg") {

			buf, err := os.ReadFile(path)
			errors.Handle(err)
			write.Header().Add("Content-Type", "image/svg+xml")
			write.Write(buf)

		} else if strings.HasSuffix(path, ".png") {

			buf, err := os.ReadFile(path)
			errors.Handle(err)
			write.Header().Add("Content-Type", "image/png")
			write.Write(buf)

		} else if strings.HasSuffix(path, ".jpg") {

			buf, err := os.ReadFile(path)
			errors.Handle(err)
			write.Header().Add("Content-Type", "image/jpg")
			write.Write(buf)

		} else {
			write.Header().Add("Content-Type", "text/html")
			// respond with the output of template execution
			t.Execute(write, struct {
				Root        string
				Title       string
				Entry       string
				User        *user.User
				StaticsPath string
			}{Root: entry.FileRoot, Title: "Hello", User: User, Entry: entry.FileName, StaticsPath: statics.DefaultPath})
		}
	})

	api.Log.Debugf("[HTTP] server listening at %s://%s", "http", ln.Addr())
	api.Log.Fatal(http.Serve(ln, nil))

}
