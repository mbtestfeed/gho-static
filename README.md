# Gho-Static

Gho-Static is a simple static site generator, designed to make deploying easy. You write your pages in markdown, style them with CSS, and Gho-Static builds them into a plain HTML site you can open locally or publish to GitHub Pages with a single command.

## Requirements

- [git](https://git-scm.com/downloads) installed and available from the command line.
- A [GitHub](https://github.com) account you can push to from the command line, with authentication (an SSH key, or the [GitHub CLI](https://cli.github.com)) already set up.
- A basic knowledge of [markdown](https://www.markdownguide.org/basic-syntax/) to write your pages, and of CSS if you want to change the styling of your generated site.
- [Go](https://go.dev/dl/) 1.27 or later, only if you're running Gho-Static from its source code.

## Installation

### Download a published binary

1. Go to the [releases page](https://github.com/mbtestfeed/gho-static/releases) and download the binary for your operating system.
2. On macOS or Linux, make it executable: `chmod +x gho-static`
3. To run it from any folder as just `gho-static`, move it into a folder on your `PATH`. `~/.local/bin` works well, as it doesn't need admin rights:

   ```
   mkdir -p ~/.local/bin
   mv gho-static ~/.local/bin/
   ```

   If `gho-static` isn't found afterwards, add that folder to your `PATH` by adding this line to `~/.bashrc` on Linux, or `~/.zshrc` on macOS:

   ```
   export PATH="$HOME/.local/bin:$PATH"
   ```

   Then open a new terminal. On macOS, if it's blocked as being from an unidentified developer, run `xattr -d com.apple.quarantine ~/.local/bin/gho-static`. You can also skip this step and run it by its path, e.g. `./gho-static`.

#### On Windows

1. Download the Windows binary (`gho-static.exe`) from the [releases page](https://github.com/mbtestfeed/gho-static/releases).
2. Move it into a folder of its own, e.g. `C:\Tools\gho-static`.
3. Open PowerShell or Windows Terminal in that folder and run it:

   ```
   .\gho-static.exe --generate mysite
   ```

   If Windows shows "Windows protected your PC", click **More info**, then **Run anyway**.
4. To run it from any folder as just `gho-static`, add its folder to your `PATH`: search the Start menu for "Edit environment variables for your account", select **Path**, click **Edit**, then **New**, and enter the folder, e.g. `C:\Tools\gho-static`. Open a new terminal for the change to take effect.

### Run from source

```
git clone https://github.com/mbtestfeed/gho-static.git
cd gho-static
go run main.go --generate ../mysite
```

Use `go run main.go` in place of `gho-static` in any of the commands below. Paths are relative to the `gho-static` folder, so `../mysite` keeps your site outside of it.

## Quickstart

### Create your first site

```
gho-static --generate mysite
```

This asks how you'd like to deploy the site, then creates a `mysite` folder with a starter site in it, and builds it into `mysite/build`. Basic site preferences can be edited at any time in `mysite/preferences.yaml`. Add `--yes` to skip the question, or `--clean` to start from a blank site without the example content.

### Add a new page

```
gho-static --generate content/projects mysite
```

This adds a draft page at `mysite/content/projects/projects.md`, offers to add it to the menu, and rebuilds the site.

### Add a new blog post

```
gho-static --generate content/blogs/my_first_post mysite
```

This adds a draft post at `mysite/content/blogs/YYYYMMDD_my_first_post/post.md`, dated today, and rebuilds the site.

New pages and posts only have their front matter and a heading. To add your content, edit the new markdown file, then rebuild the site (see below) to see your changes.

### Add a new collection

Blogs aren't the only collection you can have. To start a new one, generate its first post, e.g.:

```
gho-static --generate content/my_new_collection/my_first_post_in_new_collection mysite
```

This creates `content/my_new_collection` with its own listing page, offers to add it to the menu, and adds the post inside it.

If you don't want a blogs section, delete the `content/blogs` folder, and remove `blogs` from `nav_links` in `preferences.yaml` to stop it being warned about.
### Build and deploy

```
gho-static mysite
gho-static --drafts mysite
gho-static --deploy mysite
```

The first command rebuilds the site after you edit it. The second goes through your drafts, asking which ones to publish. The third builds the site for publishing and deploys it.

When deploying to GitHub Pages, your site is served at `<username>.github.io`, and any of your other repos with Pages turned on is served at `<username>.github.io/<repo>`. If one of your content folders shares its name with such a repo, the repo's site takes over that path, and your page can't be reached. To fix this, either rename the folder, rename the repo, or turn Pages off on the repo.

## Flags

- `-g, --generate <path>`: create a new site at `<path>`. If `<path>` is `content/<name>`, adds a draft page to an existing site instead. If it's `content/<name>/<post>`, adds a dated draft post.
- `-c, --clean`: with `-g`, generate markdown files with only their front matter, and a new site without the example content.
- `-y, --yes`: with `-g`, don't ask for a deployment method. With `--deploy`, fail instead of asking for anything that isn't set up.
- `--preview-publish`: build without drafts and unreleased pages, as publishing does, but with links that still work locally. Otherwise they're built, with a warning on each.
- `-p, --publish`: build for publishing. Each page is written as `folder/index.html`, and drafts and unreleased pages are left out. Can't be used with `--preview-publish`.
- `--drafts`: go through each draft page, asking whether to publish it. With `--yes`, publishes them all.
- `--deploy`: build for publishing and deploy, using the deployment method and settings saved in `preferences.yaml`, or asking for any that aren't.
- `--deployment-method <github-pages|none>`: set up the site's build folder for deploying, and save the method in `preferences.yaml`.
- `--deploy-setting <name=value>`: a setting for the deployment method, e.g. `username=<name>` for GitHub Pages, used instead of (and saved over) the one in `preferences.yaml`. Can be given more than once. Secret settings, such as API keys, are never saved: give them with this, in a `GHO_STATIC_<METHOD>_<SETTING>` environment variable, or when asked.

Single-letter flags can be combined, e.g. `-cy`. Run `gho-static` with no arguments for the full usage text.

## Editing your site

### Site layout

```
mysite/
  preferences.yaml      site name, menu links, colours, fonts, sidebars
  index.md              the home page
  index.css             styles for every page
  index.html            the page layout every page is built into
  content/              your pages and posts
  structure/            the header, footer and sidebars
  static/               files copied as they are into the build
  build/                the built site (don't edit this, it's replaced on each build)
```

### Preferences

`preferences.yaml` holds the site's settings: its name, the menu links (`nav_links`, as content folder names in menu order), colours for light and dark mode, the font, logo and favicon, the default author for new pages, and the intro sidebar's image, text and social links. Rebuild the site after changing it. Its `deployment` section holds how the site is deployed: a `method` (`github-pages` or `none`) and that method's settings, such as GitHub Pages' `username`, whose `<username>.github.io` repo the site is pushed to. It's saved when the site is generated or its deployment is set up, and used by `--deploy`.

### Markdown pages

Each page is a folder in `content/` holding a markdown file named after it, e.g. `content/about/about.md`. Each page starts with front matter:

```
---
title: About
summary: A short description, shown in listings and search results.
author: Your Name
published: 2026-01-31
edited: 2026-02-14
draft: false
toc: false
tags: [news, updates]
---

# About

Your content, in markdown.
```

- `title`: the page's title, shown in the browser tab and listings.
- `summary`: shown under the page's link in listings, and used as its description.
- `author`, `published`, `edited`: shown in the byline. Pages without a published date are asked for one when building.
- `draft: true`: shows the page with a DRAFT warning, and leaves it out when publishing, deploying or building with `--preview-publish`.
- `toc: true`: adds a table of contents built from the page's headings.
- `tags`: a list of tags, each linking to a page listing everything with that tag.

The body is standard markdown, with tables, fenced code blocks, footnotes and strikethrough. HTML can be used as well.

To include an image in a page or post, put it in the same folder as its markdown file in `content/`, then link to it from the markdown with `![picture caption](./my-image.png)`.

### Blog posts and listings

A folder with an HTML file in it, like `content/blogs/blogs.html`, is a listing page: it lists each of its subfolders' posts, newest first, after an intro from the markdown file in the same folder. Post folders are named `YYYYMMDD_<name>`, each with a `post.md` inside. Posts in the same listing get previous and next links.

### Special collections

Two folder names in `content/` are reserved, and work differently from other collections:

- `content/gallery`: the gallery page shows each post as a square image tile, using the first image in the post's folder (alphabetically), or the logo if it has none. Each gallery post shows its markdown, then every image in its folder, in alphabetical order.
- `content/tags`: the tags page lists every tag used on the site, and is built again as a page for each tag, listing the pages that have it. These pages are made from the tags in your pages' front matter at each build, so the folder holds no posts of its own. Without it, tags are still shown, but don't link anywhere.

### CSS and JavaScript

- `index.css` styles the whole site. The `{{...}}` values in it are filled in from `preferences.yaml`, so change colours and fonts there. Your own CSS can use them as variables, e.g. `color: var(--banner-color)`.
- Each page's folder has a `.css` and `.js` file named after it, e.g. `content/about/about.css`. These only apply to the pages in that folder, so you can style one page without affecting the rest.
- The header, footer and sidebars each have their own HTML and CSS in `structure/`. Turn the sidebars on or off with `show_intro_sidebar` and `show_recent_activity_sidebar` in `preferences.yaml`.
- Files in `static/` are copied into `build/static` as they are, for fonts, downloads or images used on several pages.

Empty CSS and JavaScript files are left out of the build, so there's no need to delete them.

## More

See [mbtestfeed.github.io](https://mbtestfeed.github.io) for more documentation, and report bugs or request features on the [issues page](https://github.com/mbtestfeed/gho-static/issues).
