package file_access

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// readFile returns the contents of path, failing the test if it can't be read.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFileExists(t *testing.T) {
	fsys := os.DirFS(makeTestDir(t, []string{"a/b.txt"}, nil))
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "file", path: "a/b.txt", want: true},
		{name: "folder", path: "a", want: true},
		{name: "missing", path: "a/missing.txt", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FileExists(fsys, c.path); got != c.want {
				t.Errorf("FileExists(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestWriteFiles(t *testing.T) {
	t.Run("writes files, creating folders", func(t *testing.T) {
		root := makeTestDir(t, nil, nil)
		files := map[string]string{"a.txt": "A", "sub/dir/b.txt": "B"}
		if err := WriteFiles(root, files); err != nil {
			t.Fatal(err)
		}
		for name, want := range files {
			if got := readFile(t, filepath.Join(root, filepath.FromSlash(name))); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	})

	t.Run("refuses to overwrite", func(t *testing.T) {
		root := makeTestDir(t, nil, nil)
		path := filepath.Join(root, "a.txt")
		if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := WriteFiles(root, map[string]string{"a.txt": "new"}); !errors.Is(err, fs.ErrExist) {
			t.Errorf("WriteFiles over an existing file returned %v, want an fs.ErrExist error", err)
		}
		if got := readFile(t, path); got != "original" {
			t.Errorf("a.txt = %q after refused write, want \"original\"", got)
		}
	})
}

func TestNewFolder(t *testing.T) {
	dir := filepath.Join(makeTestDir(t, nil, nil), "parent", "child")
	if err := NewFolder(dir); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("NewFolder(%q) didn't create the folder: %v", dir, err)
	}
	if err := NewFolder(dir); !os.IsExist(err) {
		t.Errorf("NewFolder on an existing folder returned %v, want an os.IsExist error", err)
	}
}

func TestWriteNewFile(t *testing.T) {
	path := filepath.Join(makeTestDir(t, nil, nil), "a.txt")
	if err := WriteNewFile(path, "first"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "first" {
		t.Errorf("a.txt = %q, want \"first\"", got)
	}
	if err := WriteNewFile(path, "second"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("WriteNewFile over an existing file returned %v, want an fs.ErrExist error", err)
	}
	if got := readFile(t, path); got != "first" {
		t.Errorf("a.txt = %q after refused write, want \"first\"", got)
	}
}
