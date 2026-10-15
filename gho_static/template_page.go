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
	"unicode"

	"gho-static/utils/file_access"
)

// SubPage links to a markdown page in a subfolder of a template page's folder.
type SubPage struct {
	Title     string
	Link      string // relative to the build folder
	Published  string // yyyy/mm/dd, or empty if unset
	Draft      bool
	Unreleased bool // published date is still in the future
	Summary    string // front matter summary, truncated to 200 characters
	Tags       []Tag  // the page's tags, without their Pages
	published  time.Time
}

// summaryLength is the most characters of a page's summary shown in links
// to it, including the "..." added when it's cut short.
const summaryLength = 200

// truncate returns s cut to summaryLength characters, ending in "..." if
// anything was cut.
func truncate(s string) string {
	if r := []rune(s); len(r) > summaryLength {
		return string(r[:summaryLength-3]) + "..."
	}
	return s
}

// pageLink builds a link to the markdown page at mdPath (under the content
// folder root), or returns nil if opts doesn't allow its folder to be built.
// listed is whether the page's folder is listed by its parent's template
// page, so isn't published without a date.
func (s source) pageLink(mdPath, root string, opts Options, listed bool) (*SubPage, error) {
	draft, unreleased := s.folderStatus(path.Dir(mdPath))
	if opts.skip(draft, unreleased) {
		return nil, nil
	}
	meta := s.md[mdPath].Meta
	_, link, err := s.pagePath(mdPath, root, opts.Publish)
	if err != nil {
		return nil, err
	}
	if meta.Title == "" {
		// Untitled pages are named after their folder, e.g. 20261012_my_post
		// -> My post, or Untitled if the folder is only a date.
		title := strings.ReplaceAll(PostName(path.Base(path.Dir(mdPath))), "_", " ")
		if title == "" {
			title = "untitled"
		}
		meta.Title = strings.ToUpper(title[:1]) + title[1:]
	}
	published, ok := parseDate(meta.Published)
	if !ok && opts.Publish && listed {
		return nil, nil
	}
	var tags []Tag
	for _, name := range meta.Tags {
		tags = append(tags, s.newTag(name, root, opts.Publish))
	}
	return &SubPage{
		Title:      meta.Title,
		Link:       link,
		Published:  formatDate(meta.Published),
		Draft:      draft,
		Unreleased: meta.Unreleased,
		Summary:    truncate(meta.Summary),
		Tags:       tags,
		published:  published,
	}, nil
}

// subPages collects links to the markdown pages in dir's subfolders, skipping
// folders opts doesn't allow, ordered most recently published first.
func (s source) subPages(dir *file_access.ContentDir, root string, opts Options) ([]SubPage, error) {
	var pages []SubPage
	for _, sub := range dir.Dirs {
		for _, f := range sub.Files {
			if !strings.EqualFold(filepath.Ext(f), ".md") {
				continue
			}
			link, err := s.pageLink(f, root, opts, true)
			if err != nil {
				return nil, err
			}
			if link != nil {
				pages = append(pages, *link)
			}
		}
	}
	slices.SortFunc(pages, func(a, b SubPage) int { return b.published.Compare(a.published) })
	return pages, nil
}

// recentPages returns the n most recently published pages listed by any
// template page in content (the content folder root), skipping folders opts
// doesn't allow.
func (s source) recentPages(content *file_access.ContentDir, opts Options, n int) ([]SubPage, error) {
	var pages []SubPage
	err := walk(content, false, func(dir *file_access.ContentDir, _ bool) error {
		if hasListing(dir) {
			subs, err := s.subPages(dir, content.Path, opts)
			if err != nil {
				return err
			}
			pages = append(pages, subs...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(pages, func(a, b SubPage) int { return b.published.Compare(a.published) })
	return pages[:min(n, len(pages))], nil
}

// Tag is a front matter tag, with links to the pages that have it.
type Tag struct {
	Name  string
	Link  string    // to the tag's page, relative to the build folder, or empty if the site has no tags page
	Pages []SubPage // ordered most recently published first
	file  string    // the tag's page, relative to the build folder
}

// tagsDir is the content folder whose tagsDir.html template page lists the
// site's tags, and is rendered again for each tag's own page in a subfolder.
const tagsDir = "tags"

// collectTags gathers the tags of the markdown pages in content (the content
// folder root) that opts allows to be built, ordered by name.
func (s source) collectTags(content *file_access.ContentDir, opts Options) ([]Tag, error) {
	byKey := map[string]*Tag{}
	err := walk(content, false, func(dir *file_access.ContentDir, listed bool) error {
		for _, f := range dir.Files {
			if !strings.EqualFold(filepath.Ext(f), ".md") {
				continue
			}
			meta := s.md[f].Meta
			if len(meta.Tags) == 0 {
				continue
			}
			link, err := s.pageLink(f, content.Path, opts, listed)
			if err != nil {
				return err
			}
			if link == nil {
				continue
			}
			for _, name := range meta.Tags {
				// Tags differing only in case or punctuation share a page.
				key := tagSlug(name)
				t := byKey[key]
				if t == nil {
					tag := s.newTag(name, content.Path, opts.Publish)
					t = &tag
					byKey[key] = t
				}
				t.Pages = append(t.Pages, *link)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var tags []Tag
	for _, t := range byKey {
		slices.SortFunc(t.Pages, func(a, b SubPage) int { return b.published.Compare(a.published) })
		tags = append(tags, *t)
	}
	slices.SortFunc(tags, func(a, b Tag) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return tags, nil
}

// newTag returns the tag name, linked to its page if the content folder
// root has a tags template page.
func (s source) newTag(name, root string, publish bool) Tag {
	key := tagSlug(name)
	t := Tag{Name: name, file: path.Join(tagsDir, key, "index.html")}
	if fileExists(s, path.Join(root, tagsDir, tagsDir+".html")) {
		t.Link = t.file
		if publish {
			t.Link = path.Join(tagsDir, key) + "/"
		}
	}
	return t
}

// tagSlug returns name lowercased, with anything other than letters, digits,
// - and _ replaced by -, for use as a folder name.
func tagSlug(name string) string {
	slug := strings.Trim(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return unicode.ToLower(r)
		}
		return '-'
	}, name), "-")
	if slug == "" {
		return "tag"
	}
	return slug
}

// PostName returns folder without its leading yyyymmdd date and the
// underscore after it, if it has them.
func PostName(folder string) string {
	if len(folder) >= 8 && (len(folder) == 8 || folder[8] == '_') {
		if _, err := time.Parse("20060102", folder[:8]); err == nil {
			return strings.TrimPrefix(folder[8:], "_")
		}
	}
	return folder
}

// hasListing reports whether dir has a template page listing its subfolders.
func hasListing(dir *file_access.ContentDir) bool {
	return slices.ContainsFunc(dir.Files, func(f string) bool { return strings.EqualFold(filepath.Ext(f), ".html") })
}

// undatedPage returns the first markdown file directly in dir without a
// valid published date, or "" if there is none.
func (s source) undatedPage(dir *file_access.ContentDir) string {
	for _, f := range dir.Files {
		if !strings.EqualFold(filepath.Ext(f), ".md") {
			continue
		}
		if _, ok := parseDate(s.md[f].Meta.Published); !ok {
			return f
		}
	}
	return ""
}

// readContent crawls and reads the content folder of the site at srcDir.
func readContent(srcDir string) (source, *file_access.ContentDir, error) {
	fsys := os.DirFS(srcDir)
	content, err := file_access.CrawlContent(fsys, "content")
	if err != nil {
		return source{}, nil, err
	}
	s, err := readSource(fsys, content, time.Now())
	return s, content, err
}

// UndatedPages returns the markdown files in srcDir's listed content folders
// (those whose parent has a template page) without a valid published date.
func UndatedPages(srcDir string) ([]string, error) {
	s, content, err := readContent(srcDir)
	if err != nil {
		return nil, err
	}
	var pages []string
	walk(content, false, func(dir *file_access.ContentDir, listed bool) error {
		if listed {
			if page := s.undatedPage(dir); page != "" {
				pages = append(pages, filepath.Join(srcDir, filepath.FromSlash(page)))
			}
		}
		return nil
	})
	return pages, nil
}

// DraftPages returns the markdown files in srcDir's content folder marked
// as drafts, ordered by path.
func DraftPages(srcDir string) ([]string, error) {
	s, _, err := readContent(srcDir)
	if err != nil {
		return nil, err
	}
	var pages []string
	for p, f := range s.md {
		if f.Meta.Draft {
			pages = append(pages, filepath.Join(srcDir, filepath.FromSlash(p)))
		}
	}
	slices.Sort(pages)
	return pages, nil
}

// adjacent returns the pages either side of link in pages (ordered most
// recently published first): the older one as previous, the newer as next.
func adjacent(pages []SubPage, link string) (previous, next *SubPage) {
	i := slices.IndexFunc(pages, func(p SubPage) bool { return p.Link == link })
	if i == -1 {
		return nil, nil
	}
	if i+1 < len(pages) {
		previous = &pages[i+1]
	}
	if i > 0 {
		next = &pages[i-1]
	}
	return previous, next
}

// writeTemplatePage executes the content template at tmplPath into page's
// content, titles the page after the template's file name unless it already
// has a title, and writes it with writePage.
func (s source) writeTemplatePage(tmpl *template.Template, page Page, tmplPath, out string, assets []string) error {
	name := filepath.Base(tmplPath)
	if page.Meta.Title == "" {
		title := strings.TrimSuffix(name, filepath.Ext(name))
		page.Meta.Title = strings.ToUpper(title[:1]) + title[1:]
	}
	if page.Tag != nil {
		page.Meta.Title = page.Tag.Name
	}

	// Read, rather than template.ParseFS, as that treats the path as a glob.
	tmplBytes, err := fs.ReadFile(s, tmplPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", tmplPath, err)
	}
	contentTmpl, err := template.New(name).Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", tmplPath, err)
	}
	content, err := render(contentTmpl, name, page)
	if err != nil {
		return err
	}
	page.Content = toDiv([]byte(content))
	return writePage(tmpl, page, out, assets)
}
