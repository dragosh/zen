package app

import (
	"os"
	"path/filepath"

	"github.com/dragosh/zen/internal/core/render"
)

// Document is a file opened from the command line.
type Document struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
	// HTML is Content rendered and sanitised.
	HTML string `json:"html"`
	Error   string `json:"error,omitempty"`
}

// Documents exposes the startup files to the frontend.
type Documents struct {
	files []string
}

// Startup returns the files passed on the command line with their content.
func (d *Documents) Startup() []Document {
	docs := make([]Document, 0, len(d.files))
	for _, p := range d.files {
		doc := Document{Path: p, Name: filepath.Base(p)}
		if b, err := os.ReadFile(p); err != nil {
			doc.Error = err.Error()
		} else {
			doc.Content = string(b)
			if doc.HTML, err = render.HTML(b); err != nil {
				doc.Error = err.Error()
			}
		}
		docs = append(docs, doc)
	}
	return docs
}
