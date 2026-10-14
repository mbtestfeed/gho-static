package file_access

import (
	"fmt"
	"os"
	"path/filepath"
)

// ContentDir is a folder within the content directory, holding the files
// directly inside it and its subfolders.
type ContentDir struct {
	Path  string
	Files []string
	Dirs  []*ContentDir
}

// CrawlContent recursively collects all files under path. Folders with no
// files anywhere beneath them are omitted.
func CrawlContent(path string) (*ContentDir, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	dir := &ContentDir{Path: path}
	for _, e := range entries {
		entryPath := filepath.Join(path, e.Name())
		if e.IsDir() {
			sub, err := CrawlContent(entryPath)
			if err != nil {
				return nil, err
			}
			if len(sub.Files) > 0 || len(sub.Dirs) > 0 {
				dir.Dirs = append(dir.Dirs, sub)
			}
		} else {
			dir.Files = append(dir.Files, entryPath)
		}
	}
	return dir, nil
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
