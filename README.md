# Gho-Static

> [!WARNING]
> Gho-Static is still under active development, and a lot is likely to change. Use it at your own risk.

Gho-Static is a simple static site generator, designed to make deploying easy. You write your pages in markdown, style them with CSS, and Gho-Static builds them into a plain HTML site you can open locally or publish to GitHub Pages with a single command.

Gho-Static is intentionally simple. It's for making and deploying websites without interactive content, such as brochureware, blogs or portfolios. By design, it doesn't use any JavaScript by default. You're welcome to add your own JavaScript to pages for more advanced features, if you can write it yourself, but that's at your own risk. If Gho-Static doesn't do what you need, you're welcome to suggest or make changes on [GitHub](https://github.com/mbtestfeed/gho-static), but [it may just not be for you](https://www.merriam-webster.com/slang/bean-soup-theory), and that's okay.

<!-- toc -->

## Requirements

- [git](https://git-scm.com/downloads) installed and available from the command line.
- A [GitHub](https://github.com) account you can push to from the command line, with authentication (an SSH key, or the [GitHub CLI](https://cli.github.com)) already set up.
- A basic knowledge of [markdown](https://www.markdownguide.org/basic-syntax/) to write your pages, and of CSS if you want to change the styling of your generated site.
- [Go](https://go.dev/dl/) 1.27 or later, only if you're running Gho-Static from its source code.

## Installation

### Download a published binary

> [!NOTE]
> There are no published binaries yet, so Gho-Static can't be installed this way. Run it from its source code instead (see below).

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

This asks how you'd like to deploy the site, then creates a `mysite` folder with a starter site in it, and builds it into `mysite/build`. Basic site settings can be edited at any time in `mysite/settings.yaml`. Add `--yes` to skip the question, or `--clean` to start from a blank site without the example content.

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

If you don't want a blogs section, delete the `content/blogs` folder, and remove `blogs` from `nav_links` in `settings.yaml` to stop it being warned about.

### Build and deploy

```
gho-static mysite
gho-static --drafts mysite
gho-static --deploy mysite
```

The first command rebuilds the site after you edit it. The second goes through your drafts, asking which ones to publish. The third builds the site for publishing and deploys it.

When deploying to GitHub Pages, your site is served at `<username>.github.io`, and any of your other repos with Pages turned on is served at `<username>.github.io/<repo>`. If one of your content folders shares its name with such a repo, the repo's site takes over that path, and your page can't be reached. To fix this, either rename the folder, rename the repo, or turn Pages off on the repo.

When deploying to Cloudflare Pages, your site is served at `<project>.pages.dev`, and the project is created the first time you deploy. Set the `account_id` and `project` settings (and optionally `domain`) with `--deploy-setting` or when asked, and give the API token in the `GHO_STATIC_CLOUDFLARE_PAGES_API_TOKEN` environment variable. Create the token with the Cloudflare Pages: Edit permission at https://dash.cloudflare.com/profile/api-tokens. A domain bought elsewhere is linked to the project for you, but you still need to add a CNAME record pointing it at `<project>.pages.dev` with your domain's DNS provider. A root domain, such as `example.com`, needs its DNS moved to Cloudflare instead. Only changed files are uploaded on each deploy.

```
GHO_STATIC_CLOUDFLARE_PAGES_API_TOKEN=<token> gho-static --deployment-method cloudflare-pages --deploy-setting account_id=<id> --deploy-setting project=mysite --deploy-setting domain=blog.example.com mysite
```

## Flags

- `-h, --help, -u, --usage`: print the usage text and exit.
- `-g, --generate <path>`: create a new site at `<path>`. If `<path>` is `content/<name>`, adds a draft page to an existing site instead. If it's `content/<name>/<post>`, adds a dated draft post. If it's `theme_templates/<name>`, copies the site's currently selected theme into a new custom theme (see Themes, below).
- `-c, --clean`: with `-g`, generate markdown files with only their front matter, and a new site without the example content.
- `-y, --yes`: with `-g`, don't ask for a deployment method. With `--deploy`, fail instead of asking for anything that isn't set up.
- `--preview-publish`: build without drafts and unreleased pages, as publishing does, but with links that still work locally. Otherwise they're built, with a warning on each.
- `-p, --publish`: build for publishing. Each page is written as `folder/index.html`, and drafts and unreleased pages are left out. Can't be used with `--preview-publish`.
- `--drafts`: go through each draft page, asking whether to publish it. With `--yes`, publishes them all.
- `--deploy`: build for publishing and deploy, using the deployment method and settings saved in `settings.yaml`, or asking for any that aren't.
- `--deployment-method <github-pages|cloudflare-pages|none>`: set up the site's build folder for deploying, and save the method in `settings.yaml`.
- `--deploy-setting <name=value>`: a setting for the deployment method, e.g. `username=<name>` for GitHub Pages, used instead of (and saved over) the one in `settings.yaml`. Can be given more than once. Secret settings, such as API keys, are never saved: give them with this, in a `GHO_STATIC_<METHOD>_<SETTING>` environment variable, or when asked.
- `--use-theme <name>`: switch the site to the theme `<name>` and rebuild it, downloading the theme first if the site doesn't have it. See Themes, below.
- `--get-all-themes`: download every Gho-Static theme into `theme_templates/gho_static_themes/`, replacing any already there. Custom themes are left alone. Can also be used with `-g`, to download them into the new site.
- `--list-themes`: list the themes that can be downloaded and the site's own, marking which are downloaded, which are custom, and which one the site uses.

Single-letter flags can be combined, e.g. `-cy`. Run `gho-static --help` for the full usage text.

## Editing your site

### Site layout

```
mysite/
  settings.yaml         site name, menu links, theme, sidebars, deployment
  index.md              the home page
  index.html            the page layout every page is built into
  content/              your pages and posts
  structure/            the header, footer and sidebars' HTML
  theme_templates/      the site's look: each theme's CSS, colours and font
    gho_static_themes/  themes from Gho-Static (don't edit these)
    custom_themes/      your own themes
  static/               files copied as they are into the build
  build/                the built site (don't edit this, it's replaced on each build)
```

### Settings

`settings.yaml` holds the site's settings: its name, the menu links (`nav_links`, as content folder names in menu order), its `theme` (see below), the logo and favicon, the default author for new pages, and the intro sidebar's image, text and social links. Rebuild the site after changing it. Its `deployment` section holds how the site is deployed: a `method` (`github-pages`, `cloudflare-pages` or `none`) and that method's settings, such as GitHub Pages' `username`, whose `<username>.github.io` repo the site is pushed to, or Cloudflare Pages' `account_id`, `project` and `domain`. It's saved when the site is generated or its deployment is set up, and used by `--deploy`.

### Themes

A theme is a folder in one of two places, and `theme` in `settings.yaml` names the one the site uses:

- `theme_templates/gho_static_themes/`: themes from Gho-Static itself. Don't edit these, as `--get-all-themes` replaces them.
- `theme_templates/custom_themes/`: your own themes, which `--get-all-themes` never changes. If a custom theme has the same name as a Gho-Static one, the custom one is used.

Only the chosen theme's CSS is built into the site. New sites come with `default_developer_ugly`, which is plain and functional. Others can be downloaded from the [themes folder](https://github.com/mbtestfeed/gho-static/tree/main/theme_templates/gho_static_themes) of this repo, such as `brochureware`, a slightly more modern look (at the time of writing), like you might see on a company's site:

```
gho-static --use-theme brochureware mysite
gho-static --get-all-themes mysite
```

The first switches the site to `brochureware`, downloading it if the site doesn't have it yet. The second downloads every theme into `theme_templates/gho_static_themes/`, replacing any already there, so any changes to their files are lost: keep site-specific CSS in your project files instead (see below). Themes are downloaded with git, over SSH if you're authenticated with GitHub, otherwise anonymously over HTTPS. Theme field names should stay reasonably stable, but they may change between versions in ways that break a theme's compatibility.

Each theme holds:

- `theme.css`, styling the whole site.
- `structure/<part>/<part>.css`, styling the header, footer and each sidebar.
- `theme_preferences.yaml`, with the theme's colours for light and dark mode and its font. The `{{...}}` values in its CSS are filled in from these, so change them here rather than in the CSS.

When building, Gho-Static adds the colours for the showcase pages' light/dark mode toggle to the end of each theme's `theme.css`, setting the same CSS variables the themes use (`--banner-color`, `--text-color` and so on). A theme using those variables gets the toggle without any CSS of its own.

Theme names, in `settings.yaml`, front matter or the theme flags, must be a single folder name. A page whose theme isn't valid is left out of the build, with a warning.

To make your own theme, start from a copy of the site's currently selected theme:

```
gho-static --generate theme_templates/my_theme mysite
```

This copies it into `theme_templates/custom_themes/my_theme`, without the don't-edit warnings at the top of Gho-Static's theme files, and adds a page for it to the themes showcase. Edit its files, then select it with `--use-theme my_theme`. To base it on another theme, select that one first.

A page can be styled with a different theme from the rest of the site by setting `theme: <name>` in its front matter. If the site doesn't have that theme, the page is left out of the build.

New sites (other than clean ones) have a themes showcase collection, in `content/themes`, linked in the menu after the gallery. It lists the themes available when your copy of Gho-Static was built, then the site's custom themes, marked as custom, with a page for each one the site has, styled with that theme. It marks the currently selected theme. It's left out when publishing or deploying.

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
tags: [news, updates]
---

# About

Your content, in markdown.
```

- `title`: the page's title, shown in the browser tab and listings.
- `summary`: shown under the page's link in listings, and used as its description.
- `author`, `published`, `edited`: shown in the byline. Pages without a published date are asked for one when building.
- `draft: true`: shows the page with a DRAFT warning, and leaves it out when publishing, deploying or building with `--preview-publish`.
- `theme`: styles the page with this theme instead of the site's (see Themes, below).
- `tags`: a list of tags, each linking to a page listing everything with that tag.

The body is standard markdown, with tables, fenced code blocks, footnotes and strikethrough. HTML can be used as well. A `<!-- toc -->` line is replaced with a table of contents built from the page's headings. Quotes starting with `[!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]` or `[!CAUTION]` are shown as alerts, as on GitHub.

To include an image in a page or post, put it in the same folder as its markdown file in `content/`, then link to it from the markdown with `![picture caption](./my-image.png)`.

### Blog posts and listings

A folder with an HTML file in it, like `content/blogs/blogs.html`, is a listing page: it lists each of its subfolders' posts, newest first, after an intro from the markdown file in the same folder. Post folders are named `YYYYMMDD_<name>`, each with a `post.md` inside. Posts in the same listing get previous and next links.

### Special collections

Three folder names in `content/` work differently from other collections:

- `content/themes`: the themes showcase (see Themes, above), left out when publishing or deploying. Pages and collections can't be generated with this name.

- `content/gallery`: the gallery page shows each post as a square image tile, using the first image in the post's folder (alphabetically), or a grey placeholder image if it has none. Each gallery post shows its markdown, then every image in its folder, in alphabetical order.
- `content/tags`: the tags page lists every tag used on the site, and is built again as a page for each tag, listing the pages that have it. These pages are made from the tags in your pages' front matter at each build, so the folder holds no posts of its own. Without it, tags are still shown, but don't link anywhere.

### CSS and JavaScript

- The theme's CSS styles the whole site (see Themes, above). Your own CSS can use its colours as variables, e.g. `color: var(--banner-color)`.
- Each page's folder has a `.css` and `.js` file named after it, e.g. `content/about/about.css`. These only apply to the pages in that folder, so you can style one page without affecting the rest.
- The header, footer and sidebars each have their own HTML in `structure/`, and their CSS in the theme. Turn the intro sidebar on or off with `show_intro_sidebar` in `settings.yaml`. The recent activity sidebar lists the newest posts from the collections in `collections_displayed_in_recent_sidebar` (`gallery` and `blogs` by default), and is left out if that list is empty or those collections have no posts to show.
- `index.js` is linked from every page, as is a `.js` file beside each structure template, e.g. `structure/header/header.js`, if you add one.
- Files in `static/` are copied into `build/static` as they are, for fonts, downloads or images used on several pages.

Empty CSS and JavaScript files are left out of the build, so there's no need to delete them.

## More

See [mbtestfeed.github.io](https://mbtestfeed.github.io) for more documentation, and report bugs or request features on the [issues page](https://github.com/mbtestfeed/gho-static/issues). See [CONTRIBUTING.md](https://github.com/mbtestfeed/gho-static/blob/main/CONTRIBUTING.md) to contribute changes.

Gho-Static is free to use for any purpose, at your own risk, under the [MIT License](https://github.com/mbtestfeed/gho-static/blob/main/LICENSE.md). If you build a site with it, please keep the "Built with" link in its footer.
