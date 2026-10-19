package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	gitclient "gho-static/utils/git-client"
	"gho-static/utils/output"
)

// The Gho-Static repo, whose themes folder holds the themes that can be
// downloaded, over SSH and HTTPS.
const (
	themesRepoSSH   = "git@github.com:mbtestfeed/gho-static.git"
	themesRepoHTTPS = "https://github.com/mbtestfeed/gho-static.git"
)

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
		names, err = gitclient.RemoteDirs(repo, "themes")
		return err
	})
	return names, err
}

// GetThemes downloads the named themes, or every theme if names is empty,
// from the Gho-Static repo into site's themes folder, replacing any already
// there. It returns the names of the themes it got. It's fetched over SSH if
// GitHub authentication is set up, otherwise anonymously over HTTPS.
func GetThemes(site string, names []string, out output.Output) ([]string, error) {
	if err := gitclient.Installed(); err != nil {
		return nil, err
	}
	out.Warnf("getting themes replaces their folders in %s, so any changes to those theme files will be lost. Keep site-specific CSS in your project files instead", filepath.Join(site, "themes"))
	out.Warnf("theme field names should stay reasonably stable, but they may change between versions in ways that break a theme's compatibility")
	repo := themesRepo(out)

	tmp, err := os.MkdirTemp("", "gho-static-themes-")
	if err != nil {
		return nil, fmt.Errorf("failed to create a temporary folder: %w", err)
	}
	defer os.RemoveAll(tmp)
	paths := []string{"themes"}
	if len(names) > 0 {
		paths = nil
		for _, name := range names {
			paths = append(paths, "themes/"+name)
		}
	}
	if err := out.Step("Downloading themes from "+repo+"...", func() error {
		return gitclient.SparseClone(repo, filepath.Join(tmp, "repo"), paths...)
	}); err != nil {
		return nil, err
	}

	src := filepath.Join(tmp, "repo", "themes")
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
		dst := filepath.Join(site, "themes", name)
		if err := os.RemoveAll(dst); err != nil {
			return nil, fmt.Errorf("failed to remove %s: %w", dst, err)
		}
		if err := os.CopyFS(dst, os.DirFS(filepath.Join(src, name))); err != nil {
			return nil, fmt.Errorf("failed to copy the %s theme into %s: %w", name, dst, err)
		}
	}
	return got, nil
}
