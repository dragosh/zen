package site

import (
	"regexp"
	"sort"
)

// A module that imports another file cannot be loaded from a Blob URL (there
// is nothing to resolve the import against). The check is a scan, not a
// parser: it finds `import x from`, `import "x"`, `import {…}`, `import(…)`
// and `export … from`.
var (
	staticImportRE  = regexp.MustCompile(`(?m)(^|[;}])[ \t]*(import\s*[\w{*$"']|export\s*(\*|\{[^}]*\})\s*from\b)`)
	dynamicImportRE = regexp.MustCompile(`\bimport\s*\(`)
)

func usesImports(src []byte) bool {
	return staticImportRE.Match(src) || dynamicImportRE.Match(src)
}

// moduleFiles groups the script files of a page by module, sorted.
type moduleFiles struct {
	key   string
	files map[string]string // file -> text
}

func sortedModules(m map[string]map[string]string) []moduleFiles {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]moduleFiles, 0, len(keys))
	for _, k := range keys {
		out = append(out, moduleFiles{key: k, files: m[k]})
	}
	return out
}
