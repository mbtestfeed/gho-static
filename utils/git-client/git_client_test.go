package gitclient

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// requireGit skips the test if git isn't installed.
func requireGit(t *testing.T) {
	t.Helper()
	if Installed() != nil {
		t.Skip("git isn't installed")
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

// newRepo returns a new repo in a temporary folder, set up to commit without
// relying on the user's git config.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][]string{{"user.name", "Test"}, {"user.email", "test@example.com"}, {"commit.gpgsign", "false"}} {
		if err := Run(dir, "config", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// newRemote returns a new, empty bare repo on DefaultBranch in a temporary
// folder.
func newRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Run(dir, "init", "--bare", "-b", DefaultBranch); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeFile writes content to name in dir, failing the test if it can't.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// pushedRepo returns a repo with one commit pushed to a new remote, and the
// remote.
func pushedRepo(t *testing.T) (repo, remote string) {
	t.Helper()
	repo, remote = newRepo(t), newRemote(t)
	if err := SetRemote(repo, remote); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, "index.html", "hello")
	if err := CommitAll(repo, "first"); err != nil {
		t.Fatal(err)
	}
	if err := Push(repo); err != nil {
		t.Fatal(err)
	}
	return repo, remote
}

func TestInstalled(t *testing.T) {
	_, lookErr := exec.LookPath("git")
	if err := Installed(); (err == nil) != (lookErr == nil) {
		t.Errorf("Installed() = %v, but looking git up gave %v", err, lookErr)
	}
}

func TestRun(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	err := Run(dir, "rev-parse", "--git-dir")
	if err == nil || !strings.Contains(err.Error(), "git rev-parse --git-dir failed") {
		t.Errorf("Run outside a repo returned %v, want a failure naming the command", err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if err := Run(dir, "rev-parse", "--git-dir"); err != nil {
		t.Errorf("Run in a repo returned %v", err)
	}
}

func TestIsRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("IsRepo before Init = true")
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Error("IsRepo after Init = false")
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if IsRepo(sub) {
		t.Error("IsRepo of a folder inside a repo = true")
	}
}

func TestInit(t *testing.T) {
	requireGit(t)
	dir := newRepo(t)
	if got, want := gitOut(t, dir, "symbolic-ref", "HEAD"), "refs/heads/"+DefaultBranch; got != want {
		t.Errorf("new repo's HEAD = %q, want %q", got, want)
	}
}

func TestRemoteAndSetRemote(t *testing.T) {
	requireGit(t)
	dir := newRepo(t)
	if got := Remote(dir); got != "" {
		t.Errorf("Remote of a new repo = %q, want \"\"", got)
	}
	for _, url := range []string{"https://example.com/first.git", "https://example.com/second.git"} {
		if err := SetRemote(dir, url); err != nil {
			t.Fatal(err)
		}
		if got := Remote(dir); got != url {
			t.Errorf("Remote after SetRemote(%q) = %q", url, got)
		}
	}
}

func TestCommitAll(t *testing.T) {
	requireGit(t)
	dir := newRepo(t)
	if err := CommitAll(dir, "nothing"); err != nil {
		t.Fatalf("CommitAll with nothing to commit returned %v", err)
	}
	if err := Run(dir, "rev-parse", "HEAD"); err == nil {
		t.Error("CommitAll with nothing to commit made a commit")
	}

	writeFile(t, dir, "index.html", "hello")
	if err := CommitAll(dir, "first"); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, dir, "log", "-1", "--format=%s"); got != "first" {
		t.Errorf("last commit message = %q, want \"first\"", got)
	}

	if err := CommitAll(dir, "unchanged"); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, dir, "rev-list", "--count", "HEAD"); got != "1" {
		t.Errorf("commits after an unchanged CommitAll = %s, want 1", got)
	}
}

func TestRemoteExists(t *testing.T) {
	requireGit(t)
	if err := RemoteExists(newRemote(t)); err != nil {
		t.Errorf("RemoteExists of a repo returned %v", err)
	}
	if err := RemoteExists(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("RemoteExists of a missing repo returned no error")
	}
}

func TestClone(t *testing.T) {
	requireGit(t)
	_, remote := pushedRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	if err := Clone(remote, dir); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Fatal("Clone didn't make a repo")
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Errorf("clone is missing the pushed file: %v", err)
	}

	t.Run("empty remote starts on DefaultBranch", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "clone")
		if err := Clone(newRemote(t), dir); err != nil {
			t.Fatal(err)
		}
		if got, want := gitOut(t, dir, "symbolic-ref", "HEAD"), "refs/heads/"+DefaultBranch; got != want {
			t.Errorf("empty clone's HEAD = %q, want %q", got, want)
		}
	})
}

func TestPushAndRemoteBranch(t *testing.T) {
	requireGit(t)
	repo, remote := pushedRepo(t)
	if got, want := gitOut(t, remote, "rev-parse", DefaultBranch), gitOut(t, repo, "rev-parse", "HEAD"); got != want {
		t.Errorf("remote's %s = %s, want the pushed commit %s", DefaultBranch, got, want)
	}
	if got := RemoteBranch(repo); got != DefaultBranch {
		t.Errorf("RemoteBranch = %q, want %q", got, DefaultBranch)
	}

	empty := newRepo(t)
	if err := SetRemote(empty, newRemote(t)); err != nil {
		t.Fatal(err)
	}
	if got := RemoteBranch(empty); got != "" {
		t.Errorf("RemoteBranch of an empty remote = %q, want \"\"", got)
	}
}

func TestReachable(t *testing.T) {
	requireGit(t)
	if !Reachable(newRemote(t)) {
		t.Error("Reachable of a local repo = false")
	}
	if Reachable(filepath.Join(t.TempDir(), "missing")) {
		t.Error("Reachable of a missing repo = true")
	}
}

func TestSparseCloneAndRemoteDirs(t *testing.T) {
	requireGit(t)
	repo, remote := newRepo(t), newRemote(t)
	if err := SetRemote(repo, remote); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "themes/a/theme.css", "themes/b/theme.css", "other/file.txt"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(name)), 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, repo, name, name)
	}
	if err := CommitAll(repo, "files"); err != nil {
		t.Fatal(err)
	}
	if err := Push(repo); err != nil {
		t.Fatal(err)
	}

	dirs, err := RemoteDirs(remote, "themes")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !slices.Equal(dirs, want) {
		t.Errorf("RemoteDirs(themes) = %q, want %q", dirs, want)
	}

	dir := filepath.Join(t.TempDir(), "clone")
	if err := SparseClone(remote, dir, "themes/a"); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"README.md":          true,
		"themes/a/theme.css": true,
		"themes/b/theme.css": false,
		"other/file.txt":     false,
	} {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if got := err == nil; got != want {
			t.Errorf("%s cloned = %v, want %v", name, got, want)
		}
	}
}

func TestResetOnto(t *testing.T) {
	requireGit(t)
	pushed, remote := pushedRepo(t)
	dir := newRepo(t)
	if err := SetRemote(dir, remote); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "index.html", "changed")
	if err := ResetOnto(dir, DefaultBranch); err != nil {
		t.Fatal(err)
	}
	if got, want := gitOut(t, dir, "rev-parse", "HEAD"), gitOut(t, pushed, "rev-parse", "HEAD"); got != want {
		t.Errorf("HEAD after ResetOnto = %s, want the remote's %s", got, want)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "index.html")); err != nil || string(b) != "changed" {
		t.Errorf("index.html after ResetOnto = %q (%v), want it left as \"changed\"", b, err)
	}
}
