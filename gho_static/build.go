package gho_static

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gho-static/utils"
	"gho-static/utils/file_access"
)

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
	ls, err := loadSite(fsys, opts)
	if err != nil {
		return err
	}
	layout, err := ls.layout(version)
	if err != nil {
		return err
	}
	return ls.write(outDir, layout)
}

// loadedSite is a site's source folder, read and parsed ready to build with
// opts.
type loadedSite struct {
	source
	opts    Options
	content *file_access.ContentDir
	prefs   Preferences
	tmpl    *template.Template // the layout and structure templates
	index   mdFile             // the home page
}

// loadSite reads and parses the site in the source folder fsys to build
// with opts, marking pages published after opts.Now (or the current time)
// as unreleased.
func loadSite(fsys fs.FS, opts Options) (loadedSite, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	if !file_access.FileExists(fsys, "index.html") {
		return loadedSite{}, fmt.Errorf("missing index.html in the site folder")
	}
	if !file_access.FileExists(fsys, "index.md") {
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

	prefs, err := readOver(fsys, "settings.yaml", DefaultPreferences)
	if err != nil {
		return loadedSite{}, err
	}
	if !hasTheme(fsys, prefs.Theme) {
		return loadedSite{}, fmt.Errorf("theme %q in settings.yaml has no folder in themes/", prefs.Theme)
	}

	templates := []string{
		"index.html",
		"structure/header/header.html",
		"structure/footer/footer.html",
	}
	// The sidebars are optional, so sites made before them still build, and
	// can be turned off in preferences.
	for _, sidebar := range []struct {
		name string
		show bool
	}{
		{"intro-sidebar", prefs.ShowIntroSidebar},
		{"recent-activity", len(prefs.RecentSidebarCollections) > 0},
	} {
		if tmplPath := path.Join("structure", sidebar.name, sidebar.name+".html"); sidebar.show && file_access.FileExists(fsys, tmplPath) {
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
	return loadedSite{source: s, opts: opts, content: content, prefs: prefs, tmpl: tmpl, index: mdFile{Meta: meta, Body: body}}, nil
}

// layout returns the page data shared by every page of ls, marked with
// version: its nav links, tags and recent pages, leaving out any its
// options don't allow.
func (ls loadedSite) layout(version string) (Page, error) {
	content, prefs, opts := ls.content, ls.prefs, ls.opts
	var navLinks []NavLink
	for _, name := range prefs.navLinkNames() {
		srcs, _ := fs.Glob(ls, path.Join(content.Path, name, name+".*"))
		srcs = slices.DeleteFunc(srcs, func(s string) bool { return !utils.HasExt(s, ".md", ".html") })
		if len(srcs) == 0 {
			opts.Output.Warnf("no page found for nav link %q, so it was left out of the menu", name)
			continue
		}
		src := srcs[0]
		draft, unreleased := ls.folderStatus(path.Dir(src))
		if opts.skip(draft, unreleased) || ls.leftOut(path.Dir(src), content.Path, opts) {
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
			Theme:    prefs.Theme,
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
	if layout.Site.RecentActivity, err = ls.recentPages(content, prefs.RecentSidebarCollections, opts, 6); err != nil {
		return Page{}, err
	}
	return layout, nil
}

// write clears outDir/build, keeping any git repo, and builds ls into it
// with layout.
func (ls loadedSite) write(outDir string, layout Page) error {
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
	if file_access.FileExists(ls, "static") {
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

	// The site's theme, and any other downloaded theme a page uses.
	themes := []string{ls.prefs.Theme}
	for _, f := range ls.md {
		if t := f.Meta.Theme; t != "" && !slices.Contains(themes, t) && hasTheme(ls, t) {
			themes = append(themes, t)
		}
	}
	shared := map[string][]string{} // each theme's shared files, relative to the build folder
	for _, theme := range themes {
		files, err := ls.planShared(theme)
		if err != nil {
			return err
		}
		for _, f := range files {
			out := filepath.Join(buildDir, "static", filepath.FromSlash(f.name))
			if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", filepath.Dir(out), err)
			}
			if err := os.WriteFile(out, f.data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", f.name, err)
			}
			shared[theme] = append(shared[theme], "static/"+f.name)
		}
	}

	if err := writeMarkdownPage(ls.tmpl, layout, ls.index, filepath.Join(buildDir, "index.html"), "", shared[ls.prefs.Theme]); err != nil {
		return err
	}

	plan, err := ls.planContent(layout, shared)
	if err != nil {
		return err
	}
	return ls.writeContent(plan, buildDir)
}

// structureParts are the structure templates' folders, each named after its
// template.
var structureParts = []string{"header", "footer", "intro-sidebar", "recent-activity"}

// sharedFiles returns the paths of the css and js files linked from every
// page styled with theme, in order: the theme's theme.css and the layout's
// index.js, then each structure part's css (at the same path in the theme)
// and js (beside its template).
func sharedFiles(theme string) []string {
	files := []string{path.Join(themeDir(theme), "theme.css"), "index.js"}
	for _, p := range structureParts {
		base := path.Join("structure", p, p)
		files = append(files, path.Join(themeDir(theme), base+".css"), base+".js")
	}
	return files
}

// themeBuildDir is the folder, within the build's static folder, each
// theme's css is copied into.
const themeBuildDir = "_theme"

// planShared returns ls's non-empty shared css and js files for pages
// styled with theme (see sharedFiles), to copy into the build's static
// folder, with the theme's preference values filled into the css. The css
// goes in a folder for the theme, so several themes can be built at once.
func (ls loadedSite) planShared(theme string) ([]staticFile, error) {
	prefs, err := readOver(ls, path.Join(themeDir(theme), "theme_preferences.yaml"), DefaultThemePreferences)
	if err != nil {
		return nil, err
	}
	var files []staticFile
	for _, f := range sharedFiles(theme) {
		name := path.Base(f)
		data, err := fs.ReadFile(ls, f)
		if os.IsNotExist(err) || (err == nil && len(data) == 0) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", f, err)
		}
		if utils.HasExt(f, ".css") {
			if data, err = fillCSS(data, name, prefs); err != nil {
				return nil, err
			}
			name = path.Join(themeBuildDir, theme, name)
		}
		files = append(files, staticFile{name: name, data: data})
	}
	return files, nil
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
// whose markdown is a draft or unreleased (and not allowed by ls's options)
// has none of its own files built, as does a listed folder (one whose parent
// has a template page) with no published date when publishing. Markdown
// pages are linked to their neighbours in their parent folder's template
// page listing. Pages are linked to their own theme's shared files, if
// their front matter sets one, otherwise the site theme's.
func (ls loadedSite) planContent(layout Page, shared map[string][]string) (contentPlan, error) {
	opts := ls.opts
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
			ls.warnUnpublished(dir)
		}
		skip = skip || ls.leftOut(dir.Path, root, opts)
		if listed && opts.Publish && !skip {
			if undated := ls.undatedPage(dir); undated != "" {
				opts.Output.Errorf("%s has no published date, so it wasn't published", undated)
				skip = true
			}
		}
		if skip {
			plan.skipped = append(plan.skipped, rel)
			return nil
		}

		var assets []string // the folder's own css and js
		for _, f := range dir.Files {
			isAsset := utils.HasExt(f, assetExts...)
			if !isAsset && !utils.HasExt(f, ImageExts...) {
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
			p := plannedPage{src: f, file: file, page: layout, imgDir: rel, assets: slices.Concat(shared[ls.prefs.Theme], assets)}
			p.page.Root = rootFor(file)
			if ext == ".md" {
				md := ls.md[f]
				p.markdown = &md
				if md.Meta.Theme != "" {
					p.assets = slices.Concat(shared[md.Meta.Theme], assets)
				}
				p.page.Previous, p.page.Next = adjacent(siblings, link)
				if listed && path.Dir(filepath.ToSlash(rel)) == galleryDir {
					p.page.Images = folderImages(dir.Files, root)
				}
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
					t := plannedPage{src: f, file: tag.file, page: layout, assets: p.assets}
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
