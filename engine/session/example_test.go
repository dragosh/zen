package session_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/dragosh/zen/engine/session"
)

// Open examples/demo, list the tree, render README.md and build the site.
func Example_renderFolder() {
	runtime := fstest.MapFS{
		"static/zen.js":  {Data: []byte("/* runtime */")},
		"static/zen.css": {Data: []byte("/* styles */")},
	}
	s := session.New(session.Options{Runtime: runtime})
	defer s.Close()
	demo := filepath.Join("..", "..", "examples", "demo")
	if err := s.Open(demo); err != nil {
		fmt.Println(err)
		return
	}
	ws := s.Workspace()
	fmt.Println(ws.Name, ws.Home)
	pg, err := s.Page(ws.Home)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(strings.Contains(pg.HTML, "<h1"))
	out, err := os.MkdirTemp("", "zenmd-example")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(out)
	res, err := s.Build(context.Background(), session.BuildOptions{Out: filepath.Join(out, "site"), Portable: true}, nil)
	fmt.Println(err, res.Pages > 0)
	// Output:
	// demo README.md
	// true
	// <nil> true
}
