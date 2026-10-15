package file_access

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// ContentDir is a folder within the content directory, holding the files
// directly inside it and its subfolders. Paths are slash separated, within
// the fs.FS it was crawled from.
type ContentDir struct {
	Path  string
	Files []string
	Dirs  []*ContentDir
}

// CrawlContent recursively collects all files under dir in fsys. Folders with
// no files anywhere beneath them are omitted.
func CrawlContent(fsys fs.FS, dir string) (*ContentDir, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", dir, err)
	}

	content := &ContentDir{Path: dir}
	for _, e := range entries {
		entryPath := path.Join(dir, e.Name())
		if e.IsDir() {
			sub, err := CrawlContent(fsys, entryPath)
			if err != nil {
				return nil, err
			}
			if len(sub.Files) > 0 || len(sub.Dirs) > 0 {
				content.Dirs = append(content.Dirs, sub)
			}
		} else {
			content.Files = append(content.Files, entryPath)
		}
	}
	return content, nil
}

// MirrorDirs recreates dir's subfolder structure, relative to root, under dst.
func MirrorDirs(dir *ContentDir, root, dst string) error {
	for _, sub := range dir.Dirs {
		rel, err := filepath.Rel(root, sub.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dst, rel), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", rel, err)
		}
		if err := MirrorDirs(sub, root, dst); err != nil {
			return err
		}
	}
	return nil
}
