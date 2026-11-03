package gho_static

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gho-static/utils/output"
)

// testNow is the time test sites are built at, after their posts are
// published.
var testNow = time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)

// testSite writes a small site into a new temporary folder and returns its
// path: a home page, an about page, a draft page and a blog with one post,
// linking to each other in the ways links rewrites.
func testSite(t *testing.T) string {
	t.Helper()
	site := t.TempDir()
	files := map[string]string{
		"settings.yaml":                "nav_links: [about, blogs]\nshow_intro_sidebar: false\ncollections_displayed_in_recent_sidebar: []\n",
		"index.html":                   "<html>{{.Header}}{{.Content}}{{.Footer}}</html>\n",
		"index.md":                     "---\ntitle: Home\n---\n\n[About](./about) [About again](./content/about/about.md) [About from above](../content/about)\n",
		"structure/header/header.html": "<header></header>\n",
		"structure/footer/footer.html": "<footer></footer>\n",
		"content/about/about.md": "---\ntitle: About\n---\n\n" +
			"[Blog](/blogs) [Post](../blogs/20260101_first/post.md#top) [Draft](/drafty) [Missing](../nope) [Outside page](../../../outside.md) [Spaced](../our%20team)\n\n" +
			"![pic](./pic.png)\n",
		"content/about/pic.png":                "png",
		"content/our team/our team.md":         "---\ntitle: Our team\n---\n\nUs.\n",
		"content/drafty/drafty.md":             "---\ntitle: Drafty\ndraft: true\n---\n\nDraft.\n",
		"content/blogs/blogs.md":               "---\ntitle: Blogs\n---\n\nThe blog.\n",
		"content/blogs/blogs.html":             "{{.Intro}}{{range .SubPages}}<a href=\"{{$.Root}}{{.Link}}\">{{.Title}}</a>{{end}}\n",
		"content/blogs/20260101_first/post.md": "---\ntitle: First\npublished: 2026-01-01\n---\n\nFirst post.\n",
	}
	for name, data := range files {
		writeTestFile(t, filepath.Join(site, filepath.FromSlash(name)), data)
	}
	// The theme's files are all optional, but its folder must exist.
	if err := os.MkdirAll(filepath.Join(site, filepath.FromSlash(GhoStaticThemesDir), DefaultPreferences.Theme), 0755); err != nil {
		t.Fatal(err)
	}
	return site
}

// writeTestFile writes data to path, creating its folder.
func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

// buildTestSite builds site with opts at testNow, failing the test if it
// fails, and returns the warnings and errors written.
func buildTestSite(t *testing.T, site string, opts Options) string {
	t.Helper()
	var out, errs bytes.Buffer
	opts.Output, opts.Now = output.Output{Out: &out, Err: &errs}, testNow
	if _, err := Build(site, opts); err != nil {
		t.Fatal(err)
	}
	return errs.String()
}

// readBuilt returns the contents of name (slash separated) in site's build
// folder, failing the test if it can't be read.
func readBuilt(t *testing.T, site, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(site, "build", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// assertContains fails the test for each of wants not in got, which is
// described by name.
func assertContains(t *testing.T, name, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("%s doesn't contain %q:\n%s", name, want, got)
		}
	}
}

// assertNotContains fails the test for each of unwanted in got, which is
// described by name.
func assertNotContains(t *testing.T, name, got string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(got, u) {
			t.Errorf("%s contains %q:\n%s", name, u, got)
		}
	}
}

// assertNoTempBuilds fails the test if a temporary build folder was left in
// site.
func assertNoTempBuilds(t *testing.T, site string) {
	t.Helper()
	entries, err := os.ReadDir(site)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".build-") {
			t.Errorf("temporary build folder %s was left in the site folder", e.Name())
		}
	}
}

func TestBuildWritesFiles(t *testing.T) {
	cases := []struct {
		name    string
		opts    Options
		want    []string
		notWant []string
	}{
		{
			name: "local build writes each page as its own html file",
			want: []string{
				"index.html", "404.html", ".nojekyll",
				"about/about.html", "drafty/drafty.html",
				"blogs/blogs.html", "blogs/20260101_first/post.html",
				"static/about/pic.png",
			},
		},
		{
			name: "preview leaves out drafts, keeping local page names",
			opts: Options{Preview: true},
			want: []string{
				"index.html", "about/about.html", "blogs/blogs.html",
				"blogs/20260101_first/post.html", "static/about/pic.png",
			},
			notWant: []string{"drafty"},
		},
		{
			name: "publish writes folder/index.html and leaves out drafts",
			opts: Options{Publish: true},
			want: []string{
				"index.html", "404.html", "about/index.html",
				"blogs/index.html", "blogs/20260101_first/index.html",
				"static/about/pic.png",
			},
			notWant: []string{"drafty", "about/about.html"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site := testSite(t)
			buildTestSite(t, site, c.opts)
			for _, name := range c.want {
				if _, err := os.Stat(filepath.Join(site, "build", filepath.FromSlash(name))); err != nil {
					t.Errorf("%s wasn't built: %v", name, err)
				}
			}
			for _, name := range c.notWant {
				if _, err := os.Stat(filepath.Join(site, "build", filepath.FromSlash(name))); err == nil {
					t.Errorf("%s was built, but shouldn't have been", name)
				}
			}
			assertNoTempBuilds(t, site)
		})
	}
}

func TestBuildRewritesLinks(t *testing.T) {
	cases := []struct {
		name      string
		opts      Options
		home      string // the home page's built file
		about     string // the about page's built file
		homeLinks []string
		links     []string // on the about page
		warnings  []string
		notWarned []string
	}{
		{
			name:      "local build links relative to each page",
			home:      "index.html",
			about:     "about/about.html",
			homeLinks: []string{`href="./about/about.html">About<`, `href="./about/about.html">About again<`, `href="./about/about.html">About from above<`},
			links: []string{
				`href="../blogs/blogs.html"`,
				`href="../blogs/20260101_first/post.html#top"`,
				`href="../drafty/drafty.html"`,
				`href="../nope"`,
				`href="../our%20team/our%20team.html"`,
				`src="../static/about/pic.png"`,
			},
			warnings:  []string{"links to ../nope, which isn't in the build"},
			notWarned: []string{"links to /drafty"},
		},
		{
			name:      "preview links like a local build",
			opts:      Options{Preview: true},
			home:      "index.html",
			about:     "about/about.html",
			homeLinks: []string{`href="./about/about.html"`},
			links: []string{
				`href="../blogs/blogs.html"`,
				`href="../blogs/20260101_first/post.html#top"`,
				`href="../drafty/drafty.html"`,
				`href="../our%20team/our%20team.html"`,
			},
			warnings: []string{"links to /drafty, which isn't in the build"},
		},
		{
			name:      "publish links from the site's root, by folder",
			opts:      Options{Publish: true},
			home:      "index.html",
			about:     "about/index.html",
			homeLinks: []string{`href="/about">About<`, `href="/about">About again<`, `href="/about">About from above<`},
			links: []string{
				`href="/blogs"`,
				`href="/blogs/20260101_first#top"`,
				`href="/drafty"`,
				`href="/our%20team"`,
				`src="../static/about/pic.png"`,
			},
			warnings: []string{"links to /drafty, which isn't in the build"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site := testSite(t)
			warnings := buildTestSite(t, site, c.opts)
			assertContains(t, c.home, readBuilt(t, site, c.home), c.homeLinks...)
			about := readBuilt(t, site, c.about)
			assertContains(t, c.about, about, c.links...)

			// Links outside the site folder are removed, keeping their text.
			assertContains(t, c.about, about, "Outside page")
			assertNotContains(t, c.about, about, "outside.md")
			warnings = strings.ReplaceAll(warnings, output.Warning, "")
			assertContains(t, "warnings", warnings, append(c.warnings, "links to ../../../outside.md, which is outside the site folder, so it was removed")...)
			assertNotContains(t, "warnings", warnings, c.notWarned...)
		})
	}
}

func TestBuildNavLinks(t *testing.T) {
	site := testSite(t)
	writeTestFile(t, filepath.Join(site, "settings.yaml"), "nav_links: [about, élan_vital]\nshow_intro_sidebar: false\ncollections_displayed_in_recent_sidebar: []\n")
	writeTestFile(t, filepath.Join(site, "structure", "header", "header.html"), "<header>{{range .Site.NavLinks}}<a>{{.Name}}</a>{{end}}</header>\n")
	writeTestFile(t, filepath.Join(site, "content", "élan_vital", "élan_vital.md"), "---\ntitle: Élan vital\n---\n")
	buildTestSite(t, site, Options{})
	assertContains(t, "index.html", readBuilt(t, site, "index.html"), "<a>About</a><a>Élan vital</a>")
}

func TestBuildReplacesPreviousBuild(t *testing.T) {
	site := testSite(t)
	writeTestFile(t, filepath.Join(site, "build", ".git", "HEAD"), "ref: refs/heads/main\n")
	writeTestFile(t, filepath.Join(site, "build", "stale.html"), "stale")
	buildTestSite(t, site, Options{})

	if _, err := os.Stat(filepath.Join(site, "build", ".git", "HEAD")); err != nil {
		t.Errorf("the build's git repo wasn't kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(site, "build", "stale.html")); err == nil {
		t.Errorf("stale.html, from the previous build, wasn't removed")
	}
	readBuilt(t, site, "about/about.html")
	assertNoTempBuilds(t, site)
}

func TestBuildFailureKeepsPreviousBuild(t *testing.T) {
	site := testSite(t)
	buildTestSite(t, site, Options{})
	before := readBuilt(t, site, "index.html")

	// Parses, but fails when the first page is written into the build.
	writeTestFile(t, filepath.Join(site, "structure", "header", "header.html"), "{{.Nope}}\n")
	var errs bytes.Buffer
	if _, err := Build(site, Options{Output: output.Output{Out: &bytes.Buffer{}, Err: &errs}, Now: testNow}); err == nil {
		t.Fatal("the build didn't fail")
	}

	if after := readBuilt(t, site, "index.html"); after != before {
		t.Errorf("index.html changed after a failed build:\n%s", after)
	}
	readBuilt(t, site, "about/about.html")
	assertNoTempBuilds(t, site)
}

func TestBuildMissingSite(t *testing.T) {
	if _, err := Build(filepath.Join(t.TempDir(), "missing"), Options{}); err == nil {
		t.Error("building a missing site folder didn't fail")
	}
}
