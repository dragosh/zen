package scriptplugins

import (
	"bytes"
	"io/fs"
	"time"
)

// memFile is a read-only file held in memory. It can seek, so that
// http.ServeContent serves it with ranges and validators.
type memFile struct {
	*bytes.Reader
	name string
	size int64
}

func newMemFile(name, content string) *memFile {
	return &memFile{Reader: bytes.NewReader([]byte(content)), name: name, size: int64(len(content))}
}

func (m *memFile) Stat() (fs.FileInfo, error) { return m, nil }
func (m *memFile) Close() error               { return nil }
func (m *memFile) Name() string               { return m.name }
func (m *memFile) Size() int64                { return m.size }
func (m *memFile) Mode() fs.FileMode          { return 0o444 }
func (m *memFile) ModTime() time.Time         { return time.Time{} }
func (m *memFile) IsDir() bool                { return false }
func (m *memFile) Sys() any                   { return nil }
