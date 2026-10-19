This page is styled with the **%[1]s** theme, whichever theme the rest of the site uses. The content below shows how it looks.

<label class="mode-toggle"><input type="checkbox" id="mode-toggle"> Switch light/dark mode</label>

The switch shows this page in the opposite of your device's light or dark mode, until you leave or reload the page.

## Text

A paragraph of text, with **bold**, *italic*, `inline code` and [a link](https://github.com/mbtestfeed/gho-static). Paragraphs follow on from each other, with space between them.

This is a second paragraph, long enough to wrap onto more than one line on most screens, so you can see how the theme handles line height and the width of the content.

### A smaller heading

Headings go down to smaller sizes, for sections within sections.

## Lists

- An item in a bulleted list
- Another item
- A third item

1. The first step in a numbered list
2. The second step
3. The third step

## Quote

> A quote, set apart from the text around it.

## Code

```
gho-static --use-theme %[1]s mysite
```

## Table

| Setting | Where it's set |
| --- | --- |
| Colours and font | `themes/%[1]s/theme_preferences.yaml` |
| Site name and menu | `settings.yaml` |

## Image

Like the posts in the gallery, a page can show images from its own folder.

![A kitten](./%[2]s)

%[3]s
## Gallery

This is how the gallery page looks with this theme, with a tile for each post.

%[4]s