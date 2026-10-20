package generator

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gho-static/gho_static"
	"gho-static/utils/file_access"
	gitclient "gho-static/utils/git-client"
	"gho-static/utils/output"
)

// The Gho-Static repo, whose Gho-Static themes folder (laid out as in a
// site) holds the themes that can be downloaded, over SSH and HTTPS.
const (
	themesRepoSSH   = "git@github.com:mbtestfeed/gho-static.git"
	themesRepoHTTPS = "https://github.com/mbtestfeed/gho-static.git"
)

// themeFileWarning starts the warning at the top of each of Gho-Static's own
// theme files, in a css or yaml comment that ends at the first blank line.
const themeFileWarning = "GHO-STATIC THEME FILE"

// withoutWarning returns content without the warning Gho-Static's own theme
// files start with, if it has one.
func withoutWarning(content string) string {
	for _, prefix := range []string{"/* ", "# "} {
		if !strings.HasPrefix(content, prefix+themeFileWarning) {
			continue
		}
		for _, blank := range []string{"\r\n\r\n", "\n\n"} {
			if _, rest, ok := strings.Cut(content, blank); ok {
				return rest
			}
		}
	}
	return content
}

// GenerateTheme copies the theme in the folder src (slash separated,
// relative to the site folder site) into a new custom theme called name,
// without the warnings Gho-Static's own theme files start with, and with a
// summary saying what it's based on. If the site has a themes showcase, a
// post for the theme is added to it, published at now, by author if set,
// with the demo pictures in images.
func GenerateTheme(site, name, src string, images fs.FS, author string, now time.Time) error {
	dst := filepath.Join(site, filepath.FromSlash(gho_static.CustomThemesDir), name)
	if err := file_access.NewFolder(dst); err != nil {
		return err
	}
	files := map[string]string{}
	srcFS, err := fs.Sub(os.DirFS(site), src)
	if err == nil {
		err = fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := fs.ReadFile(srcFS, p)
			files[p] = withoutWarning(string(b))
			return err
		})
	}
	if err != nil {
		return fmt.Errorf("failed to read the theme in %s: %w", src, err)
	}
	if err := file_access.WriteFiles(dst, files); err != nil {
		return err
	}
	if err := gho_static.SetValue(filepath.Join(dst, "theme_preferences.yaml"), "summary", "A custom theme, based on "+path.Base(src)+"."); err != nil {
		return err
	}

	if info, err := os.Stat(filepath.Join(site, "content", gho_static.ShowcaseDir)); err != nil || !info.IsDir() {
		return nil
	}
	pics := map[string]string{}
	for _, pic := range demoPics {
		b, err := fs.ReadFile(images, "demo_pics/"+strings.ToLower(pic))
		if err != nil {
			return fmt.Errorf("failed to read demo picture %s: %w", pic, err)
		}
		pics[strings.ToLower(pic)] = string(b)
	}
	post := map[string]string{}
	addShowcasePost(post, pics, name, "A custom theme, based on "+path.Base(src)+".", pageMeta{author: author}, now)
	return file_access.WriteFiles(site, post)
}

// themesRepo returns the Gho-Static repo's SSH URL if GitHub authentication
// is set up, otherwise its HTTPS URL, warning on out that it's used
// anonymously.
func themesRepo(out output.Output) string {
	if gitclient.Reachable(themesRepoSSH) {
		return themesRepoSSH
	}
	out.Warnf("you're not authenticated with GitHub over SSH, so themes will be fetched anonymously over HTTPS")
	return themesRepoHTTPS
}

// ListThemes returns the names of the themes in the Gho-Static repo.
func ListThemes(out output.Output) ([]string, error) {
	if err := gitclient.Installed(); err != nil {
		return nil, err
	}
	repo := themesRepo(out)
	var names []string
	err := out.Step("Fetching the list of themes from "+repo+"...", func() error {
		var err error
		names, err = gitclient.RemoteDirs(repo, gho_static.GhoStaticThemesDir)
		return err
	})
	return names, err
}

// GetThemes downloads the named themes, or every theme if names is empty,
// from the Gho-Static repo into site's Gho-Static themes folder, replacing
// any already there. Custom themes are left alone. It returns the names of
// the themes it got. It's fetched over SSH if GitHub authentication is set
// up, otherwise anonymously over HTTPS.
func GetThemes(site string, names []string, out output.Output) ([]string, error) {
	if err := gitclient.Installed(); err != nil {
		return nil, err
	}
	themesDir := filepath.Join(site, filepath.FromSlash(gho_static.GhoStaticThemesDir))
	out.Warnf("getting themes replaces their folders in %s, so any changes to those theme files will be lost. Keep site-specific CSS in your project files, and your own themes in %s, instead", themesDir, filepath.Join(site, filepath.FromSlash(gho_static.CustomThemesDir)))
	out.Warnf("theme field names should stay reasonably stable, but they may change between versions in ways that break a theme's compatibility")
	repo := themesRepo(out)

	tmp, err := os.MkdirTemp("", "gho-static-themes-")
	if err != nil {
		return nil, fmt.Errorf("failed to create a temporary folder: %w", err)
	}
	defer os.RemoveAll(tmp)
	paths := []string{gho_static.GhoStaticThemesDir}
	if len(names) > 0 {
		paths = nil
		for _, name := range names {
			paths = append(paths, gho_static.GhoStaticThemesDir+"/"+name)
		}
	}
	if err := out.Step("Downloading themes from "+repo+"...", func() error {
		return gitclient.SparseClone(repo, filepath.Join(tmp, "repo"), paths...)
	}); err != nil {
		return nil, err
	}

	src := filepath.Join(tmp, "repo", filepath.FromSlash(gho_static.GhoStaticThemesDir))
	entries, err := os.ReadDir(src)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read the downloaded themes: %w", err)
	}
	var got []string
	for _, e := range entries {
		if e.IsDir() && (len(names) == 0 || slices.Contains(names, e.Name())) {
			got = append(got, e.Name())
		}
	}
	for _, name := range names {
		if !slices.Contains(got, name) {
			return nil, fmt.Errorf("there's no theme called %q in %s", name, themesRepoHTTPS)
		}
	}
	for _, name := range got {
		dst := filepath.Join(themesDir, name)
		if err := os.RemoveAll(dst); err != nil {
			return nil, fmt.Errorf("failed to remove %s: %w", dst, err)
		}
		if err := os.CopyFS(dst, os.DirFS(filepath.Join(src, name))); err != nil {
			return nil, fmt.Errorf("failed to copy the %s theme into %s: %w", name, dst, err)
		}
	}
	return got, nil
}
