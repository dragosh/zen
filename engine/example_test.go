package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/zenmark-org/zenmd/builtins"
	"github.com/zenmark-org/zenmd/engine/session"
)

// Example_embed opens a folder with the built-in plugins and layouts, lists
// the sidebar, renders a page and builds a static site. It is everything a
// program needs to embed zenmd.
func Example_embed() {
	runtime := fstest.MapFS{ // the app's static runtime; a real program passes frontend/dist
		"static/zen.js":  {Data: []byte("/* runtime */")},
		"static/zen.css": {Data: []byte("/* styles */")},
	}
	s := session.New(session.Options{
		Registry: builtins.Plugins(), Layouts: builtins.Layouts("dev"),
		Presets: builtins.SiteTemplates(), Runtime: runtime,
	})
	defer s.Close()
	if err := s.Open(filepath.Join("..", "examples", "demo")); err != nil {
		fmt.Println(err)
		return
	}
	ws := s.Workspace()
	for _, n := range ws.Tree {
		fmt.Println(n.Title)
	}
	page, err := s.Page("README.md")
	fmt.Println(err, strings.Contains(page.HTML, "<h1"))
	out, _ := os.MkdirTemp("", "zenmd-embed")
	defer os.RemoveAll(out)
	res, err := s.Build(context.Background(), session.BuildOptions{Out: filepath.Join(out, "site"), Portable: true}, nil)
	fmt.Println(err, res.Pages > 0)
	// Output:
	// zen.md user guide
	// Other pages
	// <nil> true
	// <nil> true
}
