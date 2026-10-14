package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gho-static/deployment"
	"gho-static/generator"
	"gho-static/gho_static"
	"gho-static/utils"
)

// Grey, Red, Link and Reset are terminal escape codes to start grey text,
// red text, green on black link text, and return to normal text.
const Grey, Red, Link, Reset = "\033[90m", "\033[31m", "\033[32;40m", "\033[0m"

const Usage = `Usage: gho-static [flags] <site folder>

Builds the site in <site folder> into ./build.

Flags:
  -g, --generate <path>   Create a new folder at <path>, fill it with a default
                          starter site and build it into <path>/build. Fails if
                          <path> already exists. If <path> is content/<name>,
                          instead adds a draft page <name>.md in a new
                          content/<name> folder inside <site folder>. If <path>
                          is content/<name>/<post>, adds a draft post.md in a
                          new content/<name>/YYYYMMDD_<post> folder inside
                          <site folder>, e.g. gho-static --generate content/news mysite
                          Builds afterwards with drafts and unreleased pages
                          included (-d -u).
  -d, --build-drafts      Also build pages whose folder contains a markdown file
                          marked "draft: true" (shown with a DRAFT warning).
  -u, --build-unreleased  Also build pages whose published date is in the future
                          (shown with an UNRELEASED warning).
  -p, --publish           Build for publishing: each page is written as
                          folder/index.html and linked by folder. Drafts and
                          unreleased pages are always skipped, overriding -d/-u.
  -y, --yes               With -g, use the default preferences without asking.
                          With --deploy, fail instead of asking for anything
                          that isn't set up.
  --deploy                Build <site folder> for publishing (-p) and deploy
                          it, using the deployment method its build folder is
                          set up for, or asking for one if there isn't one.
                          For GitHub Pages, commits the build folder and
                          pushes it.
  --deployment-method <github-pages|none>
                          Set up <site folder>/build for publishing, building
                          it first if it doesn't exist. github-pages makes it a
                          git repo, with --repo as its remote. Asks for the
                          method (and repo) if not given. With -g, used instead
                          of asking after the preferences.
  --repo <url>            With --deployment-method github-pages, the GitHub
                          repo to push the build folder to.

Single-letter flags can be combined, e.g. -du.
`

// Mode is what a Command does.
type Mode int

const (
	Build        Mode = iota // build Site
	GenerateSite             // create a starter site at GenPath with Prefs, then build it
	GeneratePage             // create a draft page at GenPath in Site, then build Site
	GeneratePost             // create a draft post PostName at GenPath in Site, then build Site
	SetupDeployment          // set up Site's build folder for Method, building it if missing
	Deploy                   // build Site for publishing and deploy it with Method
)

// Command is the work requested on the command line.
type Command struct {
	Mode     Mode
	Site     string // site folder to build; unset for GenerateSite
	GenPath  string // folder to generate
	PostName string // only set for GeneratePost
	Options  gho_static.Options
	Prefs    gho_static.Preferences // only set for GenerateSite
	Method   deployment.Method      // only set for GenerateSite, SetupDeployment and Deploy
	Repo     string                 // git remote for the build folder, only set for GitHub Pages
}

// Parse reads args (without the program name) into a Command, asking for
// the starter site's preferences when generating a site.
func Parse(args []string) (Command, error) {
	flags, positional := utils.ParseArgs(args)

	var cmd Command
	generate, useDefaults, deploying := false, false, false
	methodFlag, method, repoGiven := false, "", false
	for _, f := range flags {
		switch f.Key {
		case "g", "generate":
			generate, cmd.GenPath = true, f.Val
			continue
		case "deployment-method":
			methodFlag, method = true, f.Val
			if _, ok := deployment.Methods[method]; !ok && method != "" {
				// No method given, so ParseArgs took the site folder as one.
				if info, err := os.Stat(method); err != nil || !info.IsDir() {
					return cmd, fmt.Errorf("unknown deployment method %q: use github-pages or none", method)
				}
				positional, method = append(positional, method), ""
			}
			continue
		case "repo":
			cmd.Repo, repoGiven = f.Val, true
			continue
		case "d", "build-drafts":
			cmd.Options.BuildDrafts = true
		case "u", "build-unreleased":
			cmd.Options.BuildUnreleased = true
		case "p", "publish":
			cmd.Options.Publish = true
		case "y", "yes":
			useDefaults = true
		case "deploy":
			deploying = true
		default:
			continue
		}
		// Boolean flags take no value, so ParseArgs may have consumed a
		// following positional arg as one.
		if f.Val != "" {
			positional = append(positional, f.Val)
		}
	}
	if cmd.Options.Publish {
		cmd.Options.BuildDrafts, cmd.Options.BuildUnreleased = false, false
	}

	// The site folder is the first positional arg not used as the -g path.
	sites := positional
	if generate && cmd.GenPath == "" && len(positional) > 0 {
		cmd.GenPath, sites = positional[0], positional[1:]
	}
	if len(positional) == 0 && cmd.GenPath == "" {
		return cmd, fmt.Errorf("missing path argument")
	}

	// chooseMethod sets the deployment method and repo from the flags,
	// asking for any not given if ask is set.
	chooseMethod := func(r *bufio.Reader, ask bool) {
		cmd.Method = deployment.Methods[method]
		if method == "" && ask {
			cmd.Method = askDeploymentMethod(r)
		}
		if cmd.Method == deployment.GitHubPages && !repoGiven && ask {
			cmd.Repo = askRepo(r)
		}
	}

	if !generate {
		cmd.Site = positional[0]
		if deploying {
			cmd.Mode = Deploy
			if !methodFlag {
				existing, repo := deployment.Detect(filepath.Join(cmd.Site, "build"))
				if existing != deployment.None {
					method = "github-pages"
				}
				if repo != "" && !repoGiven {
					cmd.Repo, repoGiven = repo, true
				}
			}
			if useDefaults && (method == "" || (deployment.Methods[method] == deployment.GitHubPages && !repoGiven)) {
				return cmd, fmt.Errorf("%s has no deployment method or GitHub repo set up, and --yes stops them being asked for. Rerun without --yes, or pass --deployment-method github-pages --repo <url>.", cmd.Site)
			}
			chooseMethod(bufio.NewReader(os.Stdin), true)
			return cmd, nil
		}
		if methodFlag {
			cmd.Mode = SetupDeployment
			chooseMethod(bufio.NewReader(os.Stdin), true)
		}
		return cmd, nil
	}

	cmd.Options.BuildDrafts, cmd.Options.BuildUnreleased = true, true
	// content/<name> makes a page, content/<name>/<post> a dated post,
	// both inside the site folder.
	parts := strings.Split(filepath.ToSlash(filepath.Clean(cmd.GenPath)), "/")
	isPage := parts[0] == "content" && len(parts) == 2
	isPost := parts[0] == "content" && len(parts) == 3
	if !isPage && !isPost {
		// Check before prompting so the answers aren't wasted.
		if _, err := os.Stat(cmd.GenPath); err == nil {
			return cmd, fmt.Errorf("%s already exists", cmd.GenPath)
		}
		cmd.Mode = GenerateSite
		r := bufio.NewReader(os.Stdin)
		if useDefaults {
			fmt.Println("Using the default preferences (--yes). You can change these later in preferences.yaml.")
			cmd.Prefs = gho_static.DefaultPreferences
		} else {
			cmd.Prefs = askPreferences(r)
		}
		chooseMethod(r, !useDefaults)
		return cmd, nil
	}

	if len(sites) == 0 {
		return cmd, fmt.Errorf("missing site folder to create %s in", cmd.GenPath)
	}
	cmd.Site = sites[0]
	if info, err := os.Stat(cmd.Site); err != nil || !info.IsDir() {
		return cmd, fmt.Errorf("site folder %s does not exist. Create it first with `gho-static -g %s`, then rerun this command.", cmd.Site, cmd.Site)
	}
	if isPage {
		cmd.Mode = GeneratePage
		cmd.GenPath = filepath.Join(cmd.Site, cmd.GenPath)
	} else {
		cmd.Mode = GeneratePost
		cmd.PostName = parts[2]
		cmd.GenPath = filepath.Join(cmd.Site, parts[0], parts[1], generator.PostFolder(cmd.PostName, time.Now()))
	}
	return cmd, nil
}
