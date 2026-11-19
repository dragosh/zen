package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zenmark-org/zenmd/kit/assembly"
)

// TestAssemblyGenerated: assembly_gen.go is what the generator writes for
// app-assembly.yml today; after editing the YAML run `task build` (or
// go run ./kit/assembly/cmd/gen apps/zenmark).
func TestAssemblyGenerated(t *testing.T) {
	cat, err := assembly.BuiltinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	a, err := assembly.Load("app-assembly.yml", cat)
	if err != nil {
		t.Fatal(err)
	}
	want, err := assembly.Generate(a, "app")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("assembly_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("assembly_gen.go is out of date with app-assembly.yml; run: go run ./kit/assembly/cmd/gen apps/zenmark")
	}
	if _, err := os.Stat(filepath.Join("app-assembly.yml")); err != nil {
		t.Error(err)
	}
}
