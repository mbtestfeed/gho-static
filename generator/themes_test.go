package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"gho-static/gho_static"
	"gho-static/utils/output"
)

// writeFiles writes each of files (slash separated paths) with its contents
// into dir, creating folders as needed.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// readFile returns the contents of name (slash separated) in dir, failing
// the test if it can't be read.
func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWithoutWarning(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"css", "/* GHO-STATIC THEME FILE: don't edit it.\n   Really. */\n\nbody {}\n", "body {}\n"},
		{"yaml", "# GHO-STATIC THEME FILE: don't edit it.\n# Really.\n\nsummary: Plain.\n", "summary: Plain.\n"},
		{"windows line endings", "/* GHO-STATIC THEME FILE */\r\n\r\nbody {}\r\n", "body {}\r\n"},
		{"no warning", "body {}\n\np {}\n", "body {}\n\np {}\n"},
		{"warning not at the start", "body {}\n/* GHO-STATIC THEME FILE */\n\np {}\n", "body {}\n/* GHO-STATIC THEME FILE */\n\np {}\n"},
		{"warning without a blank line after it", "/* GHO-STATIC THEME FILE */\nbody {}\n", "/* GHO-STATIC THEME FILE */\nbody {}\n"},
	}
	for _, c := range cases {
		if got := withoutWarning(c.content); got != c.want {
			t.Errorf("%s: withoutWarning = %q, want %q", c.name, got, c.want)
		}
	}
}

// themeSite writes a site with the Gho-Static theme base, and a themes
// showcase if showcase is set, into a new temporary folder and returns it.
func themeSite(t *testing.T, showcase bool) string {
	t.Helper()
	site := t.TempDir()
	base := gho_static.GhoStaticThemesDir + "/base/"
	files := map[string]string{
		base + "theme.css":                   "/* GHO-STATIC THEME FILE: don't edit it. */\n\nbody {}\n",
		base + "theme_preferences.yaml":      "# GHO-STATIC THEME FILE: don't edit it.\n\nsummary: Base.\nfont: serif\n",
		base + "structure/header/header.css": "header {}\n",
	}
	if showcase {
		files["content/themes/themes.html"] = ""
	}
	writeFiles(t, site, files)
	return site
}

func TestGenerateTheme(t *testing.T) {
	site := themeSite(t, true)
	src := gho_static.GhoStaticThemesDir + "/base"
	if err := GenerateTheme(site, "mine", src, testImages(), "Me", testNow); err != nil {
		t.Fatal(err)
	}
	mine := gho_static.CustomThemesDir + "/mine/"
	if got := readFile(t, site, mine+"theme.css"); got != "body {}\n" {
		t.Errorf("theme.css = %q, want it without the warning", got)
	}
	if got := readFile(t, site, mine+"structure/header/header.css"); got != "header {}\n" {
		t.Errorf("header.css = %q, want it copied as it is", got)
	}
	prefsFile := readFile(t, site, mine+"theme_preferences.yaml")
	var prefs map[string]string
	if err := yaml.Unmarshal([]byte(prefsFile), &prefs); err != nil {
		t.Fatal(err)
	}
	if prefs["summary"] != "A custom theme, based on base." || prefs["font"] != "serif" || strings.Contains(prefsFile, "GHO-STATIC") {
		t.Errorf("theme_preferences.yaml = %q, want the base's without the warning, with a new summary", prefsFile)
	}

	post := readFile(t, site, "content/themes/20260601_mine/post.md")
	for _, want := range []string{"theme: mine", "author: Me", "summary: A custom theme, based on base."} {
		if !strings.Contains(post, want) {
			t.Errorf("the showcase post doesn't contain %q:\n%s", want, post)
		}
	}
	if got := readFile(t, site, "content/themes/20260601_mine/august_kittens_9.jpg"); got != "august_kittens_9.jpg" {
		t.Errorf("the showcase post's picture = %q, want the demo picture", got)
	}

	if err := GenerateTheme(site, "mine", src, testImages(), "", testNow); err == nil {
		t.Error("generating an existing theme didn't fail")
	}
	if got := readFile(t, site, mine+"theme.css"); got != "body {}\n" {
		t.Errorf("the existing theme was changed: theme.css = %q", got)
	}
}

func TestGenerateThemeWithoutShowcase(t *testing.T) {
	site := themeSite(t, false)
	if err := GenerateTheme(site, "mine", gho_static.GhoStaticThemesDir+"/base", testImages(), "", testNow); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(site, "content")); err == nil {
		t.Error("a showcase post was added to a site without a showcase")
	}
}

func TestGenerateThemeFailureRemovesIt(t *testing.T) {
	site := themeSite(t, true)
	images := testImages()
	delete(images, "demo_pics/august_kittens_9.jpg")
	if err := GenerateTheme(site, "mine", gho_static.GhoStaticThemesDir+"/base", images, "", testNow); err == nil {
		t.Fatal("generating a theme without its showcase post's picture didn't fail")
	}
	if _, err := os.Stat(filepath.Join(site, filepath.FromSlash(gho_static.CustomThemesDir), "mine")); err == nil {
		t.Error("the half-made theme was left behind")
	}
}

// localThemes writes a copy of the Gho-Static repo with the themes a, with
// a new theme.css, and b into a new temporary folder, and has the themes
// listed and got from it for the rest of the test.
func localThemes(t *testing.T) {
	t.Helper()
	repo := t.TempDir()
	writeFiles(t, repo, map[string]string{
		gho_static.GhoStaticThemesDir + "/a/theme.css": "new a",
		gho_static.GhoStaticThemesDir + "/b/theme.css": "b",
		gho_static.GhoStaticThemesDir + "/notes.txt":   "not a theme",
	})
	t.Setenv(LocalThemesEnvVar, repo)
}

func TestListThemesLocal(t *testing.T) {
	localThemes(t)
	names, err := ListThemes(output.Output{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !slices.Equal(names, want) {
		t.Errorf("ListThemes = %q, want %q", names, want)
	}
}

func TestGetThemesLocal(t *testing.T) {
	out := output.Output{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	localThemes(t)
	site := t.TempDir()
	writeFiles(t, site, map[string]string{
		gho_static.GhoStaticThemesDir + "/a/theme.css": "old a",
		gho_static.GhoStaticThemesDir + "/a/old.css":   "stale",
		gho_static.CustomThemesDir + "/a/theme.css":    "custom a",
	})

	got, err := GetThemes(site, []string{"a"}, out)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"a"}) {
		t.Errorf("GetThemes(a) got %q, want only a", got)
	}
	if a := readFile(t, site, gho_static.GhoStaticThemesDir+"/a/theme.css"); a != "new a" {
		t.Errorf("a's theme.css = %q, want it replaced", a)
	}
	if _, err := os.Stat(filepath.Join(site, filepath.FromSlash(gho_static.GhoStaticThemesDir), "a", "old.css")); err == nil {
		t.Error("a's old files weren't removed")
	}
	if _, err := os.Stat(filepath.Join(site, filepath.FromSlash(gho_static.GhoStaticThemesDir), "b")); err == nil {
		t.Error("b was got, though only a was asked for")
	}
	if custom := readFile(t, site, gho_static.CustomThemesDir+"/a/theme.css"); custom != "custom a" {
		t.Errorf("the custom theme was changed: %q", custom)
	}

	if got, err := GetThemes(site, nil, out); err != nil || !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("GetThemes(all) = %q, %v, want a and b", got, err)
	}
	if _, err := GetThemes(site, []string{"a", "missing"}, out); err == nil || !strings.Contains(err.Error(), `no theme called "missing"`) {
		t.Errorf("getting a missing theme = %v, want an error naming it", err)
	}
}
