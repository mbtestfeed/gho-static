package utils

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

// HasExt reports whether file has one of exts as its extension, ignoring
// case.
func HasExt(file string, exts ...string) bool {
	ext := filepath.Ext(file)
	return slices.ContainsFunc(exts, func(e string) bool { return strings.EqualFold(e, ext) })
}

// FileLink returns a file:// URL for path.
func FileLink(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// Windows paths (C:\...) need a leading slash to form file:///C:/...
	link := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}
	return link.String()
}
