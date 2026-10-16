// Package github_pages deploys a site's build folder to GitHub Pages, as a
// git repo of its own pushed to the Pages repo.
package github_pages

import (
	"fmt"
	"os"
	"strings"
	"time"

	gitclient "gho-static/utils/git-client"
	"gho-static/utils/output"
)

// RepoURL returns the SSH URL of user's <user>.github.io repo.
func RepoURL(user string) string {
	return "git@github.com:" + user + "/" + user + ".github.io.git"
}

// repoPath returns the owner and name of repo, an SSH or HTTPS GitHub repo
// URL.
func repoPath(repo string) (owner, name string) {
	path := strings.TrimSuffix(repo, ".git")
	for _, prefix := range []string{"git@github.com:", "https://github.com/"} {
		path = strings.TrimPrefix(path, prefix)
	}
	owner, name, _ = strings.Cut(path, "/")
	return owner, name
}

// RepoLinks returns the links to create repo and to its GitHub Pages
// settings.
func RepoLinks(repo string) (create, settings string) {
	owner, name := repoPath(repo)
	return "https://github.com/new?owner=" + owner + "&name=" + name, "https://github.com/" + owner + "/" + name + "/settings/pages"
}

// PagesURL returns the address repo's GitHub Pages site is served at: the
// root for a <owner>.github.io repo, otherwise a folder named after it.
func PagesURL(repo string) string {
	owner, name := repoPath(repo)
	if strings.EqualFold(name, owner+".github.io") {
		return "https://" + name + "/"
	}
	return "https://" + owner + ".github.io/" + name + "/"
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

// Detect returns the GitHub username whose <username>.github.io repo
// buildDir pushes to (or "" if it pushes nowhere, or to another repo), and
// whether buildDir is set up for GitHub Pages.
func Detect(buildDir string) (username string, ok bool) {
	if !gitclient.IsRepo(buildDir) {
		return "", false
	}
	owner, name := repoPath(gitclient.Remote(buildDir))
	if !strings.EqualFold(name, owner+".github.io") {
		return "", true
	}
	return owner, true
}

// Deployer deploys a build folder to GitHub Pages, as a git repo of its own
// pushed to Username's <username>.github.io repo.
type Deployer struct {
	Username string // or "" if it's to be added later
	output.Output
}

// New returns a Deployer for username. If username is "", it asks with ask
// (unless nil) for it.
func New(username string, ask func(prompt string, hints []string, secret bool) string, out output.Output) Deployer {
	if username == "" && ask != nil {
		username = ask("GitHub username: ", []string{"the site is published from the <username>.github.io repo, or press enter to add it later"}, false)
	}
	return Deployer{Username: username, Output: out}
}

// repo returns the SSH URL of d's repo, or "" if its username isn't set.
func (d Deployer) repo() string {
	if d.Username == "" {
		return ""
	}
	return RepoURL(d.Username)
}

// Settings returns d's username, if set, as its "username" setting.
func (d Deployer) Settings() map[string]string {
	if d.Username == "" {
		return nil
	}
	return map[string]string{"username": d.Username}
}

// Check returns an error explaining what to do if d's repo is set but
// can't be reached.
func (d Deployer) Check() error {
	if d.repo() == "" {
		return nil
	}
	return CheckRepo(d.repo())
}

// CheckPush returns an error explaining what's missing to push to d's repo.
func (d Deployer) CheckPush() error {
	if err := gitclient.Installed(); err != nil {
		return fmt.Errorf("can't deploy to GitHub Pages: %w", err)
	}
	if d.repo() == "" {
		return fmt.Errorf("can't deploy to GitHub Pages: no GitHub username is set to push to")
	}
	return nil
}

// Setup makes buildDir a git repo, with d's repo (if set) as its remote. If
// buildDir isn't a repo yet, it's replaced with a clone of the repo so its
// history is kept; the site should be built into it afterwards.
func (d Deployer) Setup(buildDir string) error {
	if err := gitclient.Installed(); err != nil {
		return err
	}
	if d.repo() == "" {
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", buildDir, err)
		}
		return gitclient.Init(buildDir)
	}
	return d.Step("Linking to "+PagesURL(d.repo())+". One second...", func() error {
		if err := CheckRepo(d.repo()); err != nil {
			return err
		}
		if gitclient.IsRepo(buildDir) {
			if err := gitclient.SetRemote(buildDir, d.repo()); err != nil {
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
		return gitclient.Clone(d.repo(), buildDir)
	})
}

// Push commits everything in buildDir and pushes it to its repo.
func (d Deployer) Push(buildDir string) error {
	err := d.Step("Deploying to "+gitclient.Remote(buildDir)+". Shouldn't take long...", func() error {
		if err := gitclient.CommitAll(buildDir, "Deploy "+time.Now().Format("2006-01-02 15:04")); err != nil {
			return err
		}
		return gitclient.Push(buildDir)
	})
	if err == nil {
		d.Infof("It may take a minute or two to update on GitHub.\n")
	}
	return err
}
