package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// The one editor of the export: mapping of a folder's .zenmd.yml. The app's Save as folder defaults (the website
// and the PDF forms) and `zenmd export --save-defaults` both call it, so the same choices give the same file and
// the same preview text.

// ExportKey is a key of the export: mapping to set, with its YAML tag. Key is a dotted path below export:, such as
// "out", "pdf.cover" or "theme.scheme"; the mappings on the way are made when they are missing.
type ExportKey struct{ Key, Val, Tag string }

// StrKey is an ExportKey with a text value.
func StrKey(key, val string) ExportKey { return ExportKey{key, val, "!!str"} }

// BoolKey is an ExportKey with a true or false value.
func BoolKey(key string, val bool) ExportKey {
	if val {
		return ExportKey{key, "true", "!!bool"}
	}
	return ExportKey{key, "false", "!!bool"}
}

// exportChange is one key that was changed or added.
type exportChange struct {
	key, was, now string
	existed       bool
}

// SaveExportKeys sets the keys in the export: mapping of the folder's .zenmd.yml (made when there is none) and
// returns the preview text of what changed, or nothing when there is nothing to change. Without confirm it only
// returns the text; with it, it also writes the file atomically. It edits the YAML tree, so comments and other keys
// stay, and a first write moves the old settings file to the new name. A key that already has the value
// is left alone, and a mapping that would stay empty is not left behind.
func SaveExportKeys(root string, keys []ExportKey, nothing string, confirm bool) (string, error) {
	path, legacy, _ := WorkspaceConfigPath(root)
	var doc yaml.Node
	data, rerr := os.ReadFile(path)
	if rerr != nil && !os.IsNotExist(rerr) {
		return "", rerr
	}
	if rerr == nil {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf(".zenmd.yml is not valid YAML: %v", err)
		}
	}
	fresh := doc.Kind == 0
	if fresh {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	top := doc.Content[0]
	if doc.Kind != yaml.DocumentNode || top.Kind != yaml.MappingNode {
		return "", errors.New(".zenmd.yml is not a YAML mapping; fix it first")
	}
	if fresh {
		setKey(top, "version", scalar("1", "!!int"))
	}
	exp, renamed, err := exportSection(top)
	if err != nil {
		return "", err
	}
	changes, err := setExportKeys(exp, "export.", keys)
	if err != nil {
		return "", err
	}
	if len(changes) == 0 && !renamed {
		return nothing, nil
	}
	added := true
	for _, c := range changes {
		if c.existed {
			added = false
		}
	}
	var sb strings.Builder
	if added && !renamed {
		sb.WriteString("Adds to .zenmd.yml:")
	} else {
		sb.WriteString("Changes .zenmd.yml:")
	}
	if renamed {
		sb.WriteString("\n  build: becomes export: (its values move with it)")
	}
	for _, c := range changes {
		sb.WriteString("\n  " + c.key + ": " + c.now)
		if c.was != "" {
			sb.WriteString("  (was " + c.was + ")")
		}
	}
	preview := sb.String()
	if !confirm {
		return preview, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if legacy { // the first write moves the old file to the new name
		if err := MoveLegacy(root); err != nil {
			return "", err
		}
		path = filepath.Join(root, WorkspaceFile)
	}
	if err := WriteAtomic(path, buf.Bytes(), perm); err != nil {
		return "", err
	}
	return preview, nil
}

// madeMapping is a mapping setExportKeys made: its parent, its key and itself.
type madeMapping struct {
	in   *yaml.Node
	key  string
	node *yaml.Node
}

// setExportKeys sets the keys below the mapping m and returns what changed; a key that already has the value (an
// out path compared cleaned) is left alone. Mappings made on the way to a key that changed nothing are removed.
func setExportKeys(m *yaml.Node, prefix string, want []ExportKey) ([]exportChange, error) {
	var changes []exportChange
	for _, w := range want {
		path := strings.Split(w.Key, ".")
		parent := m
		var made []madeMapping
		for _, seg := range path[:len(path)-1] {
			next := mapValue(parent, seg)
			switch {
			case next == nil || next.Kind == yaml.ScalarNode && next.Tag == "!!null":
				next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				setKey(parent, seg, next)
				made = append(made, madeMapping{parent, seg, next})
			case next.Kind != yaml.MappingNode:
				return nil, fmt.Errorf("export.%s in .zenmd.yml must be a mapping", seg)
			}
			parent = next
		}
		last := path[len(path)-1]
		cur := mapValue(parent, last)
		switch {
		case cur != nil && (cur.Value == w.Val || last == "out" && filepath.ToSlash(filepath.Clean(cur.Value)) == w.Val):
		case cur != nil:
			changes = append(changes, exportChange{prefix + w.Key, cur.Value, w.Val, true})
			cur.Value, cur.Tag, cur.Style = w.Val, w.Tag, 0
		default:
			changes = append(changes, exportChange{prefix + w.Key, "", w.Val, false})
			setKey(parent, last, scalar(w.Val, w.Tag))
		}
		for i := len(made) - 1; i >= 0; i-- { // nothing was needed: do not leave an empty mapping
			if len(made[i].node.Content) == 0 {
				deleteKey(made[i].in, made[i].key)
			}
		}
	}
	return changes, nil
}

// SiteOutValue is the website's output as export.out stores it: relative to the folder when inside it, else absolute.
func SiteOutValue(root, out string) string {
	if out == "" {
		out = DefaultExportOut
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	abs := filepath.Clean(out)
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return abs
	}
	rel, _ := filepath.Rel(root, abs)
	return filepath.ToSlash(rel)
}

// PDFOutValue is the PDF's output as export.pdf.out stores it: relative to the folder when inside it, else absolute.
// It is an error when out is the folder itself or does not name a file.
func PDFOutValue(root, out string) (string, error) {
	abs, err := ResolvePDFOut(root, out)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		abs = rel
	}
	return filepath.ToSlash(abs), nil
}

// WriteAtomic writes data next to path and renames it over path.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(perm)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}

// exportSection returns the export: mapping of the top-level mapping, creating
// it when there is none. An old build: mapping moves into it (renamed in place
// when there is no export:, else its keys are added to the ones export: lacks)
// and the build: key goes; renamed says it did. A build: or export: that is
// not a mapping is an error and nothing has changed.
func exportSection(top *yaml.Node) (exp *yaml.Node, renamed bool, err error) {
	isNull := func(n *yaml.Node) bool { return n == nil || n.Kind == yaml.ScalarNode && n.Tag == "!!null" }
	old, cur := mapValue(top, "build"), mapValue(top, "export")
	if !isNull(old) && old.Kind != yaml.MappingNode {
		return nil, false, errors.New("build in .zenmd.yml must be a mapping with keys such as out and portable")
	}
	if !isNull(cur) && cur.Kind != yaml.MappingNode {
		return nil, false, errors.New("export in .zenmd.yml must be a mapping with keys such as out and portable")
	}
	switch {
	case old == nil: // no old key: the plain case
		if isNull(cur) {
			cur = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			setKey(top, "export", cur)
		}
		return cur, false, nil
	case isNull(cur):
		if old.Kind != yaml.MappingNode {
			old = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		if cur == nil && mapKey(top, "export") == nil { // rename in place: position and comments stay
			if k := mapKey(top, "build"); k != nil {
				k.Value = "export"
			}
			if mapValue(top, "export") != old {
				setKey(top, "export", old)
			}
			return old, true, nil
		}
		setKey(top, "export", old)
		deleteKey(top, "build")
		return old, true, nil
	}
	for i := 0; i+1 < len(old.Content); i += 2 {
		if mapValue(cur, old.Content[i].Value) == nil {
			cur.Content = append(cur.Content, old.Content[i], old.Content[i+1])
		}
	}
	deleteKey(top, "build")
	return cur, true, nil
}

// mapKey is the key node of key in a mapping node, or nil.
func mapKey(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i]
		}
	}
	return nil
}

// deleteKey removes key from a mapping node.
func deleteKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func scalar(v, tag string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v} }

// mapValue is the value node of key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setKey sets key to v in a mapping node, replacing an existing value or appending.
func setKey(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
}
