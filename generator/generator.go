package generator

import (
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"gho-static/gho_static"
	"gho-static/utils"
	"gho-static/utils/dates"
	"gho-static/utils/file_access"
	"gho-static/utils/front_matter"
)

// starter holds the starter site's files, laid out as in the target folder.
//
//go:embed all:starter
var starter embed.FS

// listingPageTemplate is a page listing the posts in its folder, after its
// markdown intro.
//
//go:embed templates/listing.html
var listingPageTemplate string

// listingCSS styles a listing page.
//
//go:embed templates/listing.css
var listingCSS string

// showcasePageTemplate is the themes showcase page, listing the themes known
// when Gho-Static was built (filled in at %s) after its markdown intro.
//
//go:embed templates/showcase.html
var showcasePageTemplate string

// showcaseCSS styles the themes showcase page.
//
//go:embed templates/showcase.css
var showcaseCSS string

// showcaseIntro is the themes showcase page's markdown intro.
//
//go:embed templates/showcase.md
var showcaseIntro string

// showcasePost is a theme's showcase post body, given the theme's name, and
// an example image's file name and credit.
//
//go:embed templates/showcase_post.md
var showcasePost string

// galleryPageTemplate is a page linking to each post in its folder by its
// image, after its markdown intro.
//
//go:embed templates/gallery.html
var galleryPageTemplate string

// galleryCSS styles a gallery page.
//
//go:embed templates/gallery.css
var galleryCSS string

// contentPageTemplate is a generic content page or post's front matter,
// filled with its title, published date and draft status.
//
//go:embed templates/content.md
var contentPageTemplate string

// contactText is the starter site's contact page body.
//
//go:embed templates/contact.md
var contactText string

// pageMeta is the front matter of a generated page, other than its title
// and published date. Unset author and edited lines are left out. Clean
// pages have only front matter, without a heading.
type pageMeta struct {
	summary, author, edited string
	theme                   string // styles the page instead of the site's theme
	draft, toc, clean       bool
	tags                    []string
}

// contentPage returns the contents of a generic page or post titled after
// name, published on date, with meta.
func contentPage(name string, date time.Time, meta pageMeta) string {
	page := frontMatter(name, date, meta)
	if !meta.clean {
		page += "\n# " + utils.TitleFor(name) + "\n"
	}
	return page
}

// frontMatter returns the front matter of a generic page or post titled
// after name, published on date, with meta.
func frontMatter(name string, date time.Time, meta pageMeta) string {
	line := func(key, val string) string {
		if val == "" {
			return ""
		}
		return key + ": " + val + "\n"
	}
	summary := meta.summary
	if summary != "" {
		summary = " " + summary
	}
	return fmt.Sprintf(contentPageTemplate, utils.TitleFor(name), summary, line("author", meta.author), date.Format("2006-01-02"),
		line("edited", meta.edited), meta.draft, meta.toc, strings.Join(meta.tags, ", "), line("theme", meta.theme))
}

// examplePosts are the front matter of each example blog post, in order.
// The first summary is long enough to show it being cut short in listings.
var examplePosts = []pageMeta{
	{summary: "The first example post. Its summary runs on for a while, so that the blogs page shows how a long summary is cut short with an ellipsis once it passes two hundred characters, rather than filling the listing.", toc: true, tags: []string{"example tag", "another example tag"}},
	{summary: "The second example post.", tags: []string{"different example tag", "example tag"}},
	{summary: "The third example post, without tags."},
}

// demoPics are the Wikimedia Commons file names of the example gallery posts'
// images, in post order, each saved lowercased in img/demo_pics.
var demoPics = []string{"August_Kittens_7.jpg", "August_Kittens_9.jpg", "August_Kittens_6.jpg", "August_Kittens_4.jpg"}

// demoPicCredit credits a demo picture, given its display name and Wikimedia
// Commons file name.
const demoPicCredit = "[%s](https://commons.wikimedia.org/wiki/File:%s) by MilasAbd on Wikimedia Commons, dedicated to the public domain under [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/).\n"

// addDemoGallery adds a gallery post for each of demoPics to files, the nth
// published n days before now, and one with all of them published at now.
// pics holds the pictures' contents by lowercased file name.
func addDemoGallery(files, pics map[string]string, meta pageMeta, now time.Time) {
	meta.tags = []string{"kittens"}
	allDir := "content/gallery/" + dates.PostFolder("all_kitten_pictures", now)
	var credits string
	for i, pic := range demoPics {
		file := strings.ToLower(pic)
		name := strings.TrimSuffix(file, filepath.Ext(file))
		date := now.AddDate(0, 0, -(i + 1))
		dir := "content/gallery/" + dates.PostFolder(name, date)
		credit := fmt.Sprintf(demoPicCredit, utils.TitleFor(name), pic)
		meta.summary = "An example gallery post, with one picture."
		addPage(files, dir+"/post", ".md", contentPage(name, date, meta)+"\n"+credit, "")
		files[dir+"/"+file] = pics[file]
		files[allDir+"/"+file] = pics[file]
		credits += "- " + credit
	}
	meta.summary = "An example gallery post, with every picture."
	addPage(files, allDir+"/post", ".md", contentPage("all_kitten_pictures", now, meta)+"\n"+credits, "")

	// Shown in the gallery with the placeholder in place of an image.
	name, date := "gallery_post_without_an_image", now.AddDate(0, 0, -(len(demoPics) + 1))
	meta.summary, meta.tags = "An example gallery post, without a picture.", nil
	addPage(files, "content/gallery/"+dates.PostFolder(name, date)+"/post", ".md", contentPage(name, date, meta), "")
}

// cleanIndexText is a clean site's home page body.
const cleanIndexText = "You've built a static website in 'Clean' mode. There's nothing here - you're going to need to fill this out. Update your basic settings in settings.yaml, and add new pages using <code>gho-static --generate content/[new_page_name]</code> or new blogs using <code>gho-static --generate content/blogs/[my_new_blog_title]</code>. Please refer to mbtestfeed.github.io for additional documentation.\n"

// addListing adds a post listing page at path (slash separated, without
// extension) to files, with a markdown intro titled after name.
func addListing(files map[string]string, path, name string) {
	addPage(files, path, ".html", listingPageTemplate, listingCSS)
	files[path+".md"] = "# " + utils.TitleFor(name) + "\n"
}

// addGallery adds the gallery page, linking to each post in its folder by
// its image, to files.
func addGallery(files map[string]string) {
	addPage(files, "content/gallery/gallery", ".html", galleryPageTemplate, galleryCSS)
	files["content/gallery/gallery.md"] = "# Gallery\n"
}

// Generate writes a starter site with the given preferences into dir, with
// images copied into static/img and themes into themes, refusing to
// overwrite existing files. A logo or favicon other than the default is
// copied from its path into static/img too. Its home page is readme. A clean
// site has a single empty post instead of the examples, and its pages have
// only front matter.
func Generate(dir string, prefs gho_static.Preferences, images, themes fs.FS, readme string, clean bool) error {
	custom := map[string]string{}
	for _, img := range []struct {
		name       string
		path       *string
		defaultVal string
	}{
		{"logo", &prefs.Logo, gho_static.DefaultPreferences.Logo},
		{"favicon", &prefs.Favicon, gho_static.DefaultPreferences.Favicon},
		{"intro sidebar image", &prefs.IntroSidebarImage, gho_static.DefaultPreferences.IntroSidebarImage},
	} {
		if *img.path == "" || *img.path == img.defaultVal {
			continue
		}
		if !utils.HasExt(*img.path, gho_static.ImageExts...) {
			return fmt.Errorf("can't use %s %s: it must be one of these image types: %s", img.name, *img.path, strings.Join(gho_static.ImageExts, ", "))
		}
		b, err := os.ReadFile(*img.path)
		if err != nil {
			return fmt.Errorf("can't copy %s %s into static/img: %w", img.name, *img.path, err)
		}
		*img.path = "static/img/" + filepath.Base(*img.path)
		custom[*img.path] = string(b)
	}
	files, err := siteFiles(prefs, images, themes, readme, clean, time.Now())
	if err != nil {
		return err
	}
	maps.Copy(files, custom)
	if err := file_access.NewFolder(dir); err != nil {
		return err
	}
	return file_access.WriteFiles(dir, files)
}

// siteFiles returns the files of a starter site with the given preferences,
// generated at now, by slash separated path. See Generate.
func siteFiles(prefs gho_static.Preferences, images, themes fs.FS, readme string, clean bool, now time.Time) (map[string]string, error) {
	files := map[string]string{}
	starterFiles, _ := fs.Sub(starter, "starter")
	if err := addFiles(files, starterFiles, ""); err != nil {
		return nil, fmt.Errorf("failed to read starter files: %w", err)
	}
	if err := addFiles(files, images, "static/img/"); err != nil {
		return nil, fmt.Errorf("failed to read images: %w", err)
	}
	defaultTheme, _ := fs.Sub(themes, gho_static.DefaultPreferences.Theme)
	if err := addFiles(files, defaultTheme, "themes/"+gho_static.DefaultPreferences.Theme+"/"); err != nil {
		return nil, fmt.Errorf("failed to read the default theme: %w", err)
	}
	// Demo pictures go in the example gallery posts, not static/img.
	pics := map[string]string{}
	for name, data := range files {
		if file, ok := strings.CutPrefix(name, "static/img/demo_pics/"); ok {
			pics[file] = data
			delete(files, name)
		}
	}
	if !clean {
		// The themes showcase goes after the gallery in the menu.
		i := slices.Index(prefs.NavLinks, "gallery") + 1
		if i == 0 {
			i = len(prefs.NavLinks)
		}
		prefs.NavLinks = slices.Insert(slices.Clone(prefs.NavLinks), i, gho_static.ShowcaseDir)
	}
	prefsYAML, err := yaml.Marshal(prefs)
	if err != nil {
		return nil, fmt.Errorf("failed to encode preferences: %w", err)
	}
	files["settings.yaml"] = string(prefsYAML)
	if clean {
		files["index.md"] = "---\ntitle:\n---\n\n" + cleanIndexText
		for _, name := range []string{"about", "contact"} {
			addPage(files, "content/"+name+"/"+name, ".md", contentPage(name, now, pageMeta{author: prefs.DefaultAuthor, clean: true}), "")
		}
		addPage(files, "content/blogs/"+dates.PostFolder("first_post", now)+"/post", ".md", contentPage("first_post", now, pageMeta{author: prefs.DefaultAuthor, clean: true}), "")
		addListing(files, "content/blogs/blogs", "blogs")
		addGallery(files)
		return files, nil
	}
	files["index.md"] = "---\ntitle:\n---\n\n" + readme
	// Example pages use every front matter field, so they show what's possible.
	author := prefs.DefaultAuthor
	if author == "" {
		author = "Gho-Static"
	}
	edited := now.Format("2006-01-02")
	for i, meta := range examplePosts {
		name, date := fmt.Sprintf("example_%d", i+1), now.AddDate(0, -i, 0)
		meta.author, meta.edited = author, edited
		addPage(files, "content/blogs/"+dates.PostFolder(name, date)+"/post", ".md", contentPage(name, date, meta), "")
	}
	addDemoGallery(files, pics, pageMeta{author: author, edited: edited}, now)
	for _, name := range []string{"about", "contact"} {
		meta := pageMeta{summary: "The site's " + name + " page.", author: author, edited: edited, tags: []string{"example tag"}}
		addPage(files, "content/"+name+"/"+name, ".md", contentPage(name, now, meta), "")
	}
	files["content/contact/contact.md"] += contactText
	addListing(files, "content/blogs/blogs", "blogs")
	addGallery(files)
	if err := addShowcase(files, pics, themes, pageMeta{author: author, edited: edited}, now); err != nil {
		return nil, err
	}
	return files, nil
}

// GeneratePage creates the folder dir and writes a draft content page into
// it, named and titled after the folder, published at now, by author if set.
// A clean page has only front matter.
func GeneratePage(dir, author string, clean bool, now time.Time) error {
	if err := file_access.NewFolder(dir); err != nil {
		return err
	}
	name := filepath.Base(dir)
	files := map[string]string{}
	addPage(files, name, ".md", contentPage(name, now, pageMeta{author: author, draft: true, clean: clean}), "")
	return file_access.WriteFiles(dir, files)
}

// GeneratePost creates the folder dir and writes a draft post.md for post
// name, published at now, by author if set, into it. If dir's parent folder
// doesn't exist, it is created with a listing page named after it. A clean
// post has only front matter.
func GeneratePost(dir, name, author string, clean bool, now time.Time) error {
	parent := filepath.Dir(dir)
	files := map[string]string{}
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		parentName := filepath.Base(parent)
		addListing(files, parentName, parentName)
	}
	if err := file_access.NewFolder(dir); err != nil {
		return err
	}
	addPage(files, filepath.Base(dir)+"/post", ".md", contentPage(name, now, pageMeta{author: author, draft: true, clean: clean}), "")
	return file_access.WriteFiles(parent, files)
}

// DatePost sets the published date of the markdown page at mdPath in its
// front matter (adding one if needed) and renames its folder to the
// yyyymmdd_<name> form, replacing any existing date prefix.
func DatePost(mdPath string, date time.Time) error {
	dir := filepath.Dir(mdPath)
	name := dates.PostName(filepath.Base(dir))
	if name == "" {
		name = "untitled"
	}
	newDir := filepath.Join(filepath.Dir(dir), dates.PostFolder(name, date))
	if _, err := os.Stat(newDir); err == nil && newDir != dir {
		return fmt.Errorf("can't rename %s: %s already exists", dir, newDir)
	}

	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mdPath, err)
	}
	if err := os.WriteFile(mdPath, []byte(setPublished(string(mdBytes), date)), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", mdPath, err)
	}

	if newDir != dir {
		if err := os.Rename(dir, newDir); err != nil {
			return fmt.Errorf("failed to rename %s: %w", dir, err)
		}
	}
	return nil
}

// setPublished returns md with its published date set to date in its front
// matter, adding front matter if it has none.
func setPublished(md string, date time.Time) string {
	line := "published: " + date.Format("2006-01-02") + "\n"
	if header, rest, ok := front_matter.Split(md); ok {
		// Add as the last line, overriding any invalid published value.
		return "---\n" + header + "\n" + line + rest
	}
	return "---\n" + line + "---\n\n" + strings.ReplaceAll(md, "\r\n", "\n")
}

// PublishDraft removes the draft line from the front matter of the markdown
// page at mdPath.
func PublishDraft(mdPath string) error {
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mdPath, err)
	}
	md := removeDraft(string(mdBytes))
	if md == string(mdBytes) {
		return nil
	}
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", mdPath, err)
	}
	return nil
}

// removeDraft returns md without the draft line in its front matter, or
// unchanged if it has no front matter.
func removeDraft(md string) string {
	header, rest, ok := front_matter.Split(md)
	if !ok {
		return md
	}
	lines := slices.DeleteFunc(strings.Split(header, "\n"), func(line string) bool {
		key, _, _ := strings.Cut(line, ":")
		return strings.TrimSpace(key) == "draft"
	})
	return "---\n" + strings.Join(lines, "\n") + "\n" + rest
}

// addFiles adds each file in fsys to files, at its path with prefix added.
func addFiles(files map[string]string, fsys fs.FS, prefix string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, path)
		files[prefix+path] = string(b)
		return err
	})
}

// addPage adds the page at path (slash separated, without extension) with
// the given extension and content to files, along with a matching css file
// and an empty js file.
func addPage(files map[string]string, path, ext, content, css string) {
	files[path+ext] = content
	files[path+".css"] = css
	files[path+".js"] = ""
}
