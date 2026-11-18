package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// delegating are the Zen methods that have a counterpart in engine/session.
// Each one must be a one-line delegation: a single statement that calls a
// method of the session (z.s) and returns its result.
var delegating = []string{
	"Page", "PageInfo", "Source", "Preview", "Save", "Lint", "CancelOpen", "CreatePage",
	"Plugins", "PluginTrust", "PluginStorageGet", "PluginStorageSet", "PluginStorageKeys",
	"PluginInvoke", "PluginStream", "PluginCancel",
	"Layouts", "LayoutTrust", "LayoutStorageGet", "LayoutStorageSet", "LayoutStorageKeys",
	"BuildOptions", "BuildWebsite", "CancelBuild", "SaveBuildDefaults",
	"SiteTemplates", "TemplatesDir", "ReorderNav", "SetNavHidden",
}

func TestAppIsThin(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	found := map[string]*ast.FuncDecl{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			if star, ok := fd.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok && id.Name == "Zen" {
					found[fd.Name.Name] = fd
				}
			}
		}
	}
	for _, name := range delegating {
		fd := found[name]
		if fd == nil {
			t.Errorf("Zen.%s is missing", name)
			continue
		}
		if len(fd.Body.List) != 1 {
			t.Errorf("Zen.%s has %d statements, want one delegation to the session", name, len(fd.Body.List))
			continue
		}
		if !callsSession(fd.Body.List[0]) {
			t.Errorf("Zen.%s does not delegate to z.s", name)
		}
	}
}

// callsSession reports whether stmt is `return z.s.X(...)` or `z.s.X(...)`.
func callsSession(stmt ast.Stmt) bool {
	var e ast.Expr
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return false
		}
		e = s.Results[0]
	case *ast.ExprStmt:
		e = s.X
	default:
		return false
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// z.s.X, or z.s.X().Y for the templates API.
	for {
		switch x := sel.X.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "z" && x.Sel.Name == "s" {
				return true
			}
			sel = x
		case *ast.CallExpr:
			inner, ok := x.Fun.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			sel = inner
		default:
			return false
		}
	}
}
