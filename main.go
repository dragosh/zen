package main

import (
	"bytes"
	"embed"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"regexp"
	"text/template"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/webview/webview"
	"github.com/yuin/goldmark"
)

// The go embed directive statement must be outside of function body
// Embed the file content as string.
//go:embed web/static/title.txt
var title string

// Embed the entire directory.
//go:embed web/templates
var indexHTML embed.FS

//go:embed web/static
var staticFiles embed.FS

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {

	// Note the call to ParseFS instead of Parse
	t, err := template.ParseFS(indexHTML, "web/templates/index.html.tmpl")
	check(err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	check(err)
	defer ln.Close()

	// http.FS can be used to create a http Filesystem
	var staticFS = http.FS(staticFiles)
	fs := http.FileServer(staticFS)

	// Watcher
	w := watcher.New()
	// SetMaxEvents to 1 to allow at most 1 event's to be received
	// on the Event channel per watching cycle.
	//
	// If SetMaxEvents is not set, the default is to send all events.
	w.SetMaxEvents(1)

	// Only notify rename and move events.
	// w.FilterOps(watcher.Rename, watcher.Move)

	// Only files that match the regular expression during file listings
	// will be watched.

	r := regexp.MustCompile("^README.md$")
	w.AddFilterHook(watcher.RegexFilterHook(r, false))

	go func() {
		for {
			select {
			case event := <-w.Event:
				log.Println("File Changed: ", event) // Print the event's info.
			case err := <-w.Error:
				log.Fatalln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	cwd, err := os.Getwd()
	check(err)
	// Watch this folder for changes.
	if err := w.Add(cwd + "/README.md"); err != nil {
		log.Fatalln(err)
	}

	// markdown
	var buf bytes.Buffer

	// Print a list of all of the files and folders currently
	// being watched and their paths.
	for path := range w.WatchedFiles() {
		//fmt.Printf("%s: %s\n", path, f.Name())
		source, err := ioutil.ReadFile(path)
		check(err)
		log.Println("Watching", path)

		if err := goldmark.Convert(source, &buf); err != nil {
			panic(err)
		}
	}

	// if err := w.AddRecursive(cwd); err != nil {
	// 	log.Fatalln(err)
	// }

	// Trigger 2 events after watcher started.
	go func() {
		w.Wait()
	}()

	go func() {
		// Start the watching process - it'll check for changes every 100ms.
		if err := w.Start(time.Millisecond * 100); err != nil {
			log.Fatalln(err)
		}
	}()

	go func() {

		// Serve static files
		http.Handle("/web/", fs)
		http.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
			os.Exit(0)
		})
		// Handle all other requests
		http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			var path = req.URL.Path
			log.Println("Serving request for path", path)
			w.Header().Add("Content-Type", "text/html")
			u, _ := user.Current()

			// respond with the output of template execution
			t.Execute(w, struct {
				Title    string
				Response string
				User     *user.User
			}{Title: title, Response: path, User: u})

			err = t.ExecuteTemplate(w, "Markdown", buf.String())
			check(err)
		})

		log.Fatal(http.Serve(ln, nil))
	}()

	debug := true
	wv := webview.New(debug)
	//defer wv.Destroy()

	wv.Dispatch(func() {
		wv.SetTitle(title)
	})
	wv.SetSize(800, 600, webview.HintNone)
	wv.Navigate("http://" + ln.Addr().String())
	wv.Bind("quit", func() {
		wv.Terminate()
	})

	wv.Run()
}
