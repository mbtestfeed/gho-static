package generator

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"gho-static/gho_static"
)

// starter holds the starter site's files, laid out as in the target folder.
//
//go:embed all:starter
var starter embed.FS

// listingPageTemplate is a page listing the posts in its folder, filled
// with its title.
//
//go:embed templates/listing.html
var listingPageTemplate string

// listingCSS styles a listing page.
//
//go:embed templates/listing.css
var listingCSS string

// contentPageTemplate is a generic content page or post, filled with its
// title, published date and draft status.
//
//go:embed templates/content.md
var contentPageTemplate string

// contactText is the starter site's contact page body.
//
//go:embed templates/contact.md
var contactText string

// titleFor returns name with underscores as spaces and its first letter
// capitalised.
func titleFor(name string) string {
	title := []rune(strings.ReplaceAll(name, "_", " "))
	if len(title) > 0 {
		title[0] = unicode.ToUpper(title[0])
	}
	return string(title)
}

// pageMeta is the front matter of a generated page, other than its title
// and published date. Unset author and edited lines are left out.
type pageMeta struct {
	summary, author, edited string
	draft, toc              bool
	tags                    []string
}

// contentPage returns the contents of a generic page or post titled after
// name, published on date, with meta.
func contentPage(name string, date time.Time, meta pageMeta) string {
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
	return fmt.Sprintf(contentPageTemplate, titleFor(name), summary, line("author", meta.author), date.Format("2006-01-02"),
		line("edited", meta.edited), meta.draft, meta.toc, strings.Join(meta.tags, ", "))
}

// examplePosts are the front matter of each example blog post, in order.
// The first summary is long enough to show it being cut short in listings.
var examplePosts = []pageMeta{
	{summary: "The first example post. Its summary runs on for a while, so that the blogs page shows how a long summary is cut short with an ellipsis once it passes two hundred characters, rather than filling the listing.", toc: true, tags: []string{"example tag", "another example tag"}},
	{summary: "The second example post.", tags: []string{"different example tag", "example tag"}},
	{summary: "The third example post, without tags."},
}

// listingPage returns the contents of a post listing page titled after name.
func listingPage(name string) string {
	return fmt.Sprintf(listingPageTemplate, titleFor(name))
}

// PostFolder returns the folder name for post name published on date.
func PostFolder(name string, date time.Time) string {
	return date.Format("20060102") + "_" + name
}

// Generate writes a starter site with the given preferences into dir, with
// images copied into static/img, refusing to overwrite existing files. A
// logo or favicon other than the default is copied from its path into
// static/img too.
func Generate(dir string, prefs gho_static.Preferences, images fs.FS) error {
	files := map[string]string{}
	starterFiles, _ := fs.Sub(starter, "starter")
	if err := addFiles(files, starterFiles, ""); err != nil {
		return fmt.Errorf("failed to read starter files: %w", err)
	}
	if err := addFiles(files, images, "static/img/"); err != nil {
		return fmt.Errorf("failed to read images: %w", err)
	}
	for _, img := range []struct {
		name       string
		path       *string
		defaultVal string
	}{
		{"logo", &prefs.Logo, gho_static.DefaultPreferences.Logo},
		{"favicon", &prefs.Favicon, gho_static.DefaultPreferences.Favicon},
		{"intro sidebar image", &prefs.IntroSidebarImage, gho_static.DefaultPreferences.IntroSidebarImage},
	} {
		if *img.path == img.defaultVal {
			continue
		}
		if !slices.Contains(gho_static.ImageExts, strings.ToLower(filepath.Ext(*img.path))) {
			return fmt.Errorf("can't use %s %s: it must be one of these image types: %s", img.name, *img.path, strings.Join(gho_static.ImageExts, ", "))
		}
		b, err := os.ReadFile(*img.path)
		if err != nil {
			return fmt.Errorf("can't copy %s %s into static/img: %w", img.name, *img.path, err)
		}
		*img.path = "static/img/" + filepath.Base(*img.path)
		files[*img.path] = string(b)
	}
	prefsYAML, err := yaml.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("failed to encode preferences: %w", err)
	}
	if err := newFolder(dir); err != nil {
		return err
	}
	now := time.Now()
	files["preferences.yaml"] = string(prefsYAML)
	// Example pages use every front matter field, so they show what's possible.
	author := prefs.DefaultAuthor
	if author == "" {
		author = "Gho-Static"
	}
	edited := now.Format("2006-01-02")
	for i, meta := range examplePosts {
		name, date := fmt.Sprintf("example_%d", i+1), now.AddDate(0, -i, 0)
		meta.author, meta.edited = author, edited
		addPage(files, "content/blogs/"+PostFolder(name, date)+"/post", ".md", contentPage(name, date, meta), "")
	}
	for _, name := range []string{"about", "contact"} {
		meta := pageMeta{summary: "The site's " + name + " page.", author: author, edited: edited, tags: []string{"example tag"}}
		addPage(files, "content/"+name+"/"+name, ".md", contentPage(name, now, meta), "")
	}
	files["content/contact/contact.md"] += contactText
	addPage(files, "content/blogs/blogs", ".html", listingPage("blogs"), listingCSS)
	return writeFiles(dir, files)
}

// GeneratePage creates the folder dir and writes a draft content page into
// it, named and titled after the folder, by author if set.
func GeneratePage(dir, author string) error {
	if err := newFolder(dir); err != nil {
		return err
	}
	name := filepath.Base(dir)
	files := map[string]string{}
	addPage(files, name, ".md", contentPage(name, time.Now(), pageMeta{author: author, draft: true}), "")
	return writeFiles(dir, files)
}

// GeneratePost creates the folder dir and writes a draft post.md for post
// name, published today, by author if set, into it. If dir's parent folder
// doesn't exist, it is created with a listing page named after it.
func GeneratePost(dir, name, author string) error {
	parent := filepath.Dir(dir)
	files := map[string]string{}
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		parentName := filepath.Base(parent)
		addPage(files, parentName, ".html", listingPage(parentName), listingCSS)
	}
	if err := newFolder(dir); err != nil {
		return err
	}
	addPage(files, filepath.Base(dir)+"/post", ".md", contentPage(name, time.Now(), pageMeta{author: author, draft: true}), "")
	return writeFiles(parent, files)
}

// DatePost sets the published date of the markdown page at mdPath in its
// front matter (adding one if needed) and renames its folder to the
// yyyymmdd_<name> form, replacing any existing date prefix.
func DatePost(mdPath string, date time.Time) error {
	dir := filepath.Dir(mdPath)
	name := gho_static.PostName(filepath.Base(dir))
	if name == "" {
		name = "untitled"
	}
	newDir := filepath.Join(filepath.Dir(dir), PostFolder(name, date))
	if _, err := os.Stat(newDir); err == nil && newDir != dir {
		return fmt.Errorf("can't rename %s: %s already exists", dir, newDir)
	}

	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mdPath, err)
	}
	md := strings.ReplaceAll(string(mdBytes), "\r\n", "\n")
	line := "published: " + date.Format("2006-01-02") + "\n"
	if end := strings.Index(md[min(4, len(md)):], "\n---"); strings.HasPrefix(md, "---\n") && end != -1 {
		// Add as the last line, overriding any invalid published value.
		i := 4 + end + 1
		md = md[:i] + line + md[i:]
	} else {
		md = "---\n" + line + "---\n\n" + md
	}
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", mdPath, err)
	}

	if newDir != dir {
		if err := os.Rename(dir, newDir); err != nil {
			return fmt.Errorf("failed to rename %s: %w", dir, err)
		}
	}
	return nil
}

// PublishDraft removes the draft line from the front matter of the markdown
// page at mdPath.
func PublishDraft(mdPath string) error {
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mdPath, err)
	}
	md := strings.ReplaceAll(string(mdBytes), "\r\n", "\n")
	end := strings.Index(md[min(4, len(md)):], "\n---")
	if !strings.HasPrefix(md, "---\n") || end == -1 {
		return nil
	}
	header := strings.Split(md[4:4+end], "\n")
	header = slices.DeleteFunc(header, func(line string) bool {
		key, _, _ := strings.Cut(line, ":")
		return strings.TrimSpace(key) == "draft"
	})
	md = "---\n" + strings.Join(header, "\n") + md[4+end:]
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", mdPath, err)
	}
	return nil
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

// writeFiles writes each of files (slash separated paths relative to dir)
// into dir, creating folders as needed and refusing to overwrite existing
// files.
func writeFiles(dir string, files map[string]string) error {
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
		}
		if err := writeNewFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// newFolder creates dir along with any missing parents, returning an
// os.IsExist error if dir itself already exists.
func newFolder(dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	return os.Mkdir(dir, 0755)
}

// writeNewFile writes content to path, refusing to overwrite an existing file.
func writeNewFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	_, err = f.WriteString(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
