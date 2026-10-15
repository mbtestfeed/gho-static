package gho_static

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"

	"gho-static/utils"
	"gho-static/utils/file_access"
)

type Replacement struct {
	ID   string
	Path string
}

type NavLink struct {
	Name string
	Link       string // relative to the build folder
	Draft      bool
	Unreleased bool
}

// Site holds data shared by every page.
type Site struct {
	Title    string
	Home     string // link to the home page, relative to the build folder
	Version  string // random UUID identifying this build
	Logo     string // logo image, relative to the build folder, or empty if none
	Favicon  string // favicon image, relative to the build folder, or empty if none
	NavLinks []NavLink

	IntroImage       string // relative to the build folder
	IntroName        string
	IntroDescription string
	SocialLinks      []SocialLink // only those with a URL
	RecentBlogs      []SubPage    // the most recently published listed pages
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
	RecentBlogs  template.HTML // empty if the site has no recent blogs template
	Styles   []string
	Scripts  []string
	SubPages []SubPage // only set for content .html templates
	Intro    template.HTML // a content .html template's markdown intro, from the .md file named like it
	Previous *SubPage  // older sibling, for markdown pages listed by a template page
	Next     *SubPage  // newer sibling, for markdown pages listed by a template page
	Tags     []Tag     // every tag on the site's built pages
	Tag      *Tag      // only set when the tags template page is rendered as this tag's page
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
			if !strings.EqualFold(filepath.Ext(f), ".md") {
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
		if strings.EqualFold(filepath.Ext(a), ".css") {
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
	for _, s := range []struct {
		name string
		html *template.HTML
	}{
		{"intro-sidebar.html", &page.IntroSidebar},
		{"recent-blogs.html", &page.RecentBlogs},
	} {
		if tmpl.Lookup(s.name) == nil {
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
	Preview bool // leave out drafts and unreleased pages, as publishing does, but keep local links
	Publish bool // leave out drafts and unreleased pages, and write pages as folder/index.html linked by folder

	Warnings io.Writer // where build warnings are written, or stderr if nil
	Now      time.Time // the build's time, deciding which pages are unreleased, or the current time if zero
}

// warnings returns where o's build warnings are written.
func (o Options) warnings() io.Writer {
	if o.Warnings == nil {
		return os.Stderr
	}
	return o.Warnings
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
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && (ext == ".md" || ext == ".html") {
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

// warning and reset are terminal escape codes to start yellow on black
// warning text, and return to normal text.
const warning, reset = "\033[33;40m", "\033[0m"

// warnUnpublished writes a warning to w for each markdown page directly in
// dir that's a draft or has a publish date in the future, so isn't published.
func (s source) warnUnpublished(dir *file_access.ContentDir, w io.Writer) {
	for _, f := range dir.Files {
		if !strings.EqualFold(filepath.Ext(f), ".md") {
			continue
		}
		meta := s.md[f].Meta
		var reasons []string
		if meta.Draft {
			reasons = append(reasons, "is a draft")
		}
		if meta.Unreleased {
			reasons = append(reasons, "has a publish date in the future ("+meta.ReleaseLine()+")")
		}
		if len(reasons) > 0 {
			fmt.Fprintf(w, "%swarning: %s %s, so it won't be published or deployed%s\n", warning, f, strings.Join(reasons, " and "), reset)
		}
	}
}

// contentPlan is what building the content folder writes.
type contentPlan struct {
	pages   []plannedPage
	static  []staticFile
	skipped []string // folders left out, relative to the content folder, parents first
}

// staticFile is a file copied into the build's static folder.
type staticFile struct {
	name string // relative to the static folder, slash separated
	data []byte
}

// plannedPage is a page built from the content file src.
type plannedPage struct {
	src      string
	file     string   // relative to the build folder, slash separated
	page     Page     // everything but its Styles, Scripts and Content
	markdown *mdFile  // set for markdown pages; others execute src as a template
	imgDir   string   // see mdToHTML
	assets   []string // css and js, relative to the build folder
}

// planContent works out each page and static file built from the content
// folder, and each folder left out, without writing anything. Each folder's
// own css and js are linked after shared, and its images copied. A folder
// whose markdown is a draft or unreleased (and not allowed by opts) has none
// of its own files built, as does a listed folder (one whose parent has a
// template page) with no published date when publishing. Markdown pages are
// linked to their neighbours in their parent folder's template page listing.
func (ls loadedSite) planContent(layout Page, shared []string, opts Options) (contentPlan, error) {
	var plan contentPlan
	root := ls.content.Path
	listings := map[string][]SubPage{}
	err := walk(ls.content, false, func(dir *file_access.ContentDir, listed bool) error {
		rel, err := filepath.Rel(root, dir.Path)
		if err != nil {
			return err
		}
		siblings := listings[path.Dir(dir.Path)]
		var listing []SubPage
		if hasListing(dir) {
			if listing, err = ls.subPages(dir, root, opts); err != nil {
				return err
			}
			listings[dir.Path] = listing
		}

		skip := opts.skip(ls.folderStatus(dir.Path))
		if skip {
			ls.warnUnpublished(dir, opts.warnings())
		}
		if listed && opts.Publish && !skip {
			if undated := ls.undatedPage(dir); undated != "" {
				fmt.Fprintf(opts.warnings(), "error: %s has no published date, so it wasn't published\n", undated)
				skip = true
			}
		}
		if skip {
			plan.skipped = append(plan.skipped, rel)
			return nil
		}

		assets := append([]string{}, shared...)
		for _, f := range dir.Files {
			ext := strings.ToLower(filepath.Ext(f))
			isAsset := slices.Contains(assetExts, ext)
			if !isAsset && !slices.Contains(ImageExts, ext) {
				continue
			}
			data, err := fs.ReadFile(ls, f)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", f, err)
			}
			if len(data) == 0 {
				continue
			}
			name := path.Join(filepath.ToSlash(rel), path.Base(f))
			plan.static = append(plan.static, staticFile{name: name, data: data})
			if isAsset {
				assets = append(assets, "static/"+name)
			}
		}

		for _, f := range dir.Files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext != ".md" && ext != ".html" {
				continue
			}
			stem := strings.TrimSuffix(f, filepath.Ext(f))
			// A template page's intro is built into it, not as its own page.
			if ext == ".md" && slices.Contains(dir.Files, stem+".html") {
				continue
			}
			file, link, err := ls.pagePath(f, root, opts.Publish)
			if err != nil {
				return err
			}
			p := plannedPage{src: f, file: file, page: layout, imgDir: rel, assets: assets}
			p.page.Root = rootFor(file)
			if ext == ".md" {
				md := ls.md[f]
				p.markdown = &md
				p.page.Previous, p.page.Next = adjacent(siblings, link)
			} else {
				p.page.SubPages = listing
				if intro, ok := ls.md[stem+".md"]; ok {
					p.page.Meta = intro.Meta
					p.page.Intro = template.HTML(mdToHTML(intro.Body, p.page.Root, rel, intro.Meta.TOC))
				}
			}
			plan.pages = append(plan.pages, p)

			// The tags page is built again as each tag's own page.
			if rel == tagsDir && strings.EqualFold(filepath.Base(f), tagsDir+".html") {
				for _, tag := range layout.Tags {
					t := plannedPage{src: f, file: tag.file, page: layout, assets: assets}
					t.page.Root, t.page.Tag = rootFor(tag.file), &tag
					plan.pages = append(plan.pages, t)
				}
			}
		}
		return nil
	})
	return plan, err
}

// writeContent writes plan's static files and pages into buildDir, then
// removes the mirrored folders of each folder it left out, if nothing else
// was built in them.
func (ls loadedSite) writeContent(plan contentPlan, buildDir string) error {
	for _, f := range plan.static {
		if err := os.WriteFile(filepath.Join(buildDir, "static", filepath.FromSlash(f.name)), f.data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", f.name, err)
		}
	}
	for _, p := range plan.pages {
		out := filepath.Join(buildDir, filepath.FromSlash(p.file))
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(out), err)
		}
		var err error
		if p.markdown != nil {
			err = writeMarkdownPage(ls.tmpl, p.page, *p.markdown, out, p.imgDir, p.assets)
		} else {
			err = ls.writeTemplatePage(ls.tmpl, p.page, p.src, out, p.assets)
		}
		if err != nil {
			return err
		}
	}
	// Subfolders come after their parents, so are removed first.
	for _, rel := range slices.Backward(plan.skipped) {
		os.Remove(filepath.Join(buildDir, rel))
		os.Remove(filepath.Join(buildDir, "static", rel))
	}
	return nil
}

// fileExists reports whether name exists in fsys.
func fileExists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

// Build generates the site in the folder site into site/build, returning its
// build version.
func Build(site string, opts Options) (string, error) {
	if _, err := os.Stat(site); err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	version := newVersion()
	if err := buildSite(os.DirFS(site), site, opts, version); err != nil {
		return "", err
	}
	return version, nil
}

// buildSite generates the site from the source folder fsys into outDir,
// marked with version.
func buildSite(fsys fs.FS, outDir string, opts Options, version string) error {
	if outDir == "" {
		outDir = "."
	}
	if info, err := os.Stat(outDir); err != nil || !info.IsDir() {
		return fmt.Errorf("invalid output directory: %s", outDir)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	ls, err := loadSite(fsys, now)
	if err != nil {
		return err
	}
	layout, err := ls.layout(opts, version)
	if err != nil {
		return err
	}
	return ls.write(outDir, layout, opts)
}

// loadedSite is a site's source folder, read and parsed ready to build.
type loadedSite struct {
	source
	content *file_access.ContentDir
	prefs   Preferences
	tmpl    *template.Template // the layout and structure templates
	index   mdFile             // the home page
}

// loadSite reads and parses the site in the source folder fsys, marking
// pages published after now as unreleased.
func loadSite(fsys fs.FS, now time.Time) (loadedSite, error) {
	if !fileExists(fsys, "index.html") {
		return loadedSite{}, fmt.Errorf("missing index.html in the site folder")
	}
	if !fileExists(fsys, "index.md") {
		return loadedSite{}, fmt.Errorf("missing index.md in the site folder")
	}

	content, err := file_access.CrawlContent(fsys, "content")
	if err != nil {
		return loadedSite{}, err
	}
	s, err := readSource(fsys, content, now)
	if err != nil {
		return loadedSite{}, err
	}

	prefs, err := readPreferences(fsys, "preferences.yaml")
	if err != nil {
		return loadedSite{}, err
	}

	templates := []string{
		"index.html",
		"structure/header/header.html",
		"structure/footer/footer.html",
	}
	// The sidebars are optional, so sites made before them still build, and
	// can be turned off in preferences. Recent blogs is also left out of
	// sites without a blogs folder.
	for _, sidebar := range []struct {
		name string
		show bool
	}{
		{"intro-sidebar", prefs.ShowIntroSidebar},
		{"recent-blogs", prefs.ShowRecentBlogsSidebar && fileExists(fsys, path.Join(content.Path, "blogs"))},
	} {
		if tmplPath := path.Join("structure", sidebar.name, sidebar.name+".html"); sidebar.show && fileExists(fsys, tmplPath) {
			templates = append(templates, tmplPath)
		}
	}
	tmpl, err := template.ParseFS(fsys, templates...)
	if err != nil {
		return loadedSite{}, fmt.Errorf("failed to parse templates: %w", err)
	}

	indexBytes, err := fs.ReadFile(fsys, "index.md")
	if err != nil {
		return loadedSite{}, fmt.Errorf("failed to read index.md: %w", err)
	}
	meta, body := parseFrontMatter(indexBytes)
	meta.Unreleased = meta.IsUnreleased(now)
	return loadedSite{source: s, content: content, prefs: prefs, tmpl: tmpl, index: mdFile{Meta: meta, Body: body}}, nil
}

// layout returns the page data shared by every page of ls, marked with
// version: its nav links, tags and recent pages, leaving out any opts
// doesn't allow.
func (ls loadedSite) layout(opts Options, version string) (Page, error) {
	content, prefs := ls.content, ls.prefs
	var navLinks []NavLink
	for _, name := range prefs.navLinkNames() {
		srcs, _ := fs.Glob(ls, path.Join(content.Path, name, name+".*"))
		srcs = slices.DeleteFunc(srcs, func(s string) bool {
			ext := strings.ToLower(filepath.Ext(s))
			return ext != ".md" && ext != ".html"
		})
		if len(srcs) == 0 {
			fmt.Fprintf(opts.warnings(), "%swarning: no page found for nav link %q, so it was left out of the menu%s\n", warning, name, reset)
			continue
		}
		src := srcs[0]
		draft, unreleased := ls.folderStatus(path.Dir(src))
		if opts.skip(draft, unreleased) {
			continue
		}
		_, link, err := ls.pagePath(src, content.Path, opts.Publish)
		if err != nil {
			return Page{}, err
		}
		navLinks = append(navLinks, NavLink{Name: strings.ToUpper(name[:1]) + name[1:], Link: link, Draft: draft, Unreleased: unreleased})
	}

	home := "index.html"
	if opts.Publish {
		home = ""
	}
	layout := Page{
		Site: &Site{
			Title:    prefs.SiteName,
			Home:     home,
			Version:  version,
			Logo:     prefs.Logo,
			Favicon:  prefs.Favicon,
			NavLinks: navLinks,

			IntroImage:       prefs.IntroSidebarImage,
			IntroName:        prefs.IntroName,
			IntroDescription: prefs.IntroDescription,
			SocialLinks:      prefs.SocialLinks.populated(),
		},
		Root: rootFor("index.html"),
	}
	var err error
	if layout.Tags, err = ls.collectTags(content, opts); err != nil {
		return Page{}, err
	}
	if layout.Site.RecentBlogs, err = ls.recentPages(content, opts, 3); err != nil {
		return Page{}, err
	}
	return layout, nil
}

// write clears outDir/build, keeping any git repo, and builds ls into it
// with layout.
func (ls loadedSite) write(outDir string, layout Page, opts Options) error {
	content := ls.content
	// Clear the previous build, keeping any git repo it was published from.
	buildDir := filepath.Join(outDir, "build")
	entries, err := os.ReadDir(buildDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", buildDir, err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(buildDir, e.Name())); err != nil {
			return fmt.Errorf("failed to remove %s: %w", e.Name(), err)
		}
	}
	if err := os.MkdirAll(filepath.Join(buildDir, "static"), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", buildDir, err)
	}
	// Stops GitHub Pages running Jekyll over the already built site.
	if err := os.WriteFile(filepath.Join(buildDir, ".nojekyll"), nil, 0644); err != nil {
		return fmt.Errorf("failed to write .nojekyll: %w", err)
	}
	// The site's own static files (images, logo) are copied as is.
	if fileExists(ls, "static") {
		staticFS, err := fs.Sub(ls, "static")
		if err == nil {
			err = os.CopyFS(filepath.Join(buildDir, "static"), staticFS)
		}
		if err != nil {
			return fmt.Errorf("failed to copy static: %w", err)
		}
	}
	for _, dst := range []string{buildDir, filepath.Join(buildDir, "static")} {
		if err := file_access.MirrorDirs(content, content.Path, dst); err != nil {
			return err
		}
	}

	replacements := []Replacement{
		{ID: "content", Path: "index.md"},
		{ID: "header", Path: "structure/header/header.html"},
		{ID: "footer", Path: "structure/footer/footer.html"},
		{ID: "intro-sidebar", Path: "structure/intro-sidebar/intro-sidebar.html"},
		{ID: "recent-blogs", Path: "structure/recent-blogs/recent-blogs.html"},
	}
	var shared []string
	for _, r := range replacements {
		base := strings.TrimSuffix(r.Path, filepath.Ext(r.Path))
		for _, ext := range assetExts {
			name := filepath.Base(base) + ext
			assetBytes, err := fs.ReadFile(ls, base+ext)
			if os.IsNotExist(err) || (err == nil && len(assetBytes) == 0) {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", name, err)
			}
			if ext == ".css" {
				if assetBytes, err = fillCSS(assetBytes, name, ls.prefs); err != nil {
					return err
				}
			}
			if err := os.WriteFile(filepath.Join(buildDir, "static", name), assetBytes, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", name, err)
			}
			shared = append(shared, "static/"+name)
		}
	}

	if err := writeMarkdownPage(ls.tmpl, layout, ls.index, filepath.Join(buildDir, "index.html"), "", shared); err != nil {
		return err
	}

	plan, err := ls.planContent(layout, shared, opts)
	if err != nil {
		return err
	}
	return ls.writeContent(plan, buildDir)
}
