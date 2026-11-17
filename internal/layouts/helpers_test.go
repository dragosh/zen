package layouts

import (
	"os"
	"path/filepath"
)

func writeConfig(dir, content string) error {
	return os.WriteFile(filepath.Join(dir, ".zen.yml"), []byte(content), 0o644)
}
