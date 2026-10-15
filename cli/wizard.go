package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gho-static/deployment"
	"gho-static/deployment/github_pages"
)

// Terminal is where questions are asked and answered.
type Terminal struct {
	In          *bufio.Reader
	Out         io.Writer
	Interactive bool // whether anyone can answer on In
}

// Stdio returns the terminal on stdin and stdout.
func Stdio() Terminal {
	info, err := os.Stdin.Stat()
	return Terminal{In: bufio.NewReader(os.Stdin), Out: os.Stdout, Interactive: err == nil && info.Mode()&os.ModeCharDevice != 0}
}

// ask prints prompt with hints in grey on the lines beneath it, reads a line
// of input typed after the prompt and returns it trimmed.
func (t Terminal) ask(prompt string, hints []string) string {
	fmt.Fprint(t.Out, prompt+"\n")
	for _, h := range hints {
		fmt.Fprintln(t.Out, Grey+h+Reset)
	}
	// Move back up to the end of the prompt line for the input.
	fmt.Fprintf(t.Out, "\033[%dA\r\033[%dC", len(hints)+1, len(prompt))
	answer, _ := t.In.ReadString('\n')
	// Enter leaves the cursor on the first hint line; move below them all.
	fmt.Fprintf(t.Out, "\033[%dB", len(hints))
	return strings.TrimSpace(answer)
}

// AskDate asks for a published date for the undated page at mdPath,
// defaulting to today, until a valid yyyymmdd date is given.
func (t Terminal) AskDate(mdPath string, today time.Time) time.Time {
	for {
		answer := t.ask(mdPath+" has no published date. Enter one as yyyymmdd: ", []string{"press enter to use today's date: " + today.Format("20060102")})
		if answer == "" {
			return today
		}
		if date, err := time.ParseInLocation("20060102", answer, time.Local); err == nil {
			return date
		}
		fmt.Fprintln(t.Out, Red+answer+" isn't a valid date in yyyymmdd format."+Reset)
	}
}

// AskToPublishDraft asks whether to publish the draft page at mdPath.
func (t Terminal) AskToPublishDraft(mdPath string) bool {
	for {
		switch strings.ToLower(t.ask(mdPath+" is a draft. Publish it? Y/N ", []string{"press enter for no"})) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}

// askNavLink asks whether to add a menu link for the content folder name.
func (t Terminal) askNavLink(name string) bool {
	for {
		switch strings.ToLower(t.ask("Add a menu link for "+name+"? Y/N ", []string{"press enter for yes"})) {
		case "", "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

// askNewCollection asks whether to add a new collection of posts named name.
func (t Terminal) askNewCollection(name string) bool {
	for {
		switch strings.ToLower(t.ask("There's no "+name+" collection yet. Are you sure you want to add a new collection called "+name+"? Y/N ", []string{"press enter for no"})) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}

// askDeploymentMethod asks how the build folder will be published.
func (t Terminal) askDeploymentMethod() deployment.Method {
	for {
		switch strings.ToLower(t.ask("How will you deploy the site? ", []string{"1: GitHub Pages", "2: none (default)"})) {
		case "", "2", "none":
			return deployment.None
		case "1", "github pages":
			return deployment.GitHubPages
		}
	}
}

// askRepo asks for a GitHub username and returns the URL of their
// <user>.github.io repo, or "" if none is given.
func (t Terminal) askRepo() string {
	user := t.ask("GitHub username: ", []string{"the site is published from the <username>.github.io repo, or press enter to add it later"})
	if user == "" {
		return ""
	}
	return github_pages.RepoURL(user)
}
