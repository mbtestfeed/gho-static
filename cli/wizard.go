package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"gho-static/deployment"
	"gho-static/deployment/github_pages"
)

// ask prints prompt with hints in grey on the lines beneath it, reads a line
// of input typed after the prompt and returns it trimmed.
func ask(r *bufio.Reader, prompt string, hints []string) string {
	fmt.Print(prompt + "\n")
	for _, h := range hints {
		fmt.Println(Grey + h + Reset)
	}
	// Move back up to the end of the prompt line for the input.
	fmt.Printf("\033[%dA\r\033[%dC", len(hints)+1, len(prompt))
	answer, _ := r.ReadString('\n')
	// Enter leaves the cursor on the first hint line; move below them all.
	fmt.Printf("\033[%dB", len(hints))
	return strings.TrimSpace(answer)
}

// AskDate asks for a published date for the undated page at mdPath,
// defaulting to today, until a valid yyyymmdd date is given.
func AskDate(r *bufio.Reader, mdPath string) time.Time {
	today := time.Now()
	for {
		answer := ask(r, mdPath+" has no published date. Enter one as yyyymmdd: ", []string{"press enter to use today's date: " + today.Format("20060102")})
		if answer == "" {
			return today
		}
		if date, err := time.ParseInLocation("20060102", answer, time.Local); err == nil {
			return date
		}
		fmt.Println(Red + answer + " isn't a valid date in yyyymmdd format." + Reset)
	}
}

// AskToPublishDraft asks whether to publish the draft page at mdPath.
func AskToPublishDraft(r *bufio.Reader, mdPath string) bool {
	for {
		switch strings.ToLower(ask(r, mdPath+" is a draft. Publish it? Y/N ", []string{"press enter for no"})) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}

// askNavLink asks whether to add a menu link for the content folder name.
func askNavLink(r *bufio.Reader, name string) bool {
	for {
		switch strings.ToLower(ask(r, "Add a menu link for "+name+"? Y/N ", []string{"press enter for yes"})) {
		case "", "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

// askNewCollection asks whether to add a new collection of posts named name.
func askNewCollection(r *bufio.Reader, name string) bool {
	for {
		switch strings.ToLower(ask(r, "There's no "+name+" collection yet. Are you sure you want to add a new collection called "+name+"? Y/N ", []string{"press enter for no"})) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}

// askDeploymentMethod asks how the build folder will be published.
func askDeploymentMethod(r *bufio.Reader) deployment.Method {
	for {
		switch strings.ToLower(ask(r, "How will you deploy the site? ", []string{"1: GitHub Pages", "2: none (default)"})) {
		case "", "2", "none":
			return deployment.None
		case "1", "github pages":
			return deployment.GitHubPages
		}
	}
}

// askRepo asks for a GitHub username and returns the URL of their
// <user>.github.io repo, or "" if none is given.
func askRepo(r *bufio.Reader) string {
	user := ask(r, "GitHub username: ", []string{"the site is published from the <username>.github.io repo, or press enter to add it later"})
	if user == "" {
		return ""
	}
	return github_pages.RepoURL(user)
}
