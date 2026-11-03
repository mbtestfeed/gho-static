package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gopkg.in/yaml.v3"

	"gho-static/gho_static"
	"gho-static/utils/output"
)

// testNow is when test sites, pages and posts are generated.
var testNow = time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)

// testImages returns images as the binary embeds them: a favicon and the
// demo pictures, each holding its own name.
func testImages() fstest.MapFS {
	images := fstest.MapFS{"favicon.ico": {Data: []byte("icon")}}
	for _, pic := range demoPics {
		name := strings.ToLower(pic)
		images["demo_pics/"+name] = &fstest.MapFile{Data: []byte(name)}
	}
	return images
}

// testThemes returns a themes folder as the binary embeds it: the default
// theme, brochureware's preferences and the custom themes readme.
func testThemes() fstest.MapFS {
	dir := gho_static.GhoStaticThemesDir + "/"
	return fstest.MapFS{
		dir + "default_developer_ugly/theme.css":              {Data: []byte("body {}\n")},
		dir + "default_developer_ugly/theme_preferences.yaml": {Data: []byte("summary: Plain.\n")},
		dir + "brochureware/theme_preferences.yaml":           {Data: []byte("summary: Shiny.\n")},
		gho_static.CustomThemesDir + "/README.md":             {Data: []byte("custom readme")},
	}
}

// navLinks returns the nav_links in the settings.yaml in files.
func navLinks(t *testing.T, files map[string]string) []string {
	t.Helper()
	var prefs struct {
		NavLinks []string `yaml:"nav_links"`
	}
	if err := yaml.Unmarshal([]byte(files["settings.yaml"]), &prefs); err != nil {
		t.Fatal(err)
	}
	return prefs.NavLinks
}

// assertFiles fails the test for each of want missing from files, and each
// of notWant in it.
func assertFiles(t *testing.T, files map[string]string, want, notWant []string) {
	t.Helper()
	for _, name := range want {
		if _, ok := files[name]; !ok {
			t.Errorf("%s is missing", name)
		}
	}
	for _, name := range notWant {
		if _, ok := files[name]; ok {
			t.Errorf("%s shouldn't be there", name)
		}
	}
}

func TestContentPage(t *testing.T) {
	cases := []struct {
		name string
		meta pageMeta
		want string
	}{
		{
			name: "plain",
			want: "---\ntitle: My page\nsummary:\npublished: 2026-06-01\ndraft: false\ntags: []\n---\n\n# My page\n",
		},
		{
			name: "every field",
			meta: pageMeta{summary: "About it.", author: "Me", edited: "2026-06-02", theme: "brochureware", draft: true, toc: true, tags: []string{"a", "b c"}},
			want: "---\ntitle: My page\nsummary: About it.\nauthor: Me\npublished: 2026-06-01\nedited: 2026-06-02\ndraft: true\ntags: [a, b c]\ntheme: brochureware\n---\n\n# My page\n\n<!-- toc -->\n",
		},
		{
			name: "clean has no heading",
			meta: pageMeta{clean: true, draft: true},
			want: "---\ntitle: My page\nsummary:\npublished: 2026-06-01\ndraft: true\ntags: []\n---\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := contentPage("my_page", testNow, c.meta); got != c.want {
				t.Errorf("contentPage =\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

func TestSiteFiles(t *testing.T) {
	files, err := siteFiles(gho_static.DefaultPreferences, testImages(), testThemes(), "readme\n", false, testNow)
	if err != nil {
		t.Fatal(err)
	}
	assertFiles(t, files, []string{
		"index.html", "index.js", "structure/header/header.html",
		"static/img/favicon.ico",
		"theme_templates/gho_static_themes/default_developer_ugly/theme.css",
		"theme_templates/custom_themes/README.md",
		"content/about/about.md", "content/about/about.css", "content/about/about.js",
		"content/contact/contact.md",
		"content/blogs/blogs.html", "content/blogs/blogs.md",
		"content/blogs/20260601_example_1/post.md",
		"content/blogs/20260501_example_2/post.md",
		"content/blogs/20260401_example_3/post.md",
		"content/gallery/gallery.html",
		"content/gallery/20260531_august_kittens_7/post.md",
		"content/gallery/20260531_august_kittens_7/august_kittens_7.jpg",
		"content/gallery/20260601_all_kitten_pictures/august_kittens_4.jpg",
		"content/gallery/20260527_gallery_post_without_an_image/post.md",
		"content/themes/themes.html", "content/themes/themes.md",
		"content/themes/20260601_default_developer_ugly/post.md",
		"content/themes/20260531_brochureware/post.md",
		"content/themes/20260531_brochureware/august_kittens_9.jpg",
	}, []string{
		"static/img/demo_pics/august_kittens_7.jpg",
		"theme_templates/gho_static_themes/brochureware/theme_preferences.yaml",
	})

	if want := []string{"about", "blogs", "gallery", "themes", "contact"}; !slices.Equal(navLinks(t, files), want) {
		t.Errorf("nav_links = %q, want %q, with themes after gallery", navLinks(t, files), want)
	}
	if !slices.Equal(gho_static.DefaultPreferences.NavLinks, []string{"about", "blogs", "gallery", "contact"}) {
		t.Errorf("the default nav links were changed: %q", gho_static.DefaultPreferences.NavLinks)
	}
	if got, want := files["index.md"], "---\ntitle:\n---\n\nreadme\n"; got != want {
		t.Errorf("index.md = %q, want %q", got, want)
	}
	if got := files["content/gallery/20260531_august_kittens_7/august_kittens_7.jpg"]; got != "august_kittens_7.jpg" {
		t.Errorf("the gallery post's picture = %q, want the demo picture", got)
	}
	if !strings.HasSuffix(files["content/about/about.md"], aboutText) {
		t.Error("about.md doesn't end with the about page's text")
	}
	example := files["content/blogs/20260601_example_1/post.md"]
	for _, want := range []string{"author: Gho-Static", "edited: 2026-06-01", "tags: [example tag, another example tag]", "<!-- toc -->"} {
		if !strings.Contains(example, want) {
			t.Errorf("the first example post doesn't contain %q:\n%s", want, example)
		}
	}
	showcase := files["content/themes/themes.html"]
	if d, b := strings.Index(showcase, `"default_developer_ugly"`), strings.Index(showcase, `"brochureware"`); d == -1 || b == -1 || d > b {
		t.Errorf("the showcase page doesn't list the default theme then brochureware:\n%s", showcase)
	}
	if !strings.Contains(files["content/themes/20260531_brochureware/post.md"], "theme: brochureware\n") {
		t.Error("brochureware's showcase post isn't styled with it")
	}
}

func TestSiteFilesClean(t *testing.T) {
	prefs := gho_static.DefaultPreferences
	prefs.DefaultAuthor = "Me"
	files, err := siteFiles(prefs, testImages(), testThemes(), "readme\n", true, testNow)
	if err != nil {
		t.Fatal(err)
	}
	assertFiles(t, files, []string{
		"content/about/about.md", "content/contact/contact.md",
		"content/blogs/blogs.html", "content/blogs/20260601_first_post/post.md",
		"content/gallery/gallery.html",
	}, []string{
		"content/blogs/20260601_example_1/post.md",
		"content/gallery/20260601_all_kitten_pictures/post.md",
		"content/themes/themes.html",
	})
	for name := range files {
		if strings.HasPrefix(name, "content/themes/") || strings.HasPrefix(name, "static/img/demo_pics/") {
			t.Errorf("%s shouldn't be in a clean site", name)
		}
	}
	if want := gho_static.DefaultPreferences.NavLinks; !slices.Equal(navLinks(t, files), want) {
		t.Errorf("nav_links = %q, want %q", navLinks(t, files), want)
	}
	if !strings.Contains(files["index.md"], cleanIndexText) {
		t.Errorf("index.md = %q, want the clean site's text", files["index.md"])
	}
	if got, want := files["content/blogs/20260601_first_post/post.md"], contentPage("first_post", testNow, pageMeta{author: "Me", clean: true}); got != want {
		t.Errorf("first post = %q, want %q", got, want)
	}
	if strings.Contains(files["content/about/about.md"], "# ") {
		t.Errorf("a clean about page has a heading:\n%s", files["content/about/about.md"])
	}
}

func TestSiteFilesMissingThemes(t *testing.T) {
	defaultDir := gho_static.GhoStaticThemesDir + "/default_developer_ugly/"
	cases := []struct {
		name    string
		remove  []string
		clean   bool
		wantErr string
	}{
		{name: "custom themes readme", remove: []string{gho_static.CustomThemesDir + "/README.md"}, clean: true, wantErr: "custom themes readme"},
		{name: "default theme", remove: []string{defaultDir + "theme.css", defaultDir + "theme_preferences.yaml"}, clean: true, wantErr: "default theme"},
		{name: "default theme's preferences", remove: []string{defaultDir + "theme_preferences.yaml"}, wantErr: "default_developer_ugly theme's preferences"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			themes := testThemes()
			for _, name := range c.remove {
				delete(themes, name)
			}
			_, err := siteFiles(gho_static.DefaultPreferences, testImages(), themes, "", c.clean, testNow)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %v, want one about the %s", err, c.wantErr)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	logo := filepath.Join(t.TempDir(), "my_logo.png")
	if err := os.WriteFile(logo, []byte("logo"), 0644); err != nil {
		t.Fatal(err)
	}
	prefs := gho_static.DefaultPreferences
	prefs.Logo = logo
	dir := filepath.Join(t.TempDir(), "site")
	if err := Generate(dir, prefs, testImages(), testThemes(), "readme\n", true); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "static", "img", "my_logo.png")); err != nil || string(b) != "logo" {
		t.Errorf("the logo wasn't copied into static/img: %q, %v", b, err)
	}
	settings, err := os.ReadFile(filepath.Join(dir, "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), "logo: static/img/my_logo.png") {
		t.Errorf("settings.yaml doesn't point at the copied logo:\n%s", settings)
	}

	if err := Generate(dir, gho_static.DefaultPreferences, testImages(), testThemes(), "", true); !os.IsExist(err) {
		t.Errorf("generating into an existing folder = %v, want an os.IsExist error", err)
	}

	empty := t.TempDir()
	if err := Generate(empty, gho_static.DefaultPreferences, testImages(), testThemes(), "", true); err != nil {
		t.Fatalf("generating into an empty folder failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(empty, "index.html")); err != nil {
		t.Errorf("the site wasn't generated in the empty folder: %v", err)
	}
}

func TestGenerateBadImages(t *testing.T) {
	cases := []struct {
		name    string
		favicon string
		wantErr string
	}{
		{name: "not an image", favicon: "favicon.txt", wantErr: "must be one of these image types"},
		{name: "missing", favicon: filepath.Join(t.TempDir(), "missing.ico"), wantErr: "can't copy favicon"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prefs := gho_static.DefaultPreferences
			prefs.Favicon = c.favicon
			dir := filepath.Join(t.TempDir(), "site")
			err := Generate(dir, prefs, testImages(), testThemes(), "", true)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %v, want one containing %q", err, c.wantErr)
			}
			if _, err := os.Stat(dir); err == nil {
				t.Error("the site folder was created anyway")
			}
		})
	}
}

// TestGeneratedSitesBuild generates sites from this repo's own images and
// themes, and checks they build.
func TestGeneratedSitesBuild(t *testing.T) {
	for _, clean := range []bool{false, true} {
		for _, opts := range []gho_static.Options{{}, {Publish: true}} {
			dir := filepath.Join(t.TempDir(), "site")
			if err := Generate(dir, gho_static.DefaultPreferences, os.DirFS(filepath.Join("..", "img")), os.DirFS(".."), "# Readme\n", clean); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			opts.Output = output.Output{Out: &out, Err: &out}
			if _, err := gho_static.Build(dir, opts); err != nil {
				t.Errorf("building a generated site (clean %v, publish %v) failed: %v\n%s", clean, opts.Publish, err, out.String())
			}
			if _, err := os.Stat(filepath.Join(dir, "build", "index.html")); err != nil {
				t.Errorf("a generated site (clean %v, publish %v) has no home page: %v", clean, opts.Publish, err)
			}
		}
	}
}

func TestGeneratePage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "content", "my_page")
	if err := GeneratePage(dir, "Me", false, testNow); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(dir, "my_page.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := contentPage("my_page", testNow, pageMeta{author: "Me", draft: true}); string(md) != want {
		t.Errorf("my_page.md = %q, want %q", md, want)
	}
	for _, name := range []string{"my_page.css", "my_page.js"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s wasn't created: %v", name, err)
		}
	}
	if err := GeneratePage(dir, "Me", false, testNow); !os.IsExist(err) {
		t.Errorf("generating an existing page = %v, want an os.IsExist error", err)
	}
}

func TestGeneratePost(t *testing.T) {
	content := t.TempDir()
	post := filepath.Join(content, "news", "20260601_first")
	if err := GeneratePost(post, "first", "", true, testNow); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(post, "post.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := contentPage("first", testNow, pageMeta{draft: true, clean: true}); string(md) != want {
		t.Errorf("post.md = %q, want %q", md, want)
	}
	listing, err := os.ReadFile(filepath.Join(content, "news", "news.html"))
	if err != nil {
		t.Fatalf("the new collection has no listing page: %v", err)
	}
	if string(listing) != listingPageTemplate {
		t.Error("the new collection's listing page isn't the listing template")
	}

	// A second post leaves the collection's listing as it is.
	if err := os.WriteFile(filepath.Join(content, "news", "news.html"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := GeneratePost(filepath.Join(content, "news", "20260602_second"), "second", "", false, testNow); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(content, "news", "news.html")); string(b) != "edited" {
		t.Errorf("the listing page was replaced: %q", b)
	}
}

func TestDatePost(t *testing.T) {
	date := time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name    string
		folder  string
		md      string
		exists  string // another folder already in the collection
		want    string // the post's folder afterwards
		wantMD  string
		wantErr string
	}{
		{name: "undated folder", folder: "first", md: "---\ntitle: First\n---\nBody\n", want: "20260102_first", wantMD: "---\ntitle: First\npublished: 2026-01-02\n---\nBody\n"},
		{name: "replaces the folder's date", folder: "20251231_first", md: "---\ntitle: First\n---\n", want: "20260102_first", wantMD: "---\ntitle: First\npublished: 2026-01-02\n---\n"},
		{name: "already that date", folder: "20260102_first", md: "---\ntitle: First\n---\n", want: "20260102_first", wantMD: "---\ntitle: First\npublished: 2026-01-02\n---\n"},
		{name: "only a date", folder: "20251231", md: "Body\n", want: "20260102_untitled", wantMD: "---\npublished: 2026-01-02\n---\n\nBody\n"},
		{name: "taken folder", folder: "first", md: "Body\n", exists: "20260102_first", wantErr: "already exists"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			collection := t.TempDir()
			mdPath := filepath.Join(collection, c.folder, "post.md")
			if err := os.MkdirAll(filepath.Dir(mdPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(mdPath, []byte(c.md), 0644); err != nil {
				t.Fatal(err)
			}
			if c.exists != "" {
				if err := os.Mkdir(filepath.Join(collection, c.exists), 0755); err != nil {
					t.Fatal(err)
				}
			}
			err := DatePost(mdPath, date)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, c.wantErr)
				}
				if b, _ := os.ReadFile(mdPath); string(b) != c.md {
					t.Errorf("post.md was changed despite the error: %q", b)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			md, err := os.ReadFile(filepath.Join(collection, c.want, "post.md"))
			if err != nil {
				t.Fatalf("the post isn't in %s: %v", c.want, err)
			}
			if string(md) != c.wantMD {
				t.Errorf("post.md = %q, want %q", md, c.wantMD)
			}
		})
	}
}

func TestSetPublished(t *testing.T) {
	cases := []struct{ name, md, want string }{
		{"adds to front matter", "---\ntitle: A\n---\nBody\n", "---\ntitle: A\npublished: 2026-06-01\n---\nBody\n"},
		{"overrides an invalid date", "---\npublished: soon\n---\n", "---\npublished: soon\npublished: 2026-06-01\n---\n"},
		{"adds front matter", "Body\r\nMore\r\n", "---\npublished: 2026-06-01\n---\n\nBody\nMore\n"},
	}
	for _, c := range cases {
		if got := setPublished(c.md, testNow); got != c.want {
			t.Errorf("%s: setPublished = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPublishDraft(t *testing.T) {
	cases := []struct{ name, md, want string }{
		{"removes the draft line", "---\ntitle: A\ndraft: true\ntags: []\n---\nBody\n", "---\ntitle: A\ntags: []\n---\nBody\n"},
		{"without a draft line", "---\ntitle: A\n---\nBody\n", "---\ntitle: A\n---\nBody\n"},
		{"without front matter", "draft: true\n", "draft: true\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "page.md")
			if err := os.WriteFile(path, []byte(c.md), 0644); err != nil {
				t.Fatal(err)
			}
			if err := PublishDraft(path); err != nil {
				t.Fatal(err)
			}
			if b, _ := os.ReadFile(path); string(b) != c.want {
				t.Errorf("page.md = %q, want %q", b, c.want)
			}
		})
	}
	if err := PublishDraft(filepath.Join(t.TempDir(), "missing.md")); err == nil {
		t.Error("publishing a missing page didn't fail")
	}
}
