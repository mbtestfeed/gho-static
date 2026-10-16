package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"gho-static/deployment"
	"gho-static/utils/output"
)

// Terminal is where questions are asked and answered, and messages written.
type Terminal struct {
	In *bufio.Reader
	output.Output
	Interactive bool // whether anyone can answer on In

	readHidden func() ([]byte, error) // reads a line from In without showing it, if In can do that
}

// Stdio returns the terminal on stdin, stdout and stderr.
func Stdio() Terminal {
	info, err := os.Stdin.Stat()
	return Terminal{
		In:          bufio.NewReader(os.Stdin),
		Output:      output.Output{Out: os.Stdout, Err: os.Stderr},
		Interactive: err == nil && info.Mode()&os.ModeCharDevice != 0,
		readHidden:  func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) },
	}
}

// Ask prints prompt with hints in grey on the lines beneath it, reads a line
// of input typed after the prompt and returns it trimmed.
func (t Terminal) Ask(prompt string, hints []string) string {
	return t.ask(prompt, hints, func() string {
		answer, _ := t.In.ReadString('\n')
		return answer
	})
}

// AskSecret asks as Ask does, but without showing what's typed when
// someone's answering at the terminal.
func (t Terminal) AskSecret(prompt string, hints []string) string {
	if !t.Interactive || t.readHidden == nil {
		return t.Ask(prompt, hints)
	}
	return t.ask(prompt, hints, func() string {
		answer, _ := t.readHidden()
		// Nothing typed is shown, Enter included, so end the line here.
		fmt.Fprintln(t.Out)
		return string(answer)
	})
}

// AskSetting asks for a deployment setting, as a secret if secret is set.
func (t Terminal) AskSetting(prompt string, hints []string, secret bool) string {
	if secret {
		return t.AskSecret(prompt, hints)
	}
	return t.Ask(prompt, hints)
}

// ask prints prompt with hints in grey on the lines beneath it, reads the
// answer typed after the prompt with read and returns it trimmed.
func (t Terminal) ask(prompt string, hints []string, read func() string) string {
	fmt.Fprint(t.Out, prompt+"\n")
	for _, h := range hints {
		fmt.Fprintln(t.Out, output.Grey+h+output.Reset)
	}
	// Move back up to the end of the prompt line for the input.
	fmt.Fprintf(t.Out, "\033[%dA\r\033[%dC", len(hints)+1, len(prompt))
	answer := read()
	// Enter leaves the cursor on the first hint line; move below them all.
	fmt.Fprintf(t.Out, "\033[%dB", len(hints))
	return strings.TrimSpace(answer)
}

// AskDate asks for a published date for the undated page at mdPath,
// defaulting to today, until a valid yyyymmdd date is given.
func (t Terminal) AskDate(mdPath string, today time.Time) time.Time {
	for {
		answer := t.Ask(mdPath+" has no published date. Enter one as yyyymmdd: ", []string{"press enter to use today's date: " + today.Format("20060102")})
		if answer == "" {
			return today
		}
		if date, err := time.ParseInLocation("20060102", answer, time.Local); err == nil {
			return date
		}
		fmt.Fprintln(t.Out, output.Red+answer+" isn't a valid date in yyyymmdd format."+output.Reset)
	}
}

// AskToPublishDraft asks whether to publish the draft page at mdPath.
func (t Terminal) AskToPublishDraft(mdPath string) bool {
	for {
		switch strings.ToLower(t.Ask(mdPath+" is a draft. Publish it? Y/N ", []string{"press enter for no"})) {
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
		switch strings.ToLower(t.Ask("Add a menu link for "+name+"? Y/N ", []string{"press enter for yes"})) {
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
		switch strings.ToLower(t.Ask("There's no "+name+" collection yet. Are you sure you want to add a new collection called "+name+"? Y/N ", []string{"press enter for no"})) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		}
	}
}

// askDeploymentMethod asks which of deployment.Methods will publish the
// build folder, by number, name or label.
func (t Terminal) askDeploymentMethod() deployment.Method {
	var hints []string
	for i, m := range deployment.Methods {
		hint := strconv.Itoa(i+1) + ": " + m.Label
		if m.Name == deployment.Default {
			hint += " (default)"
		}
		hints = append(hints, hint)
	}
	for {
		answer := strings.ToLower(t.Ask("How will you deploy the site? ", hints))
		if answer == "" {
			answer = deployment.Default
		}
		for i, m := range deployment.Methods {
			if answer == strconv.Itoa(i+1) || answer == m.Name || answer == strings.ToLower(m.Label) {
				return m
			}
		}
	}
}
