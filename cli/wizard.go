package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"gho-static/deployment"
	"gho-static/deployment/github_pages"
	"gho-static/gho_static"
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

// askPreferences asks whether to use the default preferences and, if not,
// asks for each value in turn, keeping the default for any left blank.
func askPreferences(r *bufio.Reader) gho_static.Preferences {
	prefs := gho_static.DefaultPreferences
	defaults, _ := yaml.Marshal(prefs)
	hints := strings.Split(strings.TrimRight(string(defaults), "\n"), "\n")
	for {
		switch strings.ToLower(ask(r, "Do you want to use the default values? Y/N (You can change these later in preferences.yaml) ", hints)) {
		case "", "y", "yes":
			return prefs
		case "n", "no":
			// Nav links are asked for comma separated.
			navLinks := strings.Join(prefs.NavLinks, ",")
			for _, f := range []struct {
				name string
				val  *string
			}{
				{"site_name", &prefs.SiteName},
				{"nav_links (comma separated)", &navLinks},
				{"banner_color (light)", &prefs.BannerColor.Light},
				{"banner_color (dark)", &prefs.BannerColor.Dark},
				{"banner_text_color (light)", &prefs.BannerTextColor.Light},
				{"banner_text_color (dark)", &prefs.BannerTextColor.Dark},
				{"background_color (light)", &prefs.BackgroundColor.Light},
				{"background_color (dark)", &prefs.BackgroundColor.Dark},
				{"text_color (light)", &prefs.TextColor.Light},
				{"text_color (dark)", &prefs.TextColor.Dark},
				{"link (light)", &prefs.Link.Light},
				{"link (dark)", &prefs.Link.Dark},
				{"link_clicked (light)", &prefs.LinkClicked.Light},
				{"link_clicked (dark)", &prefs.LinkClicked.Dark},
				{"tag_color (light)", &prefs.TagColor.Light},
				{"tag_color (dark)", &prefs.TagColor.Dark},
				{"tag_text_color (light)", &prefs.TagTextColor.Light},
				{"tag_text_color (dark)", &prefs.TagTextColor.Dark},
				{"font", &prefs.Font},
				{"logo", &prefs.Logo},
				{"favicon", &prefs.Favicon},
				{"default_author", &prefs.DefaultAuthor},
				{"intro_sidebar_image", &prefs.IntroSidebarImage},
				{"intro_name", &prefs.IntroName},
				{"intro_description", &prefs.IntroDescription},
			} {
				if v := ask(r, f.name+": ", []string{"press enter to accept default value: " + *f.val}); v != "" {
					*f.val = v
				}
			}
			for _, f := range []struct {
				name string
				val  *bool
			}{
				{"show_intro_sidebar", &prefs.ShowIntroSidebar},
				{"show_recent_blogs_sidebar", &prefs.ShowRecentBlogsSidebar},
			} {
				def := map[bool]string{true: "Y", false: "N"}[*f.val]
				switch strings.ToLower(ask(r, f.name+" Y/N: ", []string{"press enter to accept default value: " + def})) {
				case "y", "yes", "true":
					*f.val = true
				case "n", "no", "false":
					*f.val = false
				}
			}
			prefs.NavLinks = strings.Split(navLinks, ",")
			return prefs
		}
	}
}
