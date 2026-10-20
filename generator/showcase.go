package generator

import (
	"fmt"
	"html"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"gho-static/gho_static"
	"gho-static/utils"
	"gho-static/utils/dates"
)

// showcaseItem is a theme's entry in the themes showcase page: a link to
// its showcase post if the theme is downloaded, otherwise a note that it
// isn't, then its summary. It's given the theme's name and summary, each
// quoted as a template string.
const showcaseItem = `    <li>
        {{- with $.ThemePage %[1]s}}
        <a class="theme-name" href="{{$.Root}}{{.Link}}">{{%[1]s}}</a>
        {{- else}}
        <span class="theme-name">{{%[1]s}}</span> <span class="theme-missing">(not available locally)</span>
        {{- end}}
        {{- if eq $.Site.Theme %[1]s}} <span class="theme-current">(currently selected)</span>{{end}}
        <div class="theme-summary">{{%[2]s}}</div>
    </li>
`

// showcaseTile is a tile in a showcase post's gallery demo, matching the
// gallery page's, given its image's path, title, published date, summary
// and alt text. Its link goes nowhere, as the posts are only for show.
const showcaseTile = `<div class="gallery-item">
<a href="#gallery-demo"><img class="gallery-image" src="%[1]s" alt="%[5]s"><div class="gallery-title">%[2]s</div></a>
<div class="gallery-date">Published on %[3]s</div>
<div class="gallery-summary">%[4]s</div>
<div class="gallery-tags"><span class="tag">example tag</span></div>
</div>
`

// showcaseGallery returns the gallery demo in a showcase post, in the
// content folder dir: a tile for each of pics (Wikimedia Commons file names,
// copied into dir), then one without an image, dated a day apart back from
// now, then the pictures' credits. It's raw HTML, without blank lines, so
// markdown passes it through as one block, with image paths relative to the
// built post (which is two folders deep, as is its folder in static).
func showcaseGallery(dir string, pics []string, now time.Time) string {
	static := "../../static/" + strings.TrimPrefix(dir, "content/") + "/"
	var sb, credits strings.Builder
	sb.WriteString(`<div class="gallery-links" id="gallery-demo">` + "\n")
	for i, pic := range pics {
		name := utils.TitleFor(strings.TrimSuffix(strings.ToLower(pic), path.Ext(pic)))
		date := now.AddDate(0, 0, -(i + 1)).Format("2006/01/02")
		fmt.Fprintf(&sb, showcaseTile, static+strings.ToLower(pic), name, date, "An example gallery post, with one picture.", "")
		credits.WriteString("- " + fmt.Sprintf(demoPicCredit, name, pic))
	}
	date := now.AddDate(0, 0, -(len(pics) + 1)).Format("2006/01/02")
	fmt.Fprintf(&sb, showcaseTile, "../../static/img/gho_placeholder_64x64.svg", "Gallery post without an image", date, "An example gallery post, without a picture.", "No image provided")
	sb.WriteString("</div>\n\n" + credits.String())
	return sb.String()
}

// addShowcasePost adds a showcase post for theme to files, styled with it
// and summarised as summary, published on date, with meta's author and
// edited date. pics holds the demo pictures' contents by lowercased file
// name; those it uses are copied into the post's folder.
func addShowcasePost(files, pics map[string]string, theme, summary string, meta pageMeta, date time.Time) {
	dir := "content/" + gho_static.ShowcaseDir + "/" + dates.PostFolder(theme, date)
	pic := strings.ToLower(demoPics[1])
	credit := fmt.Sprintf(demoPicCredit, "A kitten", demoPics[1])
	tilePics := []string{demoPics[0], demoPics[2], demoPics[3]}
	meta.summary, meta.theme = summary, theme
	body := fmt.Sprintf(showcasePost, html.EscapeString(theme), pic, credit, showcaseGallery(dir, tilePics, date))
	// The gallery's css lays out the gallery demo.
	addPage(files, dir+"/post", ".md", contentPage(theme, date, meta)+"\n"+body, galleryCSS)
	for _, p := range append([]string{demoPics[1]}, tilePics...) {
		files[dir+"/"+strings.ToLower(p)] = pics[strings.ToLower(p)]
	}
}

// addShowcase adds the themes showcase collection to files: a page listing
// each theme in themes (by its theme_preferences.yaml, the default theme
// first), and a post for each, styled with that theme, by meta's author and
// edited date. The default theme's post is published at now, and each
// other's a day before the last. pics holds the demo pictures' contents by
// lowercased file name.
func addShowcase(files, pics map[string]string, themes fs.FS, meta pageMeta, now time.Time) error {
	prefsFiles, err := fs.Glob(themes, gho_static.GhoStaticThemesDir+"/*/theme_preferences.yaml")
	if err != nil {
		return fmt.Errorf("failed to list themes: %w", err)
	}
	names := []string{gho_static.DefaultPreferences.Theme}
	for _, f := range prefsFiles {
		if name := path.Base(path.Dir(f)); !slices.Contains(names, name) {
			names = append(names, name)
		}
	}

	var items strings.Builder
	for i, name := range names {
		b, err := fs.ReadFile(themes, path.Join(gho_static.GhoStaticThemesDir, name, "theme_preferences.yaml"))
		if err != nil {
			return fmt.Errorf("failed to read the %s theme's preferences: %w", name, err)
		}
		var prefs gho_static.ThemePreferences
		if err := yaml.Unmarshal(b, &prefs); err != nil {
			return fmt.Errorf("failed to parse the %s theme's preferences: %w", name, err)
		}
		fmt.Fprintf(&items, showcaseItem, strconv.Quote(name), strconv.Quote(prefs.Summary))
		addShowcasePost(files, pics, name, prefs.Summary, meta, now.AddDate(0, 0, -i))
	}

	page := "content/" + gho_static.ShowcaseDir + "/" + gho_static.ShowcaseDir
	addPage(files, page, ".html", fmt.Sprintf(showcasePageTemplate, items.String()), showcaseCSS)
	files[page+".md"] = showcaseIntro
	return nil
}
