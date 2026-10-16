package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gho-static/cli"
	"gho-static/deployment"
	"gho-static/generator"
	"gho-static/gho_static"
	"gho-static/utils/output"
)

// img holds the images copied into each generated site's static/img folder.
//
//go:embed img
var img embed.FS

// readme is the home page of each generated site, other than clean ones.
//
//go:embed README.md
var readme string

// fileLink returns a file:// URL for path.
func fileLink(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// Windows paths (C:\...) need a leading slash to form file:///C:/...
	link := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}
	return link.String()
}

// app runs gho-static's commands, asking questions and writing messages on
// term, as of now.
type app struct {
	term cli.Terminal
	now  time.Time
}

// build builds site into site/build, first asking for a published date
// (defaulting to today) for each listed page without one, unless publishing
// (which skips them instead). It returns the build version.
func (a app) build(site string, opts gho_static.Options) (string, error) {
	for !opts.Publish {
		pages, err := gho_static.UndatedPages(site)
		if err != nil {
			return "", err
		}
		if len(pages) == 0 {
			break
		}
		// Dating renames the page's folder, so look again after each one.
		if err := generator.DatePost(pages[0], a.term.AskDate(pages[0], a.now)); err != nil {
			return "", err
		}
	}
	return gho_static.Build(site, opts)
}

// printVersion prints the build version.
func (a app) printVersion(version string) {
	a.term.Infof("Build version: %s\n", version)
}

// publishDrafts asks whether to publish each of site's draft pages,
// publishing them all if yes is set, or none if no one can answer.
func (a app) publishDrafts(site string, yes bool) error {
	pages, err := gho_static.DraftPages(site)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		a.term.Infof("There are no draft pages.\n")
		return nil
	}
	if !yes && !a.term.Interactive {
		return fmt.Errorf("%d draft pages weren't published, as there's no one to ask. Rerun with --yes to publish them all.", len(pages))
	}
	for _, page := range pages {
		if !yes && !a.term.AskToPublishDraft(page) {
			continue
		}
		if err := generator.PublishDraft(page); err != nil {
			return err
		}
		a.term.Infof("Published %s\n", page)
	}
	return nil
}

func main() {
	a := app{term: cli.Stdio(), now: time.Now()}
	if err := a.run(os.Args[1:]); err != nil {
		fmt.Fprintln(a.term.Err, err)
		os.Exit(1)
	}
}

// run does what args (without the program name) ask, asking for anything
// missing.
func (a app) run(args []string) error {
	term, now := a.term, a.now
	cmd, err := cli.Parse(args)
	if err == nil {
		cmd, err = cli.Resolve(cmd, term, now)
	}
	if err != nil {
		return err
	}
	cmd.Options.Output = term.Output

	// Added before generating, so the page isn't made if its menu link can't
	// be added.
	prefsPath := filepath.Join(cmd.Site, "preferences.yaml")
	navLinkAdded := false
	if cmd.NavLink != "" {
		if navLinkAdded, err = gho_static.AddToList(prefsPath, "nav_links", cmd.NavLink); err != nil {
			return fmt.Errorf("%s wasn't generated, as its menu link couldn't be added: %w", cmd.GenPath, err)
		}
	}

	var version string
	switch cmd.Mode {
	case cli.Build:
		version, err = a.build(cmd.Site, cmd.Options)
	case cli.GenerateSite:
		// Checked first, so nothing is generated if deployment isn't ready.
		if err := cmd.Deployer.Check(); err != nil {
			return err
		}
		images, _ := fs.Sub(img, "img")
		err = generator.Generate(cmd.GenPath, cmd.Prefs, images, readme, cmd.Clean)
	case cli.GeneratePage, cli.GeneratePost:
		var prefs gho_static.Preferences
		if prefs, err = gho_static.ReadPreferences(prefsPath); err != nil {
			break
		}
		if cmd.Mode == cli.GeneratePage {
			err = generator.GeneratePage(cmd.GenPath, prefs.DefaultAuthor, cmd.Clean, now)
		} else {
			err = generator.GeneratePost(cmd.GenPath, cmd.PostName, prefs.DefaultAuthor, cmd.Clean, now)
		}
	case cli.SetupDeployment:
		err = cmd.Deployer.Setup(filepath.Join(cmd.Site, "build"))
		if err == nil {
			err = gho_static.SetDeployment(prefsPath, cmd.Deployment)
		}
		if err == nil {
			version, err = a.build(cmd.Site, cmd.Options)
		}
		if err != nil {
			return err
		}
		a.printVersion(version)
		return nil
	case cli.PublishDrafts:
		return a.publishDrafts(cmd.Site, cmd.Yes)
	case cli.Deploy:
		if version, err = deployment.Deploy(cmd.Site, cmd.Deployer, cmd.Options); err != nil {
			return err
		}
		if err := gho_static.SetDeployment(prefsPath, cmd.Deployment); err != nil {
			return fmt.Errorf("%s was deployed, but its deployment settings couldn't be saved: %w", cmd.Site, err)
		}
		term.Infof("Deployed %s.\n", cmd.Site)
		a.printVersion(version)
		return nil
	}
	if cmd.Mode != cli.Build && os.IsExist(err) {
		err = fmt.Errorf("%s already exists", cmd.GenPath)
	}
	if err != nil {
		// Undo the menu link added for the page that wasn't made.
		if navLinkAdded {
			if rmErr := gho_static.RemoveFromList(prefsPath, "nav_links", cmd.NavLink); rmErr != nil {
				err = fmt.Errorf("%w\n%s is still in nav_links, as removing it failed: %v", err, cmd.NavLink, rmErr)
			}
		}
		return err
	}
	if cmd.Mode == cli.Build {
		fmt.Fprintf(term.Out, "Built %s. You can view it at:\n    %s\n", cmd.Site, output.Link+fileLink(filepath.Join(cmd.Site, "build", "index.html"))+output.Reset)
		a.printVersion(version)
		return nil
	}

	if cmd.Mode == cli.GenerateSite {
		// Set up (pulling any existing repo) before building into it.
		if err := cmd.Deployer.Setup(filepath.Join(cmd.GenPath, "build")); err != nil {
			return fmt.Errorf("%s was generated, but its build folder couldn't be set up for deployment: %w", cmd.GenPath, err)
		}
		if version, err = a.build(cmd.GenPath, cmd.Options); err != nil {
			return err
		}
		fmt.Fprint(term.Out, output.Grey+cli.Usage+output.Reset+"\n")
		a.printVersion(version)
		kind := "default static site demo"
		if cmd.Clean {
			kind = "clean static site"
		}
		fmt.Fprintf(term.Out, "Congrats! You now have a %s, built locally. You can view what this looks like at:\n    %s\n", kind, output.Link+fileLink(filepath.Join(cmd.GenPath, "build", "index.html"))+output.Reset)
		fmt.Fprintf(term.Out, "Basic site preferences can be edited at any time in %s.\n", filepath.Join(cmd.GenPath, "preferences.yaml"))
		return nil
	}

	if version, err = a.build(cmd.Site, cmd.Options); err != nil {
		return err
	}
	src := filepath.Join(cmd.GenPath, "post.md")
	if cmd.Mode == cli.GeneratePage {
		src = filepath.Join(cmd.GenPath, filepath.Base(cmd.GenPath)+".md")
	}
	file, _, err := gho_static.PagePath(src, filepath.Join(cmd.Site, "content"), cmd.Options.Publish)
	if err != nil {
		return err
	}
	fmt.Fprintf(term.Out, "Generated %s. You can view it at:\n    %s\n", cmd.GenPath, output.Link+fileLink(filepath.Join(cmd.Site, "build", filepath.FromSlash(file)))+output.Reset)
	a.printVersion(version)
	return nil
}
