// Package github_pages deploys a site's build folder to GitHub Pages, as a
// git repo of its own pushed to the Pages repo.
package github_pages

import (
	"fmt"
	"os"
	"strings"
	"time"

	gitclient "gho-static/utils/git-client"
)

// RepoURL returns the SSH URL of user's <user>.github.io repo.
func RepoURL(user string) string {
	return "git@github.com:" + user + "/" + user + ".github.io.git"
}

// RepoLinks returns the links to create repo (an SSH or HTTPS GitHub repo
// URL) and to its GitHub Pages settings.
func RepoLinks(repo string) (create, settings string) {
	path := strings.TrimSuffix(repo, ".git")
	for _, prefix := range []string{"git@github.com:", "https://github.com/"} {
		path = strings.TrimPrefix(path, prefix)
	}
	owner, name, _ := strings.Cut(path, "/")
	return "https://github.com/new?owner=" + owner + "&name=" + name, "https://github.com/" + path + "/settings/pages"
}

// CheckRepo returns an error explaining how to create repo and turn on
// GitHub Pages for it, if it can't be reached.
func CheckRepo(repo string) error {
	if err := gitclient.Installed(); err != nil {
		return err
	}
	if err := gitclient.RemoteExists(repo); err != nil {
		create, settings := RepoLinks(repo)
		return fmt.Errorf("the GitHub repo %s doesn't exist or can't be reached. Create it (public, and empty) at:\n    %s\nthen turn on GitHub Pages for it, choosing \"Deploy from a branch\" with the root (/) folder, at:\n    %s\n\n%w", repo, create, settings, err)
	}
	return nil
}

// IsSetUp reports whether buildDir is set up for GitHub Pages.
func IsSetUp(buildDir string) bool {
	return gitclient.IsRepo(buildDir)
}

// Repo returns the repo buildDir pushes to, or "" if none is set.
func Repo(buildDir string) string {
	return gitclient.Remote(buildDir)
}

// Check returns an error explaining what's missing to deploy to repo.
func Check(repo string) error {
	if err := gitclient.Installed(); err != nil {
		return fmt.Errorf("can't deploy to GitHub Pages: %w", err)
	}
	if repo == "" {
		return fmt.Errorf("can't deploy to GitHub Pages: no GitHub repo is set to push to")
	}
	return nil
}

// Setup makes buildDir a git repo, with repo (if given) as its remote. If
// buildDir isn't a repo yet, it's replaced with a clone of repo so the
// repo's history is kept; the site should be built into it afterwards.
func Setup(buildDir, repo string) error {
	if err := gitclient.Installed(); err != nil {
		return err
	}
	if repo == "" {
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", buildDir, err)
		}
		return gitclient.Init(buildDir)
	}
	if err := CheckRepo(repo); err != nil {
		return err
	}
	if gitclient.IsRepo(buildDir) {
		if err := gitclient.SetRemote(buildDir, repo); err != nil {
			return err
		}
		// The build is regenerated each time, so its own history can give
		// way to the repo's, keeping pushes from being rejected.
		if branch := gitclient.RemoteBranch(buildDir); branch != "" {
			return gitclient.ResetOnto(buildDir, branch)
		}
		return nil
	}
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to remove %s: %w", buildDir, err)
	}
	return gitclient.Clone(repo, buildDir)
}

// Push commits everything in buildDir and pushes it to its repo.
func Push(buildDir string) error {
	if err := gitclient.CommitAll(buildDir, "Deploy "+time.Now().Format("2006-01-02 15:04")); err != nil {
		return err
	}
	return gitclient.Push(buildDir)
}
