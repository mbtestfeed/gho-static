package utils

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHasExt(t *testing.T) {
	cases := []struct {
		name string
		file string
		exts []string
		want bool
	}{
		{name: "matching extension", file: "about.md", exts: []string{".md"}, want: true},
		{name: "ignores case", file: "ABOUT.MD", exts: []string{".md"}, want: true},
		{name: "any of several", file: "blogs.html", exts: []string{".md", ".html"}, want: true},
		{name: "different extension", file: "notes.txt", exts: []string{".md"}, want: false},
		{name: "no extension", file: "README", exts: []string{".md"}, want: false},
		{name: "only the last extension counts", file: "archive.md.bak", exts: []string{".md"}, want: false},
		{name: "no extensions given", file: "about.md", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := HasExt(c.file, c.exts...); got != c.want {
				t.Errorf("HasExt(%q, %q) = %v, want %v", c.file, c.exts, got, c.want)
			}
		})
	}
}

func TestFileLink(t *testing.T) {
	t.Run("absolute path, with spaces escaped", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "my site", "index.html")
		want := "file:///" + strings.ReplaceAll(strings.TrimPrefix(filepath.ToSlash(path), "/"), " ", "%20")
		if got := FileLink(path); got != want {
			t.Errorf("FileLink(%q) = %q, want %q", path, got, want)
		}
	})

	t.Run("relative path is made absolute", func(t *testing.T) {
		abs, err := filepath.Abs(filepath.Join("build", "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := FileLink(filepath.Join("build", "index.html")), FileLink(abs); got != want {
			t.Errorf("FileLink of relative path = %q, want %q", got, want)
		}
	})
}
