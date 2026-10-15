package file_access

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const testingTmp = "testing_tmp"

func TestMain(m *testing.M) {
	if err := os.MkdirAll(testingTmp, 0755); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(testingTmp)
	os.Exit(code)
}

// makeTestDir creates a fresh directory under testing_tmp containing the
// given files and (empty) dirs, and returns its path.
func makeTestDir(t *testing.T, files, dirs []string) string {
	t.Helper()
	root, err := os.MkdirTemp(testingTmp, "case")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCrawlContent(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		dirs  []string
		want  *ContentDir
	}{
		{
			name: "empty directory",
			want: &ContentDir{Path: "."},
		},
		{
			name:  "files of any type at root",
			files: []string{"a.html", "b.md", "c.css"},
			want:  &ContentDir{Path: ".", Files: []string{"a.html", "b.md", "c.css"}},
		},
		{
			name:  "nested folders",
			files: []string{"about/about.md", "blog/2026/post.md", "blog/index.html"},
			want: &ContentDir{
				Path: ".",
				Dirs: []*ContentDir{
					{Path: "about", Files: []string{"about/about.md"}},
					{
						Path:  "blog",
						Files: []string{"blog/index.html"},
						Dirs:  []*ContentDir{{Path: "blog/2026", Files: []string{"blog/2026/post.md"}}},
					},
				},
			},
		},
		{
			name:  "empty folders omitted",
			files: []string{"page.md"},
			dirs:  []string{"empty", "deep/empty"},
			want:  &ContentDir{Path: ".", Files: []string{"page.md"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := makeTestDir(t, c.files, c.dirs)

			got, err := CrawlContent(os.DirFS(root), ".")
			if err != nil {
				t.Fatalf("CrawlContent: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("CrawlContent(%q) = %#v, want %#v", root, got, c.want)
			}
		})
	}
}

func TestMirrorDirs(t *testing.T) {
	src := makeTestDir(t, []string{"top.md", "about/about.md", "blog/2026/post.md"}, []string{"empty"})
	dst := makeTestDir(t, nil, nil)

	content, err := CrawlContent(os.DirFS(src), ".")
	if err != nil {
		t.Fatalf("CrawlContent: %v", err)
	}
	if err := MirrorDirs(content, content.Path, dst); err != nil {
		t.Fatalf("MirrorDirs: %v", err)
	}

	for _, d := range []string{"about", "blog", "blog/2026"} {
		if info, err := os.Stat(filepath.Join(dst, d)); err != nil || !info.IsDir() {
			t.Errorf("expected dir %s to exist", d)
		}
	}
	for _, p := range []string{"empty", "top.md", "about/about.md", "blog/2026/post.md"} {
		if _, err := os.Stat(filepath.Join(dst, p)); !os.IsNotExist(err) {
			t.Errorf("expected %s not to exist", p)
		}
	}
}

func TestCrawlContentMissingDir(t *testing.T) {
	if _, err := CrawlContent(os.DirFS(testingTmp), "missing"); err == nil {
		t.Error("expected error for missing directory")
	}
}
