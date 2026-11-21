// Package rating renders :::rating containers as a row of radio stars and
// :::rating-results as the list of every rating in the folder. The frontend is
// assets/rating.js; a rating is kept by it in ctx.storage under "rating-<id>".
package rating

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"regexp"
	"strconv"
	"strings"

	"github.com/zenmark-org/zenmd/engine/plugins"
)

//go:embed assets/rating.js assets/rating.css
var assets embed.FS

// The storage key is "rating-<id>" and keys hold at most 64 characters, so an
// id holds at most 57.
var idRE = regexp.MustCompile(`^[A-Za-z0-9._-]{1,57}$`)

const (
	defaultTitle = "Rate this page"
	defaultMax   = 5
	minMax       = 3
	maxMax       = 10
)

type plugin struct{}

// New returns the rating plugin.
func New() plugins.Plugin { return plugin{} }

func (plugin) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID:          "rating",
		Version:     "0.1.0",
		APIVersion:  plugins.APIVersion,
		Description: "Star ratings kept on this Mac",
		Default:     true,
		Help: plugins.Help{
			Summary: "A rating block asks one question and the reader picks 1 to 5 stars. The rating is kept on this Mac, and a results block lists every rating in the folder.",
			Snippets: []plugins.Snippet{
				{Title: "A rating", Markdown: ":::rating{id=course title=\"How was this course?\"}\n:::\n"},
				{Title: "Stars with labels", Markdown: "id names the stored rating and must be unique in the page. max is the number of stars (3 to 10), labels one label per star.\n\n:::rating{id=guide title=\"How useful was this page?\" max=3 labels=\"Not much|Useful|Very useful\"}\n:::\n"},
				{Title: "All your ratings", Markdown: ":::rating-results\n:::\n"},
			},
		},
		Blocks: []string{"rating", "rating-results"},
	}
}

func (plugin) RenderBlock(ctx plugins.Context, b plugins.Block) (string, error) {
	if b.Name == "rating-results" {
		return `<div class="zr-results"></div>`, nil
	}
	id := b.Attrs["id"]
	if !idRE.MatchString(id) {
		return "", fmt.Errorf("the rating id %q must be 1 to 57 letters, digits, . _ -", id)
	}
	title := b.Attrs["title"]
	if title == "" {
		title = defaultTitle
	}
	max := defaultMax
	if v, ok := b.Attrs["max"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < minMax || n > maxMax {
			return "", fmt.Errorf("max %q must be a number from %d to %d", v, minMax, maxMax)
		}
		max = n
	}
	var labels []string
	if v := b.Attrs["labels"]; v != "" {
		labels = strings.Split(v, "|")
		if len(labels) != max {
			return "", fmt.Errorf("labels has %d entries but there are %d stars; give one label per star, separated by |", len(labels), max)
		}
	}
	// The page sanitiser lets through div, span and class but no fieldset, label or
	// radio name, so Go renders the question and the stars as text; rating.js turns
	// them into a fieldset of radio inputs.
	var sb strings.Builder
	sb.WriteString(`<div class="zr-rating"><div class="zr-legend">` + html.EscapeString(title) + `</div>`)
	if text := strings.TrimSpace(b.Body); text != "" {
		sb.WriteString(`<div class="zr-text">` + bodyHTML(ctx, text) + `</div>`)
	}
	sb.WriteString(`<div class="zr-stars">`)
	for i := 1; i <= max; i++ {
		label := fmt.Sprintf("%d of %d", i, max)
		if labels != nil {
			label = strings.TrimSpace(labels[i-1])
		}
		sb.WriteString(`<span class="zr-star"><span class="zr-glyph" aria-hidden="true">☆</span><span class="zr-sr">` + html.EscapeString(label) + `</span></span>`)
	}
	sb.WriteString(`</div><div class="zr-status">Not rated yet</div></div>`)
	return sb.String(), nil
}

// bodyHTML renders the block's markdown, or shows it as plain text where the
// page has no pipeline.
func bodyHTML(ctx plugins.Context, text string) string {
	if ctx.Markdown != nil {
		if out, err := ctx.Markdown(text); err == nil {
			return out
		}
	}
	return html.EscapeString(text)
}

func (plugin) Assets() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

func (plugin) Entry() plugins.Assets {
	return plugins.Assets{CSS: []string{"rating.css"}, JS: []string{"rating.js"}}
}
