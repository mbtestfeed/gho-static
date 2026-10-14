package gho_static

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gho-static/utils/file_access"
)

// SubPage links to a markdown page in a subfolder of a template page's folder.
type SubPage struct {
	Title     string
	Link      string // relative to the build folder
	Published  string // yyyy/mm/dd, or empty if unset
	Draft      bool
	Unreleased bool // published date is still in the future
	published  time.Time
}

// pageLink builds a link to the markdown page at mdPath (under the content
// folder root), or returns nil if opts doesn't allow its folder to be built.
func pageLink(mdPath, root string, opts Options) (*SubPage, error) {
	draft, unreleased, err := folderStatus(filepath.Dir(mdPath))
	if err != nil {
		return nil, err
	}
	if opts.skip(draft, unreleased) {
		return nil, nil
	}
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", mdPath, err)
	}
	meta, _ := parseFrontMatter(mdBytes)
	_, link, err := PagePath(mdPath, root, opts.Publish)
	if err != nil {
		return nil, err
	}
	if meta.Title == "" {
		// Untitled pages are named after their folder, e.g. 20261012_my_post
		// -> My post, or Untitled if the folder is only a date.
		title := strings.ReplaceAll(PostName(filepath.Base(filepath.Dir(mdPath))), "_", " ")
		if title == "" {
			title = "untitled"
		}
		meta.Title = strings.ToUpper(title[:1]) + title[1:]
	}
	published, ok := parseDate(meta.Published)
	if !ok && opts.Publish {
		return nil, nil
	}
	return &SubPage{
		Title:      meta.Title,
		Link:       link,
		Published:  formatDate(meta.Published),
		Draft:      draft,
		Unreleased: meta.Unreleased(),
		published:  published,
	}, nil
}

// subPages collects links to the markdown pages in dir's subfolders, skipping
// folders opts doesn't allow, ordered most recently published first.
func subPages(dir *file_access.ContentDir, root string, opts Options) ([]SubPage, error) {
	var pages []SubPage
	for _, sub := range dir.Dirs {
		for _, f := range sub.Files {
			if !strings.EqualFold(filepath.Ext(f), ".md") {
				continue
			}
			link, err := pageLink(f, root, opts)
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
func undatedPage(dir *file_access.ContentDir) (string, error) {
	for _, f := range dir.Files {
		if !strings.EqualFold(filepath.Ext(f), ".md") {
			continue
		}
		mdBytes, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", f, err)
		}
		meta, _ := parseFrontMatter(mdBytes)
		if _, ok := parseDate(meta.Published); !ok {
			return f, nil
		}
	}
	return "", nil
}

// UndatedPages returns the markdown files in srcDir's listed content folders
// (those whose parent has a template page) without a valid published date.
func UndatedPages(srcDir string) ([]string, error) {
	content, err := file_access.CrawlContent(filepath.Join(srcDir, "content"))
	if err != nil {
		return nil, err
	}
	var pages []string
	var walk func(dir *file_access.ContentDir, listed bool) error
	walk = func(dir *file_access.ContentDir, listed bool) error {
		if listed {
			page, err := undatedPage(dir)
			if err != nil {
				return err
			}
			if page != "" {
				pages = append(pages, page)
			}
		}
		for _, sub := range dir.Dirs {
			if err := walk(sub, hasListing(dir)); err != nil {
				return err
			}
		}
		return nil
	}
	return pages, walk(content, false)
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

// writeTemplatePage executes the content template at tmplPath (with
// SubPages set to subs) into page's content, titles the page after the
// template's file name, and writes it with writePage.
func writeTemplatePage(tmpl *template.Template, page Page, subs []SubPage, tmplPath, out string, assets []string) error {
	page.SubPages = subs

	name := filepath.Base(tmplPath)
	title := strings.TrimSuffix(name, filepath.Ext(name))
	page.Meta.Title = strings.ToUpper(title[:1]) + title[1:]

	contentTmpl, err := template.ParseFiles(tmplPath)
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
