package gho_static

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPagePath(t *testing.T) {
	s := source{FS: fstest.MapFS{
		"content/about/about.md":               {},
		"content/about/pic.png":                {},
		"content/blogs/blogs.md":               {},
		"content/blogs/blogs.html":             {},
		"content/blogs/20260101_first/post.md": {},
		"content/multi/a.md":                   {},
		"content/multi/b.md":                   {},
	}}
	cases := []struct {
		name     string
		src      string
		publish  bool
		wantFile string
		wantLink string
	}{
		{name: "local page", src: "content/about/about.md", wantFile: "about/about.html", wantLink: "about/about.html"},
		{name: "local post", src: "content/blogs/20260101_first/post.md", wantFile: "blogs/20260101_first/post.html", wantLink: "blogs/20260101_first/post.html"},
		{name: "published folder's only page takes the folder", src: "content/about/about.md", publish: true, wantFile: "about/index.html", wantLink: "about/"},
		{name: "published template page and intro are one page", src: "content/blogs/blogs.html", publish: true, wantFile: "blogs/index.html", wantLink: "blogs/"},
		{name: "published post", src: "content/blogs/20260101_first/post.md", publish: true, wantFile: "blogs/20260101_first/index.html", wantLink: "blogs/20260101_first/"},
		{name: "published folder with several pages gives each a subfolder", src: "content/multi/a.md", publish: true, wantFile: "multi/a/index.html", wantLink: "multi/a/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file, link, err := s.pagePath(c.src, "content", c.publish)
			if err != nil {
				t.Fatal(err)
			}
			if file != c.wantFile || link != c.wantLink {
				t.Errorf("pagePath(%q) = %q, %q, want %q, %q", c.src, file, link, c.wantFile, c.wantLink)
			}
		})
	}
}

func TestRootFor(t *testing.T) {
	cases := []struct{ file, want string }{
		{"index.html", "./"},
		{"about/about.html", "../"},
		{"blogs/20260101_first/index.html", "../../"},
	}
	for _, c := range cases {
		if got := rootFor(c.file); got != c.want {
			t.Errorf("rootFor(%q) = %q, want %q", c.file, got, c.want)
		}
	}
}

func TestSplitLink(t *testing.T) {
	cases := []struct {
		dest       string
		wantPath   string
		wantSuffix string
		wantOK     bool
	}{
		{dest: "../about", wantPath: "../about", wantOK: true},
		{dest: "../about#top", wantPath: "../about", wantSuffix: "#top", wantOK: true},
		{dest: "a%20b.md?x=1#top", wantPath: "a b.md", wantSuffix: "?x=1#top", wantOK: true},
		{dest: "/about", wantPath: "/about", wantOK: true},
		{dest: "#top"},
		{dest: ""},
		{dest: "https://example.com/about"},
		{dest: "mailto:me@example.com"},
	}
	for _, c := range cases {
		p, suffix, ok := splitLink(c.dest)
		if p != c.wantPath || suffix != c.wantSuffix || ok != c.wantOK {
			t.Errorf("splitLink(%q) = %q, %q, %v, want %q, %q, %v", c.dest, p, suffix, ok, c.wantPath, c.wantSuffix, c.wantOK)
		}
	}
}

func TestEscapes(t *testing.T) {
	cases := []struct {
		dir, p string
		want   bool
	}{
		{"content/about", "../blogs", false},
		{"content/about", "../../index.md", false},
		{"content/about", "../../../outside.md", true},
		{".", "./about", false},
		{".", "..", true},
		{".", "../content/about", true},
		{"content/about", "/blogs", false},
		{"content/about", "/../index.md", false},
		{"content/about", "/../../outside.md", true},
	}
	for _, c := range cases {
		if got := escapes("content", c.dir, c.p); got != c.want {
			t.Errorf("escapes(%q, %q) = %v, want %v", c.dir, c.p, got, c.want)
		}
	}
}

func TestLocalLinks(t *testing.T) {
	md := "[a](./a.md) [b](/b#top) [ext](https://example.com) [top](#top) ![pic](./pic.png) ![ext](https://example.com/x.png) note[^1]\n\n[^1]: A note.\n"
	links, images := localLinks([]byte(md))
	if want := []string{"./a.md", "/b#top"}; !slices.Equal(links, want) {
		t.Errorf("links = %q, want %q", links, want)
	}
	if want := []string{"./pic.png"}; !slices.Equal(images, want) {
		t.Errorf("images = %q, want %q", images, want)
	}
}

func TestMdToHTMLLinks(t *testing.T) {
	link := func(dest string) string {
		switch dest {
		case "out", "out.png":
			return ""
		case "page":
			return "../page/page.html"
		}
		return dest
	}
	html := string(mdToHTML([]byte("[Rewritten](page) [Kept text](out) [Other](other) ![alt text](out.png)\n"), "./", "", link))
	for _, want := range []string{`href="../page/page.html"`, `href="other"`, "Kept text"} {
		if !strings.Contains(html, want) {
			t.Errorf("html doesn't contain %q:\n%s", want, html)
		}
	}
	for _, unwanted := range []string{`href="out"`, `href=""`, "<img", "alt text"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("html contains %q:\n%s", unwanted, html)
		}
	}
}
