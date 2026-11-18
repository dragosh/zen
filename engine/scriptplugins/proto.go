package scriptplugins

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// The helper protocol: newline-delimited JSON, one message per line, over the
// helper's stdin (zen to helper) and stdout (helper to zen). Every message is
// a msg whose T says which fields count:
//
//	zen -> helper
//	  load      id, hash, source        compile source once, cache it under hash
//	  call      seq, id, hash, hook, globals, limits
//	  reply     cb, value | error       answer to a callback
//	helper -> zen
//	  loaded    hash, error?            answer to load (error: compile error)
//	  result    seq, kind, text, json, type | error
//	  callback  seq, cb, fn, args       a zen module function that needs zen
//
// Callbacks: markdown(text), storage.get(key), storage.set(key, value),
// storage.keys(prefix), read(path), pages(). Permission checks, path rules and
// the renderer stay in zen.
const (
	tLoad     = "load"
	tCall     = "call"
	tReply    = "reply"
	tLoaded   = "loaded"
	tResult   = "result"
	tCallback = "callback"

	// The largest line either side reads: a whole page of HTML in an html
	// hook, or the largest output, escaped.
	maxLine = 64 << 20

	kindNone = "none"
	kindText = "text"
	kindJSON = "json"
	kindMap  = "map" // setup: the plugin map, in json
)

type msg struct {
	T       string            `json:"t"`
	Seq     int64             `json:"seq,omitempty"`
	ID      string            `json:"id,omitempty"`
	Hash    string            `json:"hash,omitempty"`
	Source  string            `json:"source,omitempty"`
	Hook    string            `json:"hook,omitempty"`
	Globals *wireGlobals      `json:"globals,omitempty"`
	Limits  *wireLimits       `json:"limits,omitempty"`
	CB      int64             `json:"cb,omitempty"`
	Fn      string            `json:"fn,omitempty"`
	Args    []json.RawMessage `json:"args,omitempty"`
	Value   json.RawMessage   `json:"value,omitempty"`
	Error   string            `json:"error,omitempty"`
	Kind    string            `json:"kind,omitempty"`
	Text    string            `json:"text,omitempty"`
	JSON    json.RawMessage   `json:"json,omitempty"`
	Type    string            `json:"type,omitempty"` // type name of out (setup)
	Kill    bool              `json:"kill,omitempty"` // result: discard this helper
}

// wireGlobals are the inputs of a call (the script's globals).
type wireGlobals struct {
	Name    string            `json:"name,omitempty"`
	Body    string            `json:"body,omitempty"`
	Attrs   map[string]string `json:"attrs,omitempty"`
	Config  json.RawMessage   `json:"config,omitempty"`
	Path    string            `json:"path,omitempty"`  // page.path
	Title   string            `json:"title,omitempty"` // page.title
	Payload json.RawMessage   `json:"payload,omitempty"`
}

type wireLimits struct {
	TimeoutMS int `json:"timeoutMs"`
	MaxOutput int `json:"maxOutput"`
}

var errLineTooLong = errors.New("line too long")

// readLine reads one line (without the newline), at most max bytes.
func readLine(r *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if len(buf)+len(chunk) > max+1 {
			return nil, errLineTooLong
		}
		buf = append(buf, chunk...)
		if err == nil {
			return bytes.TrimRight(buf, "\r\n"), nil
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF && len(buf) > 0 {
			return bytes.TrimRight(buf, "\r\n"), nil
		}
		return nil, err
	}
}

// encodeLine marshals m as one line (no HTML escaping, so a page's markup
// stays as small as it is).
func encodeLine(m *msg) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	if b.Len() > maxLine {
		return nil, errLineTooLong
	}
	return b.Bytes(), nil // Encode ends the line
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
