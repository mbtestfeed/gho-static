package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gho-static/deployment"
	"gho-static/gho_static"
	"gho-static/utils"
	"gho-static/utils/dates"
)

const Usage = `Usage: gho-static [flags] <site folder>

Builds the site in <site folder> into <site folder>/build.

Flags:
  -h, --help, -u, --usage Print this usage text and exit.
  -g, --generate <path>   Create a new folder at <path>, fill it with a default
                          starter site and build it into <path>/build. Fails if
                          <path> already exists. If <path> is content/<name>,
                          instead adds a draft page <name>.md in a new
                          content/<name> folder inside <site folder>. If <path>
                          is content/<name>/<post>, adds a draft post.md in a
                          new content/<name>/YYYYMMDD_<post> folder inside
                          <site folder>, e.g. gho-static --generate content/news mysite
                          If <path> is theme_templates/<name>, copies the
                          site's currently selected theme into a new custom
                          theme in theme_templates/custom_themes/<name>.
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
                          it, using the deployment method and settings saved
                          in its settings.yaml, or asking for any that
                          aren't. For GitHub Pages, commits the build folder
                          and pushes it. For Cloudflare Pages, uploads the
                          files that changed.
  --deployment-method <github-pages|cloudflare-pages|none>
                          Set up <site folder>/build for publishing, building
                          it first if it doesn't exist, and save the method in
                          settings.yaml. github-pages makes it a git repo,
                          with the <username>.github.io repo of the username
                          saved in settings.yaml as its remote.
                          cloudflare-pages uses the account_id, project and
                          optional domain settings, creating the project and
                          linking the domain if needed, with the API token in
                          GHO_STATIC_CLOUDFLARE_PAGES_API_TOKEN. Asks for
                          the method (and its settings) if not given. With -g,
                          used instead of asking.
  --deploy-setting <name=value>
                          A setting for the deployment method, e.g.
                          username=<name> for github-pages, used instead of
                          the one saved in settings.yaml (and saved in its
                          place).
                          Can be given more than once. Secret settings, such
                          as API keys, are never saved: give them with this,
                          in a GHO_STATIC_<METHOD>_<SETTING> environment
                          variable (e.g. GHO_STATIC_NETLIFY_TOKEN), or when
                          asked.
  --use-theme <name>      Switch <site folder> to the theme <name>, setting
                          theme in settings.yaml, and rebuild it. Downloads
                          the theme from the Gho-Static GitHub repo into
                          <site folder>/themes first if it isn't there.
  --get-all-themes        Download every theme in the Gho-Static GitHub repo
                          into <site folder>/themes, replacing any already
                          there. Can also be used with -g, to download them
                          into the new site.
  --publish-with-themes   With --publish, --preview-publish or --deploy, keep
                          the themes showcase collection (content/themes),
                          which is otherwise left out.
  --list-themes           List the themes in the Gho-Static GitHub repo and in
                          <site folder>/themes, marking which are downloaded
                          and which one the site uses.

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
	UseTheme                 // switch Site to Theme, downloading it if missing, then build Site
	GetAllThemes             // download every theme into Site's themes folder
	ListThemes               // list the themes that can be downloaded, and Site's own
	Help                     // print Usage, and do nothing else
	GenerateTheme            // copy Site's current theme into a new custom theme Theme at GenPath, then build Site
)

// Command is the work requested on the command line.
type Command struct {
	Mode     Mode
	Site     string // site folder to build; unset for GenerateSite
	GenPath  string // folder to generate
	PostName string // only set for GeneratePost
	Theme    string // only set for UseTheme and GenerateTheme
	AllThemes bool  // with GenerateSite, also download every theme into the new site (--get-all-themes)
	NavLink  string // content folder to add to nav_links, only set for GeneratePage and GeneratePost
	Yes      bool   // answer yes to every question (--yes)
	Clean    bool   // generate without example content (--clean)
	Options  gho_static.Options
	Prefs    gho_static.Preferences // only set for GenerateSite
	Deployer deployment.Deployer    // only set for GenerateSite, SetupDeployment and Deploy
	// Deployer's method and settings, to save in settings.yaml
	Deployment gho_static.Deployment

	method      string              // --deployment-method value, if it's a method
	badMethod   string              // --deployment-method value that isn't a method, so is taken as the site folder
	methodGiven bool                // --deployment-method was given
	saved       deployment.Settings // saved or detected settings for method
	given       deployment.Settings // settings given with --deploy-setting
	collection  string              // content folder a page or post is generated in
}

// Parse reads args (without the program name) into a Command, without
// looking at the disk or asking anything: Resolve does that.
func Parse(args []string) (Command, error) {
	flags, positional := utils.ParseArgs(args)

	var cmd Command
	generate, deploying, drafts, useTheme, allThemes, listThemes := false, false, false, false, false, false
	for _, f := range flags {
		switch f.Key {
		case "h", "help", "u", "usage":
			return Command{Mode: Help}, nil
		case "g", "generate":
			generate, cmd.GenPath = true, f.Val
			continue
		case "use-theme":
			useTheme, cmd.Theme = true, f.Val
			if !gho_static.ValidThemeName(cmd.Theme) {
				return cmd, fmt.Errorf("--use-theme needs a theme name, e.g. --use-theme brochureware mysite")
			}
			continue
		case "get-all-themes":
			allThemes = true
		case "list-themes":
			listThemes = true
		case "deployment-method":
			cmd.methodGiven, cmd.method = true, f.Val
			if _, ok := deployment.Find(f.Val); !ok && f.Val != "" {
				// No method given, so ParseArgs took the site folder as one.
				// Resolve checks it is a folder.
				positional, cmd.method, cmd.badMethod = append(positional, f.Val), "", f.Val
			}
			continue
		case "deploy-setting":
			name, value, ok := strings.Cut(f.Val, "=")
			if !ok || name == "" {
				return cmd, fmt.Errorf("--deploy-setting needs a name=value setting, e.g. --deploy-setting username=<name>")
			}
			if cmd.given == nil {
				cmd.given = deployment.Settings{}
			}
			cmd.given[name] = value
			continue
		case "preview-publish":
			cmd.Options.Preview = true
		case "publish-with-themes":
			cmd.Options.WithThemes = true
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
			flag := "--" + f.Key
			if len(f.Key) == 1 {
				flag = "-" + f.Key
			}
			return cmd, fmt.Errorf("unknown flag %s. Run gho-static --help to see the flags it takes.", flag)
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

	if generate && (useTheme || listThemes) {
		return cmd, fmt.Errorf("--use-theme and --list-themes can't be used with --generate. Generate the site first, then rerun with just the theme flag.")
	}
	if !generate {
		cmd.Site = positional[0]
		switch {
		case useTheme:
			cmd.Mode = UseTheme
		case allThemes:
			cmd.Mode = GetAllThemes
		case listThemes:
			cmd.Mode = ListThemes
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
	isTheme := parts[0] == gho_static.ThemeTemplatesDir && len(parts) == 2
	if parts[0] == "content" && len(parts) > 3 {
		return cmd, fmt.Errorf("can't create %s: pages can only be nested one level deep, as posts in a collection", cmd.GenPath)
	}
	if !isPage && !isPost && !isTheme {
		cmd.Mode, cmd.AllThemes = GenerateSite, allThemes
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

	if allThemes {
		return cmd, fmt.Errorf("--get-all-themes can't be used when generating a page, post or theme. Run it on its own, e.g. gho-static --get-all-themes mysite")
	}
	if isTheme {
		if !gho_static.ValidThemeName(parts[1]) {
			return cmd, fmt.Errorf("--generate theme_templates/<name> needs a theme name, e.g. --generate theme_templates/my_theme mysite")
		}
		if len(sites) == 0 {
			return cmd, fmt.Errorf("missing site folder to create %s in", cmd.GenPath)
		}
		cmd.Mode, cmd.Site, cmd.Theme = GenerateTheme, sites[0], parts[1]
		cmd.GenPath = filepath.Join(cmd.Site, filepath.FromSlash(gho_static.CustomThemesDir), cmd.Theme)
		return cmd, nil
	}
	if reason := gho_static.ReservedContentName(parts[1], isPost); reason != "" {
		return cmd, fmt.Errorf("can't create %s: %s", cmd.GenPath, reason)
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
			var names []string
			for _, m := range deployment.Methods {
				names = append(names, m.Name)
			}
			return cmd, fmt.Errorf("unknown deployment method %q: use %s", cmd.badMethod, strings.Join(names, " or "))
		}
	}

	// chooseMethod sets the Deployer from the flags and saved settings,
	// asking for the method and any settings it needs that weren't given if
	// ask is set. Settings the method doesn't use are an error.
	chooseMethod := func(ask bool) error {
		m, ok := deployment.Find(cmd.method)
		var askSetting deployment.Ask
		if ask {
			if !ok {
				m = t.askDeploymentMethod()
			}
			askSetting = t.AskSetting
		} else if !ok {
			m, _ = deployment.Find(deployment.Default)
		}
		var saved deployment.Settings
		if m.Name == cmd.method {
			saved = cmd.saved
		}
		if err := m.Validate(saved, filepath.Join(cmd.Site, "settings.yaml")); err != nil {
			return err
		}
		if err := m.Validate(cmd.given, "--deploy-setting"); err != nil {
			return err
		}
		cmd.Deployer = m.New(m.Gather(saved, cmd.given), askSetting, t.Output)
		cmd.Deployment = gho_static.Deployment{Method: m.Name, Settings: m.Saveable(cmd.Deployer.Settings())}
		return nil
	}

	switch cmd.Mode {
	case Deploy, SetupDeployment:
		// The method saved in settings.yaml is used unless one is given,
		// along with its saved settings. Sites set up before they were saved
		// there are detected from their build folder instead.
		prefs, err := gho_static.ReadPreferences(filepath.Join(cmd.Site, "settings.yaml"))
		if err != nil {
			return cmd, err
		}
		saved := prefs.Deployment
		if !cmd.methodGiven {
			cmd.method = saved.Method
			// A site that isn't deployed yet needs a method to deploy with.
			if cmd.method == deployment.Default {
				cmd.method = ""
			}
			if cmd.method == "" {
				if m, s, ok := deployment.Detect(filepath.Join(cmd.Site, "build")); ok {
					cmd.method, saved = m.Name, gho_static.Deployment{Method: m.Name, Settings: s}
				}
			}
		}
		if saved.Method == cmd.method {
			cmd.saved = saved.Settings
		}
		if cmd.Mode == SetupDeployment {
			if err := chooseMethod(true); err != nil {
				return cmd, err
			}
			break
		}
		if cmd.Yes {
			if err := chooseMethod(false); err != nil {
				return cmd, err
			}
			if _, ok := deployment.Find(cmd.method); !ok || cmd.Deployer.CheckPush() != nil {
				return cmd, fmt.Errorf("%s has no deployment method or settings set up, and --yes stops them being asked for. Rerun without --yes, or set them in the deployment section of its settings.yaml or with --deploy-setting.", cmd.Site)
			}
			break
		}
		if err := chooseMethod(true); err != nil {
			return cmd, err
		}
	case GenerateSite:
		// Check before prompting so the answers aren't wasted.
		if _, err := os.Stat(cmd.GenPath); err == nil {
			return cmd, fmt.Errorf("%s already exists", cmd.GenPath)
		}
		if err := chooseMethod(!cmd.Yes); err != nil {
			return cmd, err
		}
		cmd.Prefs.Deployment = cmd.Deployment
	case GeneratePage, GeneratePost:
		if info, err := os.Stat(cmd.Site); err != nil || !info.IsDir() {
			return cmd, fmt.Errorf("site folder %s does not exist. Create it first with `gho-static -g %s`, then rerun this command.", cmd.Site, cmd.Site)
		}
		if cmd.Mode == GeneratePost {
			cmd.GenPath = filepath.Join(cmd.Site, "content", cmd.collection, dates.PostFolder(cmd.PostName, now))
			// Posts only go in collections: folders with a listing page.
			if entries, err := os.ReadDir(filepath.Join(cmd.Site, "content", cmd.collection)); err == nil &&
				!slices.ContainsFunc(entries, func(e os.DirEntry) bool { return !e.IsDir() && utils.HasExt(e.Name(), ".html") }) {
				return cmd, fmt.Errorf("can't create %s: %s is a page, not a collection, so can't hold posts", cmd.GenPath, cmd.collection)
			}
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
	case UseTheme, GetAllThemes, ListThemes, GenerateTheme:
		if info, err := os.Stat(cmd.Site); err != nil || !info.IsDir() {
			return cmd, fmt.Errorf("site folder %s does not exist", cmd.Site)
		}
		if _, ok := gho_static.FindTheme(cmd.Site, cmd.Theme); ok && cmd.Mode == GenerateTheme {
			return cmd, fmt.Errorf("can't create %s: %s already has a theme called %s", cmd.GenPath, cmd.Site, cmd.Theme)
		}
	}
	return cmd, nil
}
