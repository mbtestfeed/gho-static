// Package gitclient runs git commands on a repo folder.
package gitclient

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Installed returns an error if git can't be found.
func Installed() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git isn't installed, or isn't on your PATH")
	}
	return nil
}

// Run runs git with args in dir, returning its output in any error.
func Run(dir string, args ...string) error {
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

// IsRepo reports whether dir is a git repo of its own, rather than inside a
// parent folder's repo.
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// DefaultBranch is the branch new repos start on.
const DefaultBranch = "main"

// Init makes dir a git repo on DefaultBranch, if it isn't already.
func Init(dir string) error {
	return Run(dir, "init", "-b", DefaultBranch)
}

// RemoteExists returns an error, with git's output, if the repo at url
// doesn't exist or can't be reached.
func RemoteExists(url string) error {
	return Run(".", "ls-remote", url)
}

// Reachable reports whether the repo at url can be reached without asking
// for credentials, e.g. over SSH with a key already set up.
func Reachable(url string) bool {
	cmd := exec.Command("git", "ls-remote", url)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	return cmd.Run() == nil
}

// SparseClone clones only the folders paths of the repo at url, at its
// latest commit, into dir. Files directly in the repo's root folder are
// cloned too.
func SparseClone(url, dir string, paths ...string) error {
	if err := Run(".", "clone", "--depth", "1", "--filter=blob:none", "--sparse", url, dir); err != nil {
		return err
	}
	return Run(dir, append([]string{"sparse-checkout", "set"}, paths...)...)
}

// RemoteDirs returns the names of the folders directly inside the folder
// path (slash separated) of the repo at url, at its latest commit, without
// downloading any files.
func RemoteDirs(url, path string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "gho-static-git-")
	if err != nil {
		return nil, fmt.Errorf("failed to create a temporary folder: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := Run(".", "clone", "--depth", "1", "--filter=blob:none", "--no-checkout", url, tmp); err != nil {
		return nil, err
	}
	out, err := exec.Command("git", "-C", tmp, "ls-tree", "-d", "--name-only", "HEAD", strings.TrimSuffix(path, "/")+"/").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed: %w", err)
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			dirs = append(dirs, line[strings.LastIndex(line, "/")+1:])
		}
	}
	return dirs, nil
}

// Clone clones the repo at url into dir, starting on DefaultBranch if the
// repo is empty.
func Clone(url, dir string) error {
	return Run(".", "-c", "init.defaultBranch="+DefaultBranch, "clone", url, dir)
}

// RemoteBranch returns the default branch of the origin remote of the repo
// at dir, or "" if the remote is empty or can't be reached.
func RemoteBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "ls-remote", "--symref", "origin", "HEAD").Output()
	if err != nil || !strings.HasPrefix(string(out), "ref: refs/heads/") {
		return ""
	}
	branch, _, _ := strings.Cut(strings.TrimPrefix(string(out), "ref: refs/heads/"), "\t")
	return branch
}

// ResetOnto fetches branch from the origin remote of the repo at dir and
// moves dir onto it, leaving dir's files as they are, so the next commit
// follows on from the remote's history. dir's own unpushed commits are
// dropped.
func ResetOnto(dir, branch string) error {
	for _, args := range [][]string{
		{"fetch", "origin", branch},
		{"symbolic-ref", "HEAD", "refs/heads/" + branch},
		{"reset", "--soft", "origin/" + branch},
	} {
		if err := Run(dir, args...); err != nil {
			return err
		}
	}
	return nil
}

// Remote returns the URL of the origin remote of the repo at dir, or "" if
// it has none.
func Remote(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// SetRemote sets the origin remote of the repo at dir to url, adding it if
// missing. dir must already be a repo, or a parent folder's remote is
// mistaken for its own.
func SetRemote(dir, url string) error {
	if Remote(dir) != "" {
		return Run(dir, "remote", "set-url", "origin", url)
	}
	return Run(dir, "remote", "add", "origin", url)
}

// CommitAll commits everything in the repo at dir with message, if anything
// changed.
func CommitAll(dir, message string) error {
	if err := Run(dir, "add", "-A"); err != nil {
		return err
	}
	if Run(dir, "diff", "--cached", "--quiet") == nil {
		return nil
	}
	return Run(dir, "commit", "-m", message)
}

// Push pushes the current branch of the repo at dir to origin.
func Push(dir string) error {
	return Run(dir, "push", "-u", "origin", "HEAD")
}
