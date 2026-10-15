package cli

import (
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

	method      string // --deployment-method value, if it's a method
	badMethod   string // --deployment-method value that isn't a method, so is taken as the site folder
	methodGiven bool   // --deployment-method was given
	repoGiven   bool   // --repo was given
	collection  string // content folder a page or post is generated in
}

// Parse reads args (without the program name) into a Command, without
// looking at the disk or asking anything: Resolve does that.
func Parse(args []string) (Command, error) {
	flags, positional := utils.ParseArgs(args)

	var cmd Command
	generate, deploying, drafts := false, false, false
	for _, f := range flags {
		switch f.Key {
		case "g", "generate":
			generate, cmd.GenPath = true, f.Val
			continue
		case "deployment-method":
			cmd.methodGiven, cmd.method = true, f.Val
			if _, ok := deployment.Methods[f.Val]; !ok && f.Val != "" {
				// No method given, so ParseArgs took the site folder as one.
				// Resolve checks it is a folder.
				positional, cmd.method, cmd.badMethod = append(positional, f.Val), "", f.Val
			}
			continue
		case "repo":
			cmd.Repo, cmd.repoGiven = f.Val, true
			continue
		case "preview-publish":
			cmd.Options.Preview = true
		case "p", "publish":
			cmd.Options.Publish = true
		case "y", "yes":
			cmd.Yes = true
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

	if !generate {
		cmd.Site = positional[0]
		switch {
		case drafts:
			cmd.Mode = PublishDrafts
		case deploying:
			cmd.Mode = Deploy
		case cmd.methodGiven:
			cmd.Mode = SetupDeployment
		}
		return cmd, nil
	}

	// content/<name> makes a page, content/<name>/<post> a dated post,
	// both inside the site folder.
	parts := strings.Split(filepath.ToSlash(filepath.Clean(cmd.GenPath)), "/")
	isPage := parts[0] == "content" && len(parts) == 2
	isPost := parts[0] == "content" && len(parts) == 3
	if !isPage && !isPost {
		cmd.Mode = GenerateSite
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
		return cmd, nil
	}

	if len(sites) == 0 {
		return cmd, fmt.Errorf("missing site folder to create %s in", cmd.GenPath)
	}
	cmd.Site, cmd.collection = sites[0], parts[1]
	if isPage {
		cmd.Mode = GeneratePage
		cmd.GenPath = filepath.Join(cmd.Site, cmd.GenPath)
	} else {
		cmd.Mode, cmd.PostName = GeneratePost, parts[2]
	}
	return cmd, nil
}

// Resolve checks cmd (from Parse) against the disk, and asks on t for
// anything it needs that wasn't given. A generated post is dated now.
func Resolve(cmd Command, t Terminal, now time.Time) (Command, error) {
	if cmd.badMethod != "" {
		if info, err := os.Stat(cmd.badMethod); err != nil || !info.IsDir() {
			return cmd, fmt.Errorf("unknown deployment method %q: use github-pages or none", cmd.badMethod)
		}
	}

	// chooseMethod sets the deployment method and repo from the flags,
	// asking for any not given if ask is set.
	chooseMethod := func(ask bool) {
		cmd.Method = deployment.Methods[cmd.method]
		if cmd.method == "" && ask {
			cmd.Method = t.askDeploymentMethod()
		}
		if cmd.Method == deployment.GitHubPages && !cmd.repoGiven && ask {
			cmd.Repo = t.askRepo()
		}
	}

	switch cmd.Mode {
	case Deploy:
		if !cmd.methodGiven {
			existing, repo := deployment.Detect(filepath.Join(cmd.Site, "build"))
			if existing != deployment.None {
				cmd.method = "github-pages"
			}
			if repo != "" && !cmd.repoGiven {
				cmd.Repo, cmd.repoGiven = repo, true
			}
		}
		if cmd.Yes && (cmd.method == "" || (deployment.Methods[cmd.method] == deployment.GitHubPages && !cmd.repoGiven)) {
			return cmd, fmt.Errorf("%s has no deployment method or GitHub repo set up, and --yes stops them being asked for. Rerun without --yes, or pass --deployment-method github-pages --repo <url>.", cmd.Site)
		}
		chooseMethod(true)
	case SetupDeployment:
		chooseMethod(true)
	case GenerateSite:
		// Check before prompting so the answers aren't wasted.
		if _, err := os.Stat(cmd.GenPath); err == nil {
			return cmd, fmt.Errorf("%s already exists", cmd.GenPath)
		}
		chooseMethod(!cmd.Yes)
	case GeneratePage, GeneratePost:
		if info, err := os.Stat(cmd.Site); err != nil || !info.IsDir() {
			return cmd, fmt.Errorf("site folder %s does not exist. Create it first with `gho-static -g %s`, then rerun this command.", cmd.Site, cmd.Site)
		}
		if cmd.Mode == GeneratePost {
			cmd.GenPath = filepath.Join(cmd.Site, "content", cmd.collection, generator.PostFolder(cmd.PostName, now))
		}
		// Offer a menu link for a new first-level folder, whether it's the
		// page itself or the parent of a new post.
		if _, err := os.Stat(filepath.Join(cmd.Site, "content", cmd.collection)); os.IsNotExist(err) {
			// A new post's parent is a new collection, so check its name first.
			if cmd.Mode == GeneratePost && !cmd.Yes {
				if !t.Interactive {
					return cmd, fmt.Errorf("%s would add a new collection, %s, and there's no one to ask whether that's intended. Rerun with --yes to add it.", cmd.GenPath, cmd.collection)
				}
				if !t.askNewCollection(cmd.collection) {
					return cmd, fmt.Errorf("%s wasn't created.", cmd.GenPath)
				}
			}
			if cmd.Yes || (t.Interactive && t.askNavLink(cmd.collection)) {
				cmd.NavLink = cmd.collection
			}
		}
	}
	return cmd, nil
}
