package main

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gho-static/cli"
	"gho-static/deployment"
	"gho-static/generator"
	"gho-static/gho_static"
)

// img holds the images copied into each generated site's static/img folder.
//
//go:embed img
var img embed.FS

// fileLink returns a file:// URL for path.
func fileLink(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// Windows paths (C:\...) need a leading slash to form file:///C:/...
	link := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}
	return link.String()
}

// fail prints err and exits.
func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// build builds site into out, first asking for a published date for each
// listed page without one, unless publishing (which skips them instead). It
// returns the build version.
func build(site, out string, opts gho_static.Options) (string, error) {
	r := bufio.NewReader(os.Stdin)
	for !opts.Publish {
		pages, err := gho_static.UndatedPages(site)
		if err != nil {
			return "", err
		}
		if len(pages) == 0 {
			break
		}
		// Dating renames the page's folder, so look again after each one.
		if err := generator.DatePost(pages[0], cli.AskDate(r, pages[0])); err != nil {
			return "", err
		}
	}
	return gho_static.Build(site, out, opts)
}

// printVersion prints the build version.
func printVersion(version string) {
	fmt.Println("Build version: " + version)
}

// publishDrafts asks whether to publish each of site's draft pages,
// publishing them all if yes is set, or none if no one can answer.
func publishDrafts(site string, yes bool) error {
	pages, err := gho_static.DraftPages(site)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		fmt.Println("There are no draft pages.")
		return nil
	}
	if !yes && !cli.Interactive() {
		return fmt.Errorf("%d draft pages weren't published, as there's no one to ask. Rerun with --yes to publish them all.", len(pages))
	}
	r := bufio.NewReader(os.Stdin)
	for _, page := range pages {
		if !yes && !cli.AskToPublishDraft(r, page) {
			continue
		}
		if err := generator.PublishDraft(page); err != nil {
			return err
		}
		fmt.Println("Published " + page)
	}
	return nil
}

func main() {
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		fail(err)
	}

	// Added before generating, so the page isn't made if its menu link can't
	// be added.
	prefsPath := filepath.Join(cmd.Site, "preferences.yaml")
	navLinkAdded := false
	if cmd.NavLink != "" {
		if navLinkAdded, err = gho_static.AddToList(prefsPath, "nav_links", cmd.NavLink); err != nil {
			fail(fmt.Errorf("%s wasn't generated, as its menu link couldn't be added: %w", cmd.GenPath, err))
		}
	}

	var version string
	switch cmd.Mode {
	case cli.Build:
		version, err = build(cmd.Site, cmd.Site, cmd.Options)
	case cli.GenerateSite:
		// Checked first, so nothing is generated if the repo isn't ready.
		if err := deployment.Check(cmd.Method, cmd.Repo); err != nil {
			fail(err)
		}
		images, _ := fs.Sub(img, "img")
		err = generator.Generate(cmd.GenPath, cmd.Prefs, images)
	case cli.GeneratePage, cli.GeneratePost:
		var prefs gho_static.Preferences
		if prefs, err = gho_static.ReadPreferences(prefsPath); err != nil {
			break
		}
		if cmd.Mode == cli.GeneratePage {
			err = generator.GeneratePage(cmd.GenPath, prefs.DefaultAuthor)
		} else {
			err = generator.GeneratePost(cmd.GenPath, cmd.PostName, prefs.DefaultAuthor)
		}
	case cli.SetupDeployment:
		err = deployment.Setup(filepath.Join(cmd.Site, "build"), cmd.Method, cmd.Repo)
		if err == nil {
			version, err = build(cmd.Site, cmd.Site, cmd.Options)
		}
		if err != nil {
			fail(err)
		}
		printVersion(version)
		return
	case cli.PublishDrafts:
		if err := publishDrafts(cmd.Site, cmd.Yes); err != nil {
			fail(err)
		}
		return
	case cli.Deploy:
		if version, err = deployment.Deploy(cmd.Site, cmd.Method, cmd.Repo); err != nil {
			fail(err)
		}
		fmt.Printf("Deployed %s (it may take a minute or two to update on GitHub).\n", cmd.Site)
		printVersion(version)
		return
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
		fail(err)
	}
	if cmd.Mode == cli.Build {
		fmt.Printf("Built %s. You can view it at:\n    %s\n", cmd.Site, cli.Link+fileLink(filepath.Join(cmd.Site, "build", "index.html"))+cli.Reset)
		printVersion(version)
		return
	}

	if cmd.Mode == cli.GenerateSite {
		// Set up (pulling any existing repo) before building into it.
		if err := deployment.Setup(filepath.Join(cmd.GenPath, "build"), cmd.Method, cmd.Repo); err != nil {
			fail(fmt.Errorf("%s was generated, but its build folder couldn't be set up for deployment: %w", cmd.GenPath, err))
		}
		// Build inside the new folder so an existing ./build isn't replaced.
		if version, err = build(cmd.GenPath, cmd.GenPath, cmd.Options); err != nil {
			fail(err)
		}
		fmt.Print(cli.Grey + cli.Usage + cli.Reset + "\n")
		printVersion(version)
		fmt.Printf("Congrats! You now have a default static site demo, built locally. You can view what this looks like at:\n    %s\n", cli.Link+fileLink(filepath.Join(cmd.GenPath, "build", "index.html"))+cli.Reset)
		return
	}

	if version, err = build(cmd.Site, cmd.Site, cmd.Options); err != nil {
		fail(err)
	}
	src := filepath.Join(cmd.GenPath, "post.md")
	if cmd.Mode == cli.GeneratePage {
		src = filepath.Join(cmd.GenPath, filepath.Base(cmd.GenPath)+".md")
	}
	file, _, err := gho_static.PagePath(src, filepath.Join(cmd.Site, "content"), cmd.Options.Publish)
	if err != nil {
		fail(err)
	}
	fmt.Printf("Generated %s. You can view it at:\n    %s\n", cmd.GenPath, cli.Link+fileLink(filepath.Join(cmd.Site, "build", filepath.FromSlash(file)))+cli.Reset)
	printVersion(version)
}
