package file_access

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileExists reports whether name exists in fsys.
func FileExists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

// WriteFiles writes each of files (slash separated paths relative to dir)
// into dir, creating folders as needed and refusing to overwrite existing
// files.
func WriteFiles(dir string, files map[string]string) error {
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
		}
		if err := WriteNewFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// NewFolder creates dir along with any missing parents, returning an
// os.IsExist error if dir itself already exists.
func NewFolder(dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	return os.Mkdir(dir, 0755)
}

// WriteNewFile writes content to path, refusing to overwrite an existing file.
func WriteNewFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	_, err = f.WriteString(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
