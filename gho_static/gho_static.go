package gho_static

import (
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

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
	Logo     string // logo image, relative to the build folder, or empty if none
	Favicon  string // favicon image, relative to the build folder, or empty if none
	NavLinks []NavLink
}

// Page is the data index.html, header.html and footer.html are executed with.
type Page struct {
	Site    *Site
	Root    string // relative path from the page to the build folder
	Meta     FrontMatter
	Header   template.HTML
	Content  template.HTML
	Footer   template.HTML
	Styles   []string
	Scripts  []string
	SubPages []SubPage // only set for content .html templates
	Previous *SubPage  // older sibling, for markdown pages listed by a template page
	Next     *SubPage  // newer sibling, for markdown pages listed by a template page
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

// writeMarkdownPage converts the markdown at mdPath into page's content and
// writes it with writePage. See mdToHTML for imgDir.
func writeMarkdownPage(tmpl *template.Template, page Page, mdPath, out, imgDir string, assets []string) error {
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", mdPath, err)
	}
	meta, body := parseFrontMatter(mdBytes)
	page.Meta = meta
	page.Content = toDiv(mdToHTML(body, page.Root, imgDir, meta.TOC))
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
	BuildDrafts     bool
	BuildUnreleased bool
	Publish         bool // write pages as folder/index.html and link by folder
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
	base := filepath.ToSlash(strings.TrimSuffix(rel, filepath.Ext(rel)))
	if !publish {
		return base + ".html", base + ".html", nil
	}

	entries, err := os.ReadDir(filepath.Dir(src))
	if err != nil {
		return "", "", fmt.Errorf("failed to read %s: %w", filepath.Dir(src), err)
	}
	pages := 0
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && (ext == ".md" || ext == ".html") {
			pages++
		}
	}
	if pages == 1 {
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
	return (draft && !o.BuildDrafts) || (unreleased && !o.BuildUnreleased)
}

// folderStatus reports whether any markdown file directly in dir is a draft
// or unreleased.
func folderStatus(dir string) (draft, unreleased bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, false, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		mdBytes, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return false, false, fmt.Errorf("failed to read %s: %w", e.Name(), err)
		}
		meta, _ := parseFrontMatter(mdBytes)
		draft = draft || meta.Draft
		unreleased = unreleased || meta.Unreleased()
	}
	return draft, unreleased, nil
}

// buildContent writes each .md file in dir (and its subfolders) as a page in
// the matching build subfolder, copying same-folder css/js into the matching
// static subfolder and linking them, after shared, in the page's head. A
// folder whose markdown is a draft or unreleased (and not allowed by opts)
// has none of its own files built, as does a listed folder (one whose parent
// has a template page) with no published date when publishing. siblings is
// the parent folder's template page listing (if any), used to link each
// markdown page to its neighbours.
func buildContent(dir *file_access.ContentDir, root string, tmpl *template.Template, layout Page, buildDir string, shared []string, opts Options, siblings []SubPage, listed bool) error {
	rel, err := filepath.Rel(root, dir.Path)
	if err != nil {
		return err
	}
	var listing []SubPage
	if hasListing(dir) {
		if listing, err = subPages(dir, root, opts); err != nil {
			return err
		}
	}

	for _, sub := range dir.Dirs {
		if err := buildContent(sub, root, tmpl, layout, buildDir, shared, opts, listing, hasListing(dir)); err != nil {
			return err
		}
	}

	draft, unreleased, err := folderStatus(dir.Path)
	if err != nil {
		return err
	}
	skip := opts.skip(draft, unreleased)
	if listed && opts.Publish && !skip {
		undated, err := undatedPage(dir)
		if err != nil {
			return err
		}
		if undated != "" {
			fmt.Fprintf(os.Stderr, "error: %s has no published date, so it wasn't published\n", undated)
			skip = true
		}
	}
	if skip {
		// Remove the mirrored folders if nothing else was built in them.
		os.Remove(filepath.Join(buildDir, rel))
		os.Remove(filepath.Join(buildDir, "static", rel))
		return nil
	}

	pageAssets := append([]string{}, shared...)
	for _, f := range dir.Files {
		ext := strings.ToLower(filepath.Ext(f))
		isAsset := slices.Contains(assetExts, ext)
		if !isAsset && !slices.Contains(ImageExts, ext) {
			continue
		}
		name := filepath.Join(rel, filepath.Base(f))
		assetBytes, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f, err)
		}
		if len(assetBytes) == 0 {
			continue
		}
		if err := os.WriteFile(filepath.Join(buildDir, "static", name), assetBytes, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
		if isAsset {
			pageAssets = append(pageAssets, "static/"+filepath.ToSlash(name))
		}
	}

	for _, f := range dir.Files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".md" && ext != ".html" {
			continue
		}
		file, link, err := PagePath(f, root, opts.Publish)
		if err != nil {
			return err
		}
		out := filepath.Join(buildDir, file)
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(out), err)
		}
		page := layout
		page.Root = rootFor(file)
		if ext == ".md" {
			page.Previous, page.Next = adjacent(siblings, link)
			err = writeMarkdownPage(tmpl, page, f, out, rel, pageAssets)
		} else {
			err = writeTemplatePage(tmpl, page, listing, f, out, pageAssets)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Build generates the site from srcDir into outDir.
func Build(srcDir, outDir string, opts Options) error {
	if outDir == "" {
		outDir = "."
	}
	if info, err := os.Stat(outDir); err != nil || !info.IsDir() {
		return fmt.Errorf("invalid output directory: %s", outDir)
	}

	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(filepath.Join(srcDir, "index.html")); err != nil {
		return fmt.Errorf("missing index.html in %s", srcDir)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "index.md")); err != nil {
		return fmt.Errorf("missing index.md in %s", srcDir)
	}

	content, err := file_access.CrawlContent(filepath.Join(srcDir, "content"))
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFiles(
		filepath.Join(srcDir, "index.html"),
		filepath.Join(srcDir, "structure", "header", "header.html"),
		filepath.Join(srcDir, "structure", "footer", "footer.html"),
	)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	replacements := []Replacement{
		{ID: "content", Path: "./index.md"},
		{ID: "header", Path: "./structure/header/header.html"},
		{ID: "footer", Path: "./structure/footer/footer.html"},
	}

	prefs, err := readPreferences(filepath.Join(srcDir, "preferences.yaml"))
	if err != nil {
		return err
	}

	var navLinks []NavLink
	for _, name := range prefs.navLinkNames() {
		srcs, _ := filepath.Glob(filepath.Join(content.Path, name, name+".*"))
		srcs = slices.DeleteFunc(srcs, func(s string) bool {
			ext := strings.ToLower(filepath.Ext(s))
			return ext != ".md" && ext != ".html"
		})
		if len(srcs) == 0 {
			return fmt.Errorf("no page found for nav link %q", name)
		}
		src := srcs[0]
		draft, unreleased, err := folderStatus(filepath.Dir(src))
		if err != nil {
			return err
		}
		if opts.skip(draft, unreleased) {
			continue
		}
		_, link, err := PagePath(src, content.Path, opts.Publish)
		if err != nil {
			return err
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
			Logo:     prefs.Logo,
			Favicon:  prefs.Favicon,
			NavLinks: navLinks,
		},
		Root: rootFor("index.html"),
	}

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
	staticDir := filepath.Join(srcDir, "static")
	if _, err := os.Stat(staticDir); err == nil {
		if err := os.CopyFS(filepath.Join(buildDir, "static"), os.DirFS(staticDir)); err != nil {
			return fmt.Errorf("failed to copy %s: %w", staticDir, err)
		}
	}
	for _, dst := range []string{buildDir, filepath.Join(buildDir, "static")} {
		if err := file_access.MirrorDirs(content, content.Path, dst); err != nil {
			return err
		}
	}

	var shared []string
	for _, r := range replacements {
		base := strings.TrimSuffix(r.Path, filepath.Ext(r.Path))
		for _, ext := range assetExts {
			name := filepath.Base(base) + ext
			assetBytes, err := os.ReadFile(filepath.Join(srcDir, base+ext))
			if os.IsNotExist(err) || (err == nil && len(assetBytes) == 0) {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", name, err)
			}
			if ext == ".css" {
				if assetBytes, err = fillCSS(assetBytes, name, prefs); err != nil {
					return err
				}
			}
			if err := os.WriteFile(filepath.Join(buildDir, "static", name), assetBytes, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", name, err)
			}
			shared = append(shared, "static/"+name)
		}
	}

	if err := writeMarkdownPage(tmpl, layout, filepath.Join(srcDir, "index.md"), filepath.Join(buildDir, "index.html"), "", shared); err != nil {
		return err
	}

	return buildContent(content, content.Path, tmpl, layout, buildDir, shared, opts, nil, false)
}
