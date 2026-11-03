package generator

import (
	"strings"
	"testing"
	"testing/fstest"

	"gho-static/gho_static"
)

func TestShowcaseGallery(t *testing.T) {
	got := showcaseGallery("content/themes/20260601_mine", []string{demoPics[0]}, testNow)
	for _, want := range []string{
		`src="../../static/themes/20260601_mine/august_kittens_7.jpg"`,
		`<div class="gallery-title">August kittens 7</div>`,
		"Published on 2026/05/31",
		`src="../../static/img/gho_placeholder_64x64.svg" alt="No image provided"`,
		"Published on 2026/05/30",
		"- [August kittens 7](https://commons.wikimedia.org/wiki/File:August_Kittens_7.jpg)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the gallery doesn't contain %q:\n%s", want, got)
		}
	}
	// A blank line would end markdown's HTML block early.
	if gallery, _, _ := strings.Cut(got, "</div>\n\n"); strings.Contains(gallery, "\n\n") {
		t.Errorf("the gallery's HTML has a blank line in it:\n%s", gallery)
	}
}

func TestAddShowcasePost(t *testing.T) {
	files := map[string]string{}
	addShowcasePost(files, map[string]string{"august_kittens_9.jpg": "k9"}, "<mine>", "Mine.", pageMeta{author: "Me"}, testNow)
	dir := "content/themes/20260601_<mine>/"
	post, ok := files[dir+"post.md"]
	if !ok {
		t.Fatalf("no post was added: %q", files)
	}
	for _, want := range []string{"summary: Mine.", "author: Me", "theme: <mine>", "**&lt;mine&gt;** theme"} {
		if !strings.Contains(post, want) {
			t.Errorf("the post doesn't contain %q:\n%s", want, post)
		}
	}
	if files[dir+"post.css"] != galleryCSS {
		t.Error("the post isn't styled with the gallery's css")
	}
	if files[dir+"august_kittens_9.jpg"] != "k9" {
		t.Error("the post's picture wasn't copied into its folder")
	}
	for _, pic := range []string{"august_kittens_7.jpg", "august_kittens_6.jpg", "august_kittens_4.jpg"} {
		if _, ok := files[dir+pic]; !ok {
			t.Errorf("the gallery demo's %s wasn't copied into the post's folder", pic)
		}
	}
}

func TestAddShowcase(t *testing.T) {
	files := map[string]string{}
	if err := addShowcase(files, map[string]string{}, testThemes(), pageMeta{}, testNow); err != nil {
		t.Fatal(err)
	}
	page := files["content/themes/themes.html"]
	for _, want := range []string{`$.ThemePage "default_developer_ugly"`, `{{"Plain."}}`, `$.ThemePage "brochureware"`, `{{"Shiny."}}`} {
		if !strings.Contains(page, want) {
			t.Errorf("the showcase page doesn't contain %q:\n%s", want, page)
		}
	}
	if files["content/themes/themes.md"] != showcaseIntro {
		t.Error("the showcase page's intro isn't the showcase intro")
	}
	if files["content/themes/themes.css"] != showcaseCSS {
		t.Error("the showcase page isn't styled with the showcase css")
	}

	themes := testThemes()
	themes[gho_static.GhoStaticThemesDir+"/broken/theme_preferences.yaml"] = &fstest.MapFile{Data: []byte("summary: [unclosed\n")}
	if err := addShowcase(map[string]string{}, map[string]string{}, themes, pageMeta{}, testNow); err == nil || !strings.Contains(err.Error(), "failed to parse the broken theme's preferences") {
		t.Errorf("a theme with invalid preferences = %v, want an error naming it", err)
	}
}
