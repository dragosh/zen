// Package render turns markdown into sanitised HTML.
package render

import (
	"errors"

	"github.com/dragosh/zen/internal/core/config"
	"github.com/dragosh/zen/internal/core/plugins"
	"github.com/dragosh/zen/internal/core/workspace"
)

type Resolver interface {
	ResolveLink(from, href string) workspace.Link
}

type Page struct {
	Path, Title, HTML string
	Frontmatter       map[string]any
	Assets            plugins.Assets
	Warnings          []string
}

type Renderer struct{}

func New(reg *plugins.Registry, cfg *config.Config, links Resolver) *Renderer { return &Renderer{} }

func (r *Renderer) Render(path string, src []byte) (Page, error) {
	return Page{}, errors.New("not implemented")
}

func (r *Renderer) Links(path string, src []byte) []workspace.Link { return nil }

func HTML(src []byte) (string, error) { return "", errors.New("not implemented") }
