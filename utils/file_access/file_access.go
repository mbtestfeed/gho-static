package file_access

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FileExists reports whether name exists in fsys.
func FileExists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

// TemplateSuffix ends the name of each HTML template gho-static generates,
// so they aren't mistaken for built pages.
const TemplateSuffix = ".template.html"

// TemplateFS presents fsys's name.template.html files as name.html, so a
// site's templates are read by the names they're built to. Sites made before
// the suffix was added keep working, with a name.template.html file taking
// precedence over a name.html one.
type TemplateFS struct{ fs.FS }

func (t TemplateFS) Open(name string) (fs.File, error) {
	if stem, ok := strings.CutSuffix(name, ".html"); ok {
		if f, err := t.FS.Open(stem + TemplateSuffix); err == nil {
			return f, nil
		}
	}
	return t.FS.Open(name)
}

func (t TemplateFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(t.FS, name)
	var out []fs.DirEntry
	for _, e := range entries {
		if stem, ok := strings.CutSuffix(e.Name(), TemplateSuffix); ok && !e.IsDir() {
			e = templateEntry{e, stem + ".html"}
		} else if stem, ok := strings.CutSuffix(e.Name(), ".html"); ok && FileExists(t.FS, path.Join(name, stem+TemplateSuffix)) {
			continue
		}
		out = append(out, e)
	}
	return out, err
}

// templateEntry is a name.template.html entry renamed to name.html.
type templateEntry struct {
	fs.DirEntry
	name string
}

func (e templateEntry) Name() string { return e.name }

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

// IsEmptyFolder reports whether dir is an existing folder holding nothing
// but hidden files and folders (those whose names start with a dot), such
// as a new git repo's .git.
func IsEmptyFolder(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			return false
		}
	}
	return true
}

// NewOrEmptyFolder creates dir as NewFolder does, or uses dir as it is if
// it's an empty folder (see IsEmptyFolder), returning an os.IsExist error if
// dir already exists otherwise.
func NewOrEmptyFolder(dir string) error {
	if IsEmptyFolder(dir) {
		return nil
	}
	return NewFolder(dir)
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
