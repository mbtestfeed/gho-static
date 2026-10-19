# Themes

This is a showcase of the themes available for Gho-Static. Each downloaded theme has its own page here, styled with that theme rather than the rest of the site's, with some example content to show how it looks.

Gho-Static only comes with the `default_developer_ugly` theme downloaded. Themes that aren't in this site's `themes/` folder are marked as not available locally, and have no page here until they're downloaded.

To see every theme that can be downloaded now, and which are already in `themes/`, run:

```
gho-static --list-themes mysite
```

To download a theme and switch the site to it, or to download every theme, run:

```
gho-static --use-theme <name> mysite
gho-static --get-all-themes mysite
```

This collection is left out when publishing or deploying, unless you add `--publish-with-themes`.

These were the themes available when this version of Gho-Static was built:
