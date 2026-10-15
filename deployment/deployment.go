// Package deployment sets up and deploys a site's build folder with its
// deployment method.
package deployment

import (
	"fmt"
	"path/filepath"

	"gho-static/deployment/github_pages"
	"gho-static/gho_static"
)

// Method is how a site's build folder is published.
type Method int

const (
	None        Method = iota
	GitHubPages        // build folder is a git repo pushed to GitHub Pages
)

// Methods maps each --deployment-method value to its method.
var Methods = map[string]Method{"none": None, "github-pages": GitHubPages}

// Detect returns the method buildDir is set up for, and the repo it
// deploys to (if any).
func Detect(buildDir string) (Method, string) {
	if github_pages.IsSetUp(buildDir) {
		return GitHubPages, github_pages.Repo(buildDir)
	}
	return None, ""
}

// Check returns an error explaining what to do if repo isn't ready for
// method to deploy to.
func Check(method Method, repo string) error {
	if method == GitHubPages && repo != "" {
		return github_pages.CheckRepo(repo)
	}
	return nil
}

// Setup sets up buildDir for method, deploying to repo. The site should be
// built into it afterwards.
func Setup(buildDir string, method Method, repo string) error {
	if method == GitHubPages {
		return github_pages.Setup(buildDir, repo)
	}
	return nil
}

// Deploy checks everything method needs is in place, builds site for
// publishing into its build folder and deploys it to repo, returning the
// build version.
func Deploy(site string, method Method, repo string) (string, error) {
	if method == None {
		return "", fmt.Errorf("no deployment method is set up for %s, so there's nothing to deploy to", site)
	}
	if err := github_pages.Check(repo); err != nil {
		return "", err
	}
	buildDir := filepath.Join(site, "build")
	if err := github_pages.Setup(buildDir, repo); err != nil {
		return "", err
	}
	version, err := gho_static.Build(site, gho_static.Options{Publish: true})
	if err != nil {
		return "", err
	}
	return version, github_pages.Push(buildDir)
}
