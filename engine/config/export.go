package config

import (
	"fmt"
	"path/filepath"
)

// The website export's folder settings are under the export: key of
// .zenmd.yml and the global config.yml. Their old key was build:; it is still
// read, and these notices (Problems, the log) say how to move on.
const (
	legacyKeyNotice = "build: in %s is now export:; rename the key"
	bothKeysNotice  = "build: in %s is ignored because export: is set"
)

// legacyBuild handles a build: key found in the file at path: with no export:
// key it is read as export: and noted; with both, it is ignored and noted.
func (c *Config) legacyBuild(path string, doc map[string]any, v any) {
	name := filepath.Base(path)
	if _, both := doc["export"]; both {
		c.notices = append(c.notices, fmt.Sprintf(bothKeysNotice, name))
		return
	}
	c.notices = append(c.notices, fmt.Sprintf(legacyKeyNotice, name))
	c.mergeBuild(v)
}

// Where an export goes when nothing says otherwise, relative to the folder.
// Both are kept out of the pages and the watcher by their .zen-build marker or
// by never holding markdown (see workspace.Files).
const (
	DefaultExportOut = "exports/html" // the website
	DefaultPDFDir    = "exports/pdf"  // the PDF
)
