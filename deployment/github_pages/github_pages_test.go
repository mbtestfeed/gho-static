package github_pages

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitclient "gho-static/utils/git-client"
	"gho-static/utils/output"
)

// requireGit skips the test if git isn't installed.
func requireGit(t *testing.T) {
	t.Helper()
	if gitclient.Installed() != nil {
		t.Skip("git isn't installed")
	}
}

// quiet returns an Output writing to a buffer, and the buffer.
func quiet() (output.Output, *bytes.Buffer) {
	var out bytes.Buffer
	return output.Output{Out: &out, Err: &out}, &out
}

func TestRepoURL(t *testing.T) {
	if got, want := RepoURL("me"), "git@github.com:me/me.github.io.git"; got != want {
		t.Errorf("RepoURL = %q, want %q", got, want)
	}
}

func TestRepoPath(t *testing.T) {
	for _, repo := range []string{
		"git@github.com:me/site.git",
		"git@github.com:me/site",
		"https://github.com/me/site.git",
		"https://github.com/me/site",
	} {
		if owner, name := repoPath(repo); owner != "me" || name != "site" {
			t.Errorf("repoPath(%q) = %q, %q, want me, site", repo, owner, name)
		}
	}
}

func TestRepoLinks(t *testing.T) {
	create, settings := RepoLinks(RepoURL("me"))
	if want := "https://github.com/new?owner=me&name=me.github.io"; create != want {
		t.Errorf("create link = %q, want %q", create, want)
	}
	if want := "https://github.com/me/me.github.io/settings/pages"; settings != want {
		t.Errorf("settings link = %q, want %q", settings, want)
	}
}

func TestPagesURL(t *testing.T) {
	cases := []struct{ repo, want string }{
		{RepoURL("me"), "https://me.github.io/"},
		{"git@github.com:Me/me.github.io.git", "https://me.github.io/"},
		{"https://github.com/me/project.git", "https://me.github.io/project/"},
	}
	for _, c := range cases {
		if got := PagesURL(c.repo); got != c.want {
			t.Errorf("PagesURL(%q) = %q, want %q", c.repo, got, c.want)
		}
	}
}

func TestDetect(t *testing.T) {
	if _, ok := Detect(t.TempDir()); ok {
		t.Error("detected GitHub Pages for a folder that isn't a repo")
	}
	requireGit(t)
	cases := []struct {
		name   string
		remote string // or none if ""
		want   string
	}{
		{name: "pages repo", remote: RepoURL("me"), want: "me"},
		{name: "pages repo over https", remote: "https://github.com/me/me.github.io.git", want: "me"},
		{name: "other repo", remote: "git@github.com:me/project.git"},
		{name: "no remote"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			build := t.TempDir()
			if err := gitclient.Init(build); err != nil {
				t.Fatal(err)
			}
			if c.remote != "" {
				if err := gitclient.SetRemote(build, c.remote); err != nil {
					t.Fatal(err)
				}
			}
			if username, ok := Detect(build); !ok || username != c.want {
				t.Errorf("Detect = %q, %v, want %q, true", username, ok, c.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	out, _ := quiet()
	var asked []string
	ask := func(prompt string, hints []string, secret bool) string {
		asked = append(asked, prompt)
		return "asked"
	}
	if d := New("me", ask, out); d.Username != "me" || len(asked) != 0 {
		t.Errorf("New with a username = %q, asking %q, want me without asking", d.Username, asked)
	}
	if d := New("", ask, out); d.Username != "asked" || len(asked) != 1 {
		t.Errorf("New without a username = %q, asking %q, want it asked for", d.Username, asked)
	}
	if d := New("", nil, out); d.Username != "" {
		t.Errorf("New without a username or asking = %q, want none", d.Username)
	}
}

func TestSettings(t *testing.T) {
	if s := (Deployer{Username: "me"}).Settings(); len(s) != 1 || s["username"] != "me" {
		t.Errorf("Settings = %v, want the username", s)
	}
	if s := (Deployer{}).Settings(); s != nil {
		t.Errorf("Settings without a username = %v, want nil", s)
	}
}

func TestChecksWithoutUsername(t *testing.T) {
	requireGit(t)
	d := Deployer{}
	if err := d.Check(); err != nil {
		t.Errorf("Check without a username = %v, want nil, as it's added later", err)
	}
	if err := d.CheckPush(); err == nil || !strings.Contains(err.Error(), "no GitHub username") {
		t.Errorf("CheckPush without a username = %v, want an error saying so", err)
	}
}

// gitOut runs git with args in dir, returning its trimmed output and failing
// the test if it fails.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func TestSetupAndPush(t *testing.T) {
	requireGit(t)
	out, written := quiet()
	d := Deployer{Output: out}

	// Without a username, the build folder is made a repo to push later.
	build := filepath.Join(t.TempDir(), "build")
	if err := d.Setup(build); err != nil {
		t.Fatal(err)
	}
	if !gitclient.IsRepo(build) {
		t.Fatal("Setup didn't make the build folder a repo")
	}
	for _, kv := range [][]string{{"user.name", "Test"}, {"user.email", "test@example.com"}, {"commit.gpgsign", "false"}} {
		if err := gitclient.Run(build, "config", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}

	// Pushed to a local remote, in place of GitHub.
	remote := t.TempDir()
	if err := gitclient.Run(remote, "init", "--bare", "-b", gitclient.DefaultBranch); err != nil {
		t.Fatal(err)
	}
	if err := gitclient.SetRemote(build, remote); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(build, "index.html"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := d.Push(build); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, remote, "show", gitclient.DefaultBranch+":index.html"); got != "hello" {
		t.Errorf("pushed index.html = %q, want hello", got)
	}
	if !strings.Contains(written.String(), "Deploying to "+remote) {
		t.Errorf("the deploy wasn't announced:\n%s", written)
	}

	// Pushing again with nothing changed adds no commit.
	before := gitOut(t, remote, "rev-list", "--count", gitclient.DefaultBranch)
	if err := d.Push(build); err != nil {
		t.Fatalf("pushing without changes failed: %v", err)
	}
	if after := gitOut(t, remote, "rev-list", "--count", gitclient.DefaultBranch); after != before {
		t.Errorf("pushing without changes made %s commits, want %s", after, before)
	}
}
