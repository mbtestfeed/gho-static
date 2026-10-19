package gho_static

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"

	"gho-static/utils"
	"gho-static/utils/file_access"
	"gho-static/utils/output"
)

type NavLink struct {
	Name string
	Link       string // relative to the build folder
	Draft      bool
	Unreleased bool
}

// Site holds data shared by every page.
type Site struct {
	Title    string
	Theme    string // the theme selected in settings.yaml
	Home     string // link to the home page, relative to the build folder
	Version  string // random UUID identifying this build
	Logo     string // logo image, relative to the build folder, or empty if none
	Favicon  string // favicon image, relative to the build folder, or empty if none
	NavLinks []NavLink

	IntroImage       string // relative to the build folder
	IntroName        string
	IntroDescription string
	SocialLinks      []SocialLink // only those with a URL
	RecentActivity   []SubPage    // the most recently published posts in the recent activity sidebar's collections
}

// VersionComment returns an HTML comment naming the build version. It's
// given as HTML, since html/template drops comments written in templates.
func (s *Site) VersionComment() template.HTML {
	return template.HTML("<!-- Build version: " + s.Version + " -->")
}

// newVersion returns a random (version 4) UUID.
func newVersion() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Page is the data index.html and the structure templates are executed with.
type Page struct {
	Site    *Site
	Root    string // relative path from the page to the build folder
	Meta     FrontMatter
	Header   template.HTML
	Content  template.HTML
	Footer   template.HTML
	IntroSidebar template.HTML // empty if the site has no intro sidebar template
	RecentActivity template.HTML // empty if the site has no recent activity template
	Styles   []string
	Scripts  []string
	SubPages []SubPage // only set for content .html templates
	Intro    template.HTML // a content .html template's markdown intro, from the .md file named like it
	Previous *SubPage  // older sibling, for markdown pages listed by a template page
	Next     *SubPage  // newer sibling, for markdown pages listed by a template page
	Tags     []Tag     // every tag on the site's built pages
	Tag      *Tag      // only set when the tags template page is rendered as this tag's page
	Images   []string  // only set for gallery pages: their folder's images, relative to the build folder
}

// ThemePage returns the sub-page using theme, as set in its front matter,
// or nil if there's none (as when the theme isn't downloaded).
func (p Page) ThemePage(theme string) *SubPage {
	for i, s := range p.SubPages {
		if s.Theme == theme {
			return &p.SubPages[i]
		}
	}
	return nil
}

// Attribution is the footer's link back to the gho-static project.
// Please don't remove or change attribution. This is a free tool to use and
// adapt, all I ask is that you link back to the original project. Thanks!
const (
	attributionURL = "https://github.com/mbtestfeed/gho-static"
	Attribution    = `<div id="gho-static">Built with <a href="` + attributionURL + `">mbtestfeed/gho-static</a></div>`
)

var assetExts = []string{".css", ".js"}

var ImageExts =[]string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".avif", ".ico"}

// mdToHTML converts md to HTML, with a table of contents at the top if toc is
// set. If imgDir is set, local image links are pointed at their copies under
// static/imgDir, relative to the page via prefix.
func mdToHTML(md []byte, prefix, imgDir string, toc bool) []byte {
	// CommonExtensions already includes tables, fenced code, autolinks and
	// strikethrough; footnotes and heading IDs generated from their text are
	// added to them.
	doc := markdown.Parse(md, parser.NewWithExtensions(parser.CommonExtensions|parser.Footnotes|parser.AutoHeadingIDs))
	if imgDir != "" {
		ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
			img, ok := node.(*ast.Image)
			if !ok || !entering {
				return ast.GoToNext
			}
			dest := string(img.Destination)
			if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "data:") {
				return ast.GoToNext
			}
			img.Destination = []byte(prefix + "static/" + filepath.ToSlash(filepath.Join(imgDir, dest)))
			return ast.GoToNext
		})
	}
	flags := mdhtml.CommonFlags
	if toc {
		flags |= mdhtml.TOC
	}
	return markdown.Render(doc, mdhtml.NewRenderer(mdhtml.RendererOptions{Flags: flags}))
}

// toDiv indents content for placement inside one of the template's divs.
func toDiv(content []byte) template.HTML {
	return template.HTML(utils.IndentFileContent(content, 2))
}

// render executes the named template with page.
func render(tmpl *template.Template, name string, page Page) (string, error) {
	var sb strings.Builder
	if err := tmpl.ExecuteTemplate(&sb, name, page); err != nil {
		return "", fmt.Errorf("failed to render %s: %w", name, err)
	}
	return sb.String(), nil
}

// mdFile is a markdown page's front matter and the markdown after it.
type mdFile struct {
	Meta FrontMatter
	Body []byte
}

// source is the site's source folder, with each markdown file in its content
// folder held by path, so each is only read and parsed once. Paths are slash
// separated, within the source folder.
type source struct {
	fs.FS
	md map[string]mdFile
}

// walk calls fn on dir and then each folder beneath it, with whether the
// folder is listed (its parent has a template page), stopping at the first
// error. listed is dir's own.
func walk(dir *file_access.ContentDir, listed bool, fn func(dir *file_access.ContentDir, listed bool) error) error {
	if err := fn(dir, listed); err != nil {
		return err
	}
	for _, sub := range dir.Dirs {
		if err := walk(sub, hasListing(dir), fn); err != nil {
			return err
		}
	}
	return nil
}

// readSource reads and parses each markdown file in content (crawled from
// fsys) and its subfolders, marking those published after now unreleased.
func readSource(fsys fs.FS, content *file_access.ContentDir, now time.Time) (source, error) {
	s := source{FS: fsys, md: map[string]mdFile{}}
	return s, walk(content, false, func(dir *file_access.ContentDir, _ bool) error {
		for _, f := range dir.Files {
			if !utils.HasExt(f, ".md") {
				continue
			}
			mdBytes, err := fs.ReadFile(fsys, f)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", f, err)
			}
			meta, body := parseFrontMatter(mdBytes)
			meta.Unreleased = meta.IsUnreleased(now)
			s.md[f] = mdFile{Meta: meta, Body: body}
		}
		return nil
	})
}

// writeMarkdownPage converts f into page's content and writes it with
// writePage. See mdToHTML for imgDir.
func writeMarkdownPage(tmpl *template.Template, page Page, f mdFile, out, imgDir string, assets []string) error {
	page.Meta = f.Meta
	page.Content = toDiv(mdToHTML(f.Body, page.Root, imgDir, f.Meta.TOC))
	return writePage(tmpl, page, out, assets)
}

// writePage links assets (paths relative to the build folder) relative to
// the page via page.Root, renders page into the layout and writes it to out.
func writePage(tmpl *template.Template, page Page, out string, assets []string) error {
	for _, a := range assets {
		if utils.HasExt(a, ".css") {
			page.Styles = append(page.Styles, page.Root+a)
		} else {
			page.Scripts = append(page.Scripts, page.Root+a)
		}
	}

	header, err := render(tmpl, "header.html", page)
	if err != nil {
		return err
	}
	footer, err := render(tmpl, "footer.html", page)
	if err != nil {
		return err
	}
	// Please don't remove or change attribution. This is a free tool to use
	// and adapt, all I ask is that you link back to the original project.
	// Thanks!
	if !strings.Contains(footer, `id="gho-static"`) || !strings.Contains(footer, attributionURL) {
		footer += Attribution + "\n"
	}
	page.Header, page.Footer = toDiv([]byte(header)), toDiv([]byte(footer))
	// The recent activity sidebar is left out when it has nothing to list.
	for _, s := range []struct {
		name string
		html *template.HTML
		show bool
	}{
		{"intro-sidebar.html", &page.IntroSidebar, true},
		{"recent-activity.html", &page.RecentActivity, len(page.Site.RecentActivity) > 0},
	} {
		if !s.show || tmpl.Lookup(s.name) == nil {
			continue
		}
		sidebar, err := render(tmpl, s.name, page)
		if err != nil {
			return err
		}
		*s.html = toDiv([]byte(sidebar))
	}

	html, err := render(tmpl, "index.html", page)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", out, err)
	}
	return nil
}

// Options controls which content is built.
type Options struct {
	Preview    bool // leave out drafts and unreleased pages, as publishing does, but keep local links
	Publish    bool // leave out drafts and unreleased pages, and write pages as folder/index.html linked by folder
	WithThemes bool // keep the themes showcase collection when previewing or publishing

	Output output.Output // where build warnings are written
	Now    time.Time     // the build's time, deciding which pages are unreleased, or the current time if zero
}

// PagePath returns where the page built from src (a .md or .html file under
// the content folder root) is written, relative to the build folder, and the
// link to it. When publishing, pages are written as index.html and linked by
// folder: a folder's only page takes the folder itself, otherwise each page
// gets a subfolder named after it.
func PagePath(src, root string, publish bool) (file, link string, err error) {
	rel, err := filepath.Rel(root, src)
	if err != nil {
		return "", "", err
	}
	return source{FS: os.DirFS(root)}.pagePath(filepath.ToSlash(rel), ".", publish)
}

// pagePath is PagePath for src and root in s.
func (s source) pagePath(src, root string, publish bool) (file, link string, err error) {
	rel, err := filepath.Rel(root, src)
	if err != nil {
		return "", "", err
	}
	base := filepath.ToSlash(strings.TrimSuffix(rel, filepath.Ext(rel)))
	if !publish {
		return base + ".html", base + ".html", nil
	}

	entries, err := fs.ReadDir(s, path.Dir(src))
	if err != nil {
		return "", "", fmt.Errorf("failed to read %s: %w", path.Dir(src), err)
	}
	// A template page and its markdown intro share a name, and are one page.
	pages := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() && utils.HasExt(e.Name(), ".md", ".html") {
			pages[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = true
		}
	}
	if len(pages) == 1 {
		base = path.Dir(base)
	}
	return path.Join(base, "index.html"), base + "/", nil
}

// rootFor returns the relative path from a page written at file (relative to
// the build folder) back to the build folder.
func rootFor(file string) string {
	if depth := strings.Count(file, "/"); depth > 0 {
		return strings.Repeat("../", depth)
	}
	return "./"
}

// skip reports whether a folder with the given status should not be built.
func (o Options) skip(draft, unreleased bool) bool {
	return (draft || unreleased) && (o.Preview || o.Publish)
}

// folderStatus reports whether any markdown file directly in dir is a draft
// or unreleased.
func (s source) folderStatus(dir string) (draft, unreleased bool) {
	for p, f := range s.md {
		if path.Dir(p) == dir {
			draft = draft || f.Meta.Draft
			unreleased = unreleased || f.Meta.Unreleased
		}
	}
	return draft, unreleased
}

// ShowcaseDir is the content folder of the themes showcase collection, left
// out when previewing or publishing unless asked for.
const ShowcaseDir = "themes"

// leftOut reports whether the content folder dir (under the content folder
// root) isn't built: it's in the themes showcase when previewing or
// publishing without opts.WithThemes, or a markdown file directly in it uses
// a theme that isn't downloaded.
func (s source) leftOut(dir, root string, opts Options) bool {
	if rel := strings.TrimPrefix(dir, root+"/"); (opts.Preview || opts.Publish) && !opts.WithThemes &&
		(rel == ShowcaseDir || strings.HasPrefix(rel, ShowcaseDir+"/")) {
		return true
	}
	for p, f := range s.md {
		if path.Dir(p) == dir && f.Meta.Theme != "" && !hasTheme(s, f.Meta.Theme) {
			return true
		}
	}
	return false
}

// warnUnpublished writes a warning for each markdown page directly in dir
// that's a draft or has a publish date in the future, so isn't published.
func (ls loadedSite) warnUnpublished(dir *file_access.ContentDir) {
	for _, f := range dir.Files {
		if !utils.HasExt(f, ".md") {
			continue
		}
		meta := ls.md[f].Meta
		var reasons []string
		if meta.Draft {
			reasons = append(reasons, "is a draft")
		}
		if meta.Unreleased {
			reasons = append(reasons, "has a publish date in the future ("+meta.ReleaseLine()+")")
		}
		if len(reasons) > 0 {
			ls.opts.Output.Warnf("%s %s, so it won't be published or deployed", f, strings.Join(reasons, " and "))
		}
	}
}
