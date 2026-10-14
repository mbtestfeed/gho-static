package gho_static

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Color is a pair of light and dark mode colors.
type Color struct {
	Light string `yaml:"light"`
	Dark  string `yaml:"dark"`
}

// Preferences are the site wide settings read from preferences.yaml.
type Preferences struct {
	SiteName        string `yaml:"site_name"`
	NavLinks        string `yaml:"nav_links"` // comma separated content folder names, in menu order
	BannerColor     Color  `yaml:"banner_color"`      // header, footer and menu background
	BannerTextColor Color  `yaml:"banner_text_color"` // text and links in the header, footer and menu
	BackgroundColor Color  `yaml:"background_color"`
	TextColor       Color  `yaml:"text_color"`
	Link            Color  `yaml:"link"`
	LinkClicked     Color  `yaml:"link_clicked"`
	Font            string `yaml:"font"`
	Logo            string `yaml:"logo"`    // header image, relative to the site folder
	Favicon         string `yaml:"favicon"` // browser tab icon, relative to the site folder
}

// DefaultPreferences are used for any setting missing from preferences.yaml.
var DefaultPreferences = Preferences{
	SiteName:        "Gho-Static",
	NavLinks:        "about,blogs,contact",
	BannerColor:     Color{Light: "#add8e6", Dark: "#add8e6"},
	BannerTextColor: Color{Light: "#000000", Dark: "#000000"},
	BackgroundColor: Color{Light: "#ffffff", Dark: "#000000"},
	TextColor:       Color{Light: "#000000", Dark: "#ffffff"},
	Link:           Color{Light: "#0000ee", Dark: "#8ab4f8"},
	LinkClicked:     Color{Light: "#551a8b", Dark: "#c58af9"},
	Font:           "sans-serif",
	Logo:            "static/img/gho_stephanie_64x64.svg",
	Favicon:         "static/img/favicon.ico",
}

// readPreferences reads the preferences file at path over the defaults. A
// missing file gives the defaults.
func readPreferences(path string) (Preferences, error) {
	prefs := DefaultPreferences
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return prefs, nil
	}
	if err != nil {
		return prefs, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &prefs); err != nil {
		return prefs, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return prefs, nil
}

// fillCSS executes css (named name, for errors) as a text template with
// prefs, filling in any preference fields it references.
func fillCSS(css []byte, name string, prefs Preferences) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(css))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", name, err)
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, prefs); err != nil {
		return nil, fmt.Errorf("failed to render %s: %w", name, err)
	}
	return []byte(sb.String()), nil
}

// navLinkNames returns the content folder names listed in NavLinks.
func (p Preferences) navLinkNames() []string {
	var names []string
	for _, n := range strings.Split(p.NavLinks, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names
}
