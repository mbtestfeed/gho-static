# Contributing to Gho-Static

Thanks for wanting to help. Bug reports, suggestions and pull requests are all welcome.

## Code of conduct

Everyone contributing changes or raising issues is asked to act respectfully and professionally. Discrimination on the basis of sex, gender, sexuality, race, age, disability, religion, or other personal factors will not be tolerated - unless your position is one of intolerance. Judgement of this is up to the maintainer(s).

## Before you start

Gho-Static is intentionally simple: a static site generator for sites without interactive content, with no JavaScript by default. Changes that keep it that way are the most likely to be accepted. If you're planning something big, open an issue first to talk it through, so you don't spend time on something that might not fit.

## Reporting bugs and suggesting features

Use the [issues page](https://github.com/mbtestfeed/gho-static/issues). For bugs, include:

- what you ran, and the full output
- what you expected to happen
- your operating system, and the Gho-Static version (or commit, if running from source)

## Making changes

1. Fork the repo and create a branch for your change.
2. Keep each pull request to one change, and keep it as small as it can be while still readable.
3. Match the style of the surrounding code, and run `gofmt` on any Go you change.
4. Add or update tests for what you change, and check they pass with `go test ./...`.
5. Keep paths and URLs working on Windows as well as macOS and Linux: use `filepath` for paths on disk and `path` for slash-separated ones.
6. Update `README.md` if your change affects how Gho-Static is used.

Please don't remove or change the "Built with" attribution in the footer.

## Licence

By contributing, you agree that your contributions are licensed under the project's [MIT License](LICENSE.md).
