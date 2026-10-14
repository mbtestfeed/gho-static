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
// listed page without one, unless publishing (which skips them instead).
func build(site, out string, opts gho_static.Options) error {
	r := bufio.NewReader(os.Stdin)
	for !opts.Publish {
		pages, err := gho_static.UndatedPages(site)
		if err != nil {
			return err
		}
		if len(pages) == 0 {
			break
		}
		// Dating renames the page's folder, so look again after each one.
		if err := generator.DatePost(pages[0], cli.AskDate(r, pages[0])); err != nil {
			return err
		}
	}
	return gho_static.Build(site, out, opts)
}

func main() {
	cmd, err := cli.Parse(os.Args[1:])
	if err != nil {
		fail(err)
	}

	switch cmd.Mode {
	case cli.Build:
		err = build(cmd.Site, "", cmd.Options)
	case cli.GenerateSite:
		// Checked first, so nothing is generated if the repo isn't ready.
		if err := deployment.Check(cmd.Method, cmd.Repo); err != nil {
			fail(err)
		}
		images, _ := fs.Sub(img, "img")
		err = generator.Generate(cmd.GenPath, cmd.Prefs, images)
	case cli.GeneratePage:
		err = generator.GeneratePage(cmd.GenPath)
	case cli.GeneratePost:
		err = generator.GeneratePost(cmd.GenPath, cmd.PostName)
	case cli.SetupDeployment:
		err = deployment.Setup(filepath.Join(cmd.Site, "build"), cmd.Method, cmd.Repo)
		if err == nil {
			err = build(cmd.Site, cmd.Site, cmd.Options)
		}
		if err != nil {
			fail(err)
		}
		return
	case cli.Deploy:
		if err := deployment.Deploy(cmd.Site, cmd.Method, cmd.Repo); err != nil {
			fail(err)
		}
		fmt.Printf("Deployed %s.\n", cmd.Site)
		return
	}
	if cmd.Mode != cli.Build && os.IsExist(err) {
		err = fmt.Errorf("%s already exists", cmd.GenPath)
	}
	if err != nil {
		fail(err)
	}
	if cmd.Mode == cli.Build {
		return
	}

	if cmd.Mode == cli.GenerateSite {
		// Set up (pulling any existing repo) before building into it.
		if err := deployment.Setup(filepath.Join(cmd.GenPath, "build"), cmd.Method, cmd.Repo); err != nil {
			fail(fmt.Errorf("%s was generated, but its build folder couldn't be set up for deployment: %w", cmd.GenPath, err))
		}
		// Build inside the new folder so an existing ./build isn't replaced.
		if err := build(cmd.GenPath, cmd.GenPath, cmd.Options); err != nil {
			fail(err)
		}
		fmt.Print(cli.Grey + cli.Usage + cli.Reset + "\n")
		fmt.Printf("Congrats! You now have a default static site demo, built locally. You can view what this looks like at:\n    %s\n", cli.Link+fileLink(filepath.Join(cmd.GenPath, "build", "index.html"))+cli.Reset)
		return
	}

	if err := build(cmd.Site, cmd.Site, cmd.Options); err != nil {
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
}
