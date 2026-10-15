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

Builds the site in <site folder> into <site folder>/build.

Flags:
  -g, --generate <path>   Create a new folder at <path>, fill it with a default
                          starter site and build it into <path>/build. Fails if
                          <path> already exists. If <path> is content/<name>,
                          instead adds a draft page <name>.md in a new
                          content/<name> folder inside <site folder>. If <path>
                          is content/<name>/<post>, adds a draft post.md in a
                          new content/<name>/YYYYMMDD_<post> folder inside
                          <site folder>, e.g. gho-static --generate content/news mysite
                          Builds afterwards.
  -c, --clean            With -g, generate markdown files with only their front
                          matter. A new site is named after its folder, with
                          one empty post instead of the example posts, and no
                          intro sidebar image, name or links.
  --preview-publish       Build without drafts and unreleased pages, as
                          publishing does, but keep the links working locally.
                          Otherwise they're built, with a warning on each.
  -p, --publish           Build for publishing: each page is written as
                          folder/index.html and linked by folder. Drafts and
                          unreleased pages are left out. Can't be used with
                          --preview-publish.
  -y, --yes               With -g, don't ask for a deployment method.
                          With --deploy, fail instead of asking for anything
                          that isn't set up.
  --drafts                Go through each draft page in <site folder>, asking
                          whether to publish it (removing its "draft: true").
                          With --yes, publishes them all.
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
                          of asking.
  --repo <url>            With --deployment-method github-pages, the GitHub
                          repo to push the build folder to.

Single-letter flags can be combined, e.g. -cy.
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
	PublishDrafts            // ask whether to publish each of Site's draft pages
)

// Command is the work requested on the command line.
type Command struct {
	Mode     Mode
	Site     string // site folder to build; unset for GenerateSite
	GenPath  string // folder to generate
	PostName string // only set for GeneratePost
	NavLink  string // content folder to add to nav_links, only set for GeneratePage and GeneratePost
	Yes      bool   // answer yes to every question (--yes)
	Clean    bool   // generate without example content (--clean)
	Options  gho_static.Options
	Prefs    gho_static.Preferences // only set for GenerateSite
	Method   deployment.Method      // only set for GenerateSite, SetupDeployment and Deploy
	Repo     string                 // git remote for the build folder, only set for GitHub Pages
}

// Parse reads args (without the program name) into a Command, asking for
// anything missing that it needs.
func Parse(args []string) (Command, error) {
	flags, positional := utils.ParseArgs(args)

	var cmd Command
	generate, useDefaults, deploying, drafts := false, false, false, false
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
		case "preview-publish":
			cmd.Options.Preview = true
		case "p", "publish":
			cmd.Options.Publish = true
		case "y", "yes":
			useDefaults = true
		case "c", "clean":
			cmd.Clean = true
		case "deploy":
			deploying = true
		case "drafts":
			drafts = true
		default:
			continue
		}
		// Boolean flags take no value, so ParseArgs may have consumed a
		// following positional arg as one.
		if f.Val != "" {
			positional = append(positional, f.Val)
		}
	}
	if cmd.Options.Publish && cmd.Options.Preview {
		return cmd, fmt.Errorf("--preview-publish and --publish can't be used together: --preview-publish previews the published site with links that work locally, while --publish builds it for deploying. Rerun with just one of them.")
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
		if drafts {
			cmd.Mode, cmd.Yes = PublishDrafts, useDefaults
			return cmd, nil
		}
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
		cmd.Prefs = gho_static.DefaultPreferences
		if cmd.Clean {
			cmd.Prefs.SiteName = filepath.Base(filepath.Clean(cmd.GenPath))
			cmd.Prefs.IntroSidebarImage, cmd.Prefs.IntroName = "", ""
			cmd.Prefs.IntroDescription = "This is a blank slate, created using Gho-Static"
			// Keep the link names, blanked, so they're easy to fill in.
			cmd.Prefs.SocialLinks = nil
			for _, l := range gho_static.DefaultPreferences.SocialLinks {
				cmd.Prefs.SocialLinks = append(cmd.Prefs.SocialLinks, gho_static.SocialLink{Name: l.Name})
			}
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
	// Offer a menu link for a new first-level folder, whether it's the page
	// itself or the parent of a new post.
	if _, err := os.Stat(filepath.Join(cmd.Site, "content", parts[1])); os.IsNotExist(err) {
		r := bufio.NewReader(os.Stdin)
		// A new post's parent is a new collection, so check its name first.
		if isPost && !useDefaults {
			if !Interactive() {
				return cmd, fmt.Errorf("%s would add a new collection, %s, and there's no one to ask whether that's intended. Rerun with --yes to add it.", cmd.GenPath, parts[1])
			}
			if !askNewCollection(r, parts[1]) {
				return cmd, fmt.Errorf("%s wasn't created.", cmd.GenPath)
			}
		}
		if useDefaults || (Interactive() && askNavLink(r, parts[1])) {
			cmd.NavLink = parts[1]
		}
	}
	return cmd, nil
}

// Interactive reports whether stdin is a terminal someone can answer
// questions from.
func Interactive() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
