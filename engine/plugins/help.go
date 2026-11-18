package plugins

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Limits of a plugin's help text.
const (
	MaxHelpSummary  = 500  // characters
	MaxHelpSnippets = 8    //
	MaxSnippetBytes = 4096 // per snippet's markdown
	maxSnippetTitle = 100  // characters
)

// Snippet is one example of how to use a plugin in a page: a title and the
// markdown to copy (Settings › Plugins shows it, with a Copy button).
type Snippet struct {
	Title    string
	Markdown string
}

// Help is what Settings › Plugins shows next to a plugin: a short summary
// and snippets. It is optional for external plugins.
type Help struct {
	Summary  string
	Snippets []Snippet
}

// CheckHelp validates h against the limits above.
func CheckHelp(h Help) error {
	if utf8.RuneCountInString(h.Summary) > MaxHelpSummary {
		return fmt.Errorf("help summary is longer than %d characters", MaxHelpSummary)
	}
	if len(h.Snippets) > MaxHelpSnippets {
		return fmt.Errorf("help has more than %d snippets", MaxHelpSnippets)
	}
	for i, s := range h.Snippets {
		switch {
		case s.Title == "":
			return fmt.Errorf("help snippet %d needs a title", i+1)
		case utf8.RuneCountInString(s.Title) > maxSnippetTitle:
			return fmt.Errorf("help snippet %d: title is longer than %d characters", i+1, maxSnippetTitle)
		case s.Markdown == "":
			return fmt.Errorf("help snippet %d needs markdown", i+1)
		case len(s.Markdown) > MaxSnippetBytes:
			return fmt.Errorf("help snippet %d is larger than %d KB", i+1, MaxSnippetBytes>>10)
		}
	}
	return nil
}

var errHelpShape = errors.New("help must be a map {summary, snippets: [{title, markdown}]}")

// ParseHelp builds a Help from decoded data (a map from YAML or JSON), for
// loaders that read a `help` key. Keys other than summary, snippets (and
// title, markdown inside a snippet) are rejected.
func ParseHelp(v any) (Help, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return Help{}, errHelpShape
	}
	var h Help
	for k, val := range m {
		switch k {
		case "summary":
			s, ok := val.(string)
			if !ok {
				return Help{}, errors.New("help summary must be a string")
			}
			h.Summary = s
		case "snippets":
			list, ok := val.([]any)
			if !ok {
				return Help{}, errors.New("help snippets must be a list of {title, markdown}")
			}
			for i, e := range list {
				sm, ok := e.(map[string]any)
				if !ok {
					return Help{}, fmt.Errorf("help snippet %d must be a map {title, markdown}", i+1)
				}
				var sn Snippet
				for sk, sv := range sm {
					s, isStr := sv.(string)
					switch {
					case sk != "title" && sk != "markdown":
						return Help{}, fmt.Errorf("help snippet %d: unknown key %q", i+1, sk)
					case !isStr:
						return Help{}, fmt.Errorf("help snippet %d: %s must be a string", i+1, sk)
					case sk == "title":
						sn.Title = s
					default:
						sn.Markdown = s
					}
				}
				h.Snippets = append(h.Snippets, sn)
			}
		default:
			return Help{}, fmt.Errorf("unknown key %q in help (want summary, snippets)", k)
		}
	}
	return h, CheckHelp(h)
}

// Fences returns s with every "~~~" replaced by a code fence ("```"), so that
// built-in help snippets can be written in Go raw strings.
func Fences(s string) string { return strings.ReplaceAll(s, "~~~", "```") }
