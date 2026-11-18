package engine_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// publicRoots are the trees whose packages other programs may import.
var publicRoots = []string{".", "../builtins"}

// TestExportedAreDocumented walks every package under engine/ and builtins/
// and checks that it has a package doc and that every exported function,
// method, type, constant and variable has a doc comment starting with its
// name (a grouped declaration may be covered by the comment on the group).
func TestExportedAreDocumented(t *testing.T) {
	var problems []string
	for _, root := range publicRoots {
		err := filepath.WalkDir(root, func(dir string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			problems = append(problems, checkDir(t, dir)...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

func checkDir(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for name, pkg := range pkgs {
		if name == "main" {
			continue
		}
		documented := false
		for _, f := range pkg.Files {
			if f.Doc != nil && strings.HasPrefix(strings.TrimSpace(f.Doc.Text()), "Package "+name) {
				documented = true
			}
		}
		if !documented {
			out = append(out, dir+": package "+name+" has no package doc starting with \"Package "+name+"\"")
		}
		for fname, f := range pkg.Files {
			for _, decl := range f.Decls {
				out = append(out, checkDecl(fset, filepath.ToSlash(fname), decl)...)
			}
		}
	}
	return out
}

func checkDecl(fset *token.FileSet, file string, decl ast.Decl) []string {
	var out []string
	bad := func(pos token.Pos, name, why string) {
		out = append(out, file+":"+itoa(fset.Position(pos).Line)+": "+name+" "+why)
	}
	startsWith := func(g *ast.CommentGroup, name string) bool {
		return g != nil && strings.HasPrefix(strings.TrimSpace(g.Text()), name)
	}
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return nil
		}
		name := d.Name.Name
		if d.Recv != nil && len(d.Recv.List) > 0 {
			recv := recvName(d.Recv.List[0].Type)
			if !ast.IsExported(recv) {
				return nil
			}
			name = recv + "." + name
		}
		if !startsWith(d.Doc, d.Name.Name) {
			bad(d.Pos(), name, "has no doc comment starting with its name")
		}
	case *ast.GenDecl:
		if d.Tok == token.IMPORT {
			return nil
		}
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if !s.Name.IsExported() {
					continue
				}
				doc := s.Doc
				if doc == nil && !d.Lparen.IsValid() {
					doc = d.Doc
				}
				if !startsWith(doc, s.Name.Name) {
					bad(s.Pos(), s.Name.Name, "has no doc comment starting with its name")
				}
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if !n.IsExported() {
						continue
					}
					doc := s.Doc
					if doc == nil && d.Lparen.IsValid() {
						if d.Doc != nil {
							continue // covered by the comment on the group
						}
						// A comment-less group: fall through to the error.
					}
					if doc == nil && !d.Lparen.IsValid() {
						doc = d.Doc
					}
					if !startsWith(doc, n.Name) {
						bad(n.Pos(), n.Name, "has no doc comment starting with its name")
					}
				}
			}
		}
	}
	return out
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
