package gho_static

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"gho-static/utils/yaml_edit"
)

// Color is a pair of light and dark mode colors.
type Color struct {
	Light string `yaml:"light"`
	Dark  string `yaml:"dark"`
}

// Preferences are the site wide settings read from settings.yaml.
type Preferences struct {
	SiteName        string `yaml:"site_name"`
	NavLinks        []string `yaml:"nav_links"` // content folder names, in menu order
	Theme           string `yaml:"theme"`   // folder in themes/ holding the site's css and theme preferences
	Logo            string `yaml:"logo"`    // header image, relative to the site folder
	Favicon         string `yaml:"favicon"` // browser tab icon, relative to the site folder
	DefaultAuthor   string `yaml:"default_author"` // added to new markdown pages' front matter, if set
	ShowIntroSidebar       bool   `yaml:"show_intro_sidebar"`
	RecentSidebarCollections []string `yaml:"collections_displayed_in_recent_sidebar"` // content folder names whose posts the recent activity sidebar lists; empty hides it
	IntroSidebarImage      string `yaml:"intro_sidebar_image"` // intro sidebar image, relative to the site folder
	IntroName        string      `yaml:"intro_name"`        // intro sidebar heading
	IntroDescription string      `yaml:"intro_description"` // intro sidebar text
	SocialLinks      SocialLinks `yaml:"social_links"`      // intro sidebar links, as name: url pairs; blank ones aren't shown
	Deployment       Deployment  `yaml:"deployment,omitempty"` // how the build folder is published, saved once it's set up
}

// ThemePreferences are a theme's colours and font, read from its
// theme_preferences.yaml and filled into its css.
type ThemePreferences struct {
	BannerColor     Color  `yaml:"banner_color"`      // header, footer and menu background
	BannerTextColor Color  `yaml:"banner_text_color"` // text and links in the header, footer and menu
	BackgroundColor Color  `yaml:"background_color"`
	TextColor       Color  `yaml:"text_color"`
	Link            Color  `yaml:"link"`
	LinkClicked     Color  `yaml:"link_clicked"`
	TagColor        Color  `yaml:"tag_color"`      // tag pill background
	TagTextColor    Color  `yaml:"tag_text_color"` // tag pill text
	Font            string `yaml:"font"`
}

// Deployment is how a site's build folder is published: a deployment
// method's name, and its settings alongside it.
type Deployment struct {
	Method   string            `yaml:"method"`
	Settings map[string]string `yaml:",inline"`
}

// SocialLink is a named link shown in the intro sidebar.
type SocialLink struct {
	Name string
	URL  string
}

// Href returns the link's URL, with https:// added if it has no scheme.
func (l SocialLink) Href() string {
	if strings.Contains(l.URL, "://") {
		return l.URL
	}
	return "https://" + l.URL
}

// SocialLinks are written in settings.yaml as a mapping of name: url
// pairs, kept in the order given, so any name can be added.
type SocialLinks []SocialLink

func (l *SocialLinks) UnmarshalYAML(n *yaml.Node) error {
	if n.Tag == "!!null" {
		*l = nil
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: social_links must be name: url pairs", n.Line)
	}
	*l = nil
	for i := 0; i+1 < len(n.Content); i += 2 {
		*l = append(*l, SocialLink{Name: n.Content[i].Value, URL: n.Content[i+1].Value})
	}
	return nil
}

func (l SocialLinks) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, s := range l {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s.Name}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s.URL})
	}
	return n, nil
}

// populated returns the links that have a URL.
func (l SocialLinks) populated() []SocialLink {
	var links []SocialLink
	for _, s := range l {
		if strings.TrimSpace(s.URL) != "" {
			links = append(links, s)
		}
	}
	return links
}

// DefaultThemePreferences are used for any theme preference missing from a
// theme's theme_preferences.yaml.
var DefaultThemePreferences = ThemePreferences{
	BannerColor:     Color{Light: "#add8e6", Dark: "#add8e6"},
	BannerTextColor: Color{Light: "#000000", Dark: "#000000"},
	BackgroundColor: Color{Light: "#ffffff", Dark: "#000000"},
	TextColor:       Color{Light: "#000000", Dark: "#ffffff"},
	Link:            Color{Light: "#0000ee", Dark: "#8ab4f8"},
	LinkClicked:     Color{Light: "#551a8b", Dark: "#c58af9"},
	TagColor:        Color{Light: "#add8e6", Dark: "#add8e6"},
	TagTextColor:    Color{Light: "#000000", Dark: "#000000"},
	Font:            "sans-serif",
}

// DefaultPreferences are used for any setting missing from settings.yaml.
var DefaultPreferences = Preferences{
	SiteName:        "Gho-Static",
	NavLinks:        []string{"about", "blogs", "gallery", "contact"},
	Theme:           "default_developer_ugly",
	Logo:            "static/img/gho_stephanie_64x64.svg",
	Favicon:         "static/img/favicon.ico",
	ShowIntroSidebar:       true,
	RecentSidebarCollections: []string{"gallery", "blogs"},
	IntroSidebarImage:      "static/img/gho_stephanie_64x64.svg",
	IntroName:        "Gho-Static",
	IntroDescription: "A simple static site generator, designed to make deploying easy",
	SocialLinks: SocialLinks{
		{Name: "github", URL: "github.com/mbtestfeed/gho-static"},
		{Name: "linkedin"},
		{Name: "twitter"},
		{Name: "mastodon"},
		{Name: "bluesky"},
		{Name: "facebook"},
	},
}

// ReadPreferences reads the preferences file at path over the defaults. A
// missing file gives the defaults.
func ReadPreferences(path string) (Preferences, error) {
	return readOver(os.DirFS(filepath.Dir(path)), filepath.Base(path), DefaultPreferences)
}

// readOver reads the yaml file at path in fsys over defaults. A missing file
// gives the defaults.
func readOver[T any](fsys fs.FS, path string, defaults T) (T, error) {
	prefs := defaults
	b, err := fs.ReadFile(fsys, path)
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

// AddToList adds value to the end of the list setting key in the
// preferences file at path, unless it's there already, keeping the rest of
// the file as it is. An empty setting starts as an empty list, and a missing
// one (or file) from its default.
// It reports whether value was added, rather than already there.
func AddToList(path, key, value string) (added bool, err error) {
	err = editList(path, key, func(list *yaml.Node) bool {
		if slices.ContainsFunc(list.Content, func(item *yaml.Node) bool { return item.Value == value }) {
			return false
		}
		if len(list.Content) == 0 {
			list.Style = 0 // block style rather than an empty [] list's flow style
		}
		list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: value})
		added = true
		return true
	})
	return added, err
}

// RemoveFromList removes value from the list setting key in the preferences
// file at path, as AddToList adds it.
func RemoveFromList(path, key, value string) error {
	return editList(path, key, func(list *yaml.Node) bool {
		n := len(list.Content)
		list.Content = slices.DeleteFunc(list.Content, func(item *yaml.Node) bool { return item.Value == value })
		return len(list.Content) != n
	})
}

// SetValue sets the setting key to value in the settings file at path,
// keeping the rest of the file, including any comment on the setting, as
// it is.
func SetValue(path, key, value string) error {
	return yaml_edit.EditFile(path, func(root *yaml.Node) (bool, error) {
		n := yaml_edit.MappingValue(root, key)
		if n == nil {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Value: value})
			return true, nil
		}
		if n.Kind == yaml.ScalarNode && n.Value == value {
			return false, nil
		}
		n.Kind, n.Tag, n.Style, n.Value, n.Content = yaml.ScalarNode, "", 0, value, nil
		return true, nil
	})
}

// SetDeployment saves d as the deployment setting in the preferences file
// at path, keeping the rest of the file as it is.
func SetDeployment(path string, d Deployment) error {
	return yaml_edit.EditFile(path, func(root *yaml.Node) (bool, error) {
		var value yaml.Node
		if err := value.Encode(d); err != nil {
			return false, err
		}
		old := yaml_edit.MappingValue(root, "deployment")
		if old == nil {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "deployment"}, &value)
			return true, nil
		}
		var current Deployment
		if old.Decode(&current) == nil && current.Method == d.Method && maps.Equal(current.Settings, d.Settings) {
			return false, nil
		}
		*old = value
		return true, nil
	})
}

// editList calls edit with the list setting key in the preferences file at
// path (see AddToList for empty and missing settings), writing the file
// back if edit reports a change, keeping the rest of it as it is.
func editList(path, key string, edit func(list *yaml.Node) bool) error {
	return yaml_edit.EditFile(path, func(root *yaml.Node) (bool, error) {
		list := yaml_edit.MappingValue(root, key)
		if list == nil {
			var defaults yaml.Node
			if err := defaults.Encode(DefaultPreferences); err != nil {
				return false, err
			}
			if list = yaml_edit.MappingValue(&defaults, key); list == nil {
				list = &yaml.Node{Kind: yaml.SequenceNode}
			}
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, list)
		}
		switch {
		case list.Kind == yaml.ScalarNode && (list.Tag == "!!null" || list.Value == ""):
			*list = yaml.Node{Kind: yaml.SequenceNode}
		case list.Kind != yaml.SequenceNode:
			return false, fmt.Errorf("can't change %s in %s: it isn't a list", key, path)
		}
		return edit(list), nil
	})
}

// fillCSS executes css (named name, for errors) as a text template with
// prefs, filling in any theme preference fields it references.
func fillCSS(css []byte, name string, prefs ThemePreferences) ([]byte, error) {
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

// themeDir returns the folder of p's theme, holding its css and theme
// preferences.
func (p Preferences) themeDir() string {
	return path.Join("themes", p.Theme)
}

// navLinkNames returns the content folder names listed in NavLinks, without
// blanks.
func (p Preferences) navLinkNames() []string {
	var names []string
	for _, n := range p.NavLinks {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names
}
