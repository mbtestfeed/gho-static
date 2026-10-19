package gho_static

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	NavLinks        []string `yaml:"nav_links"` // content folder names, in menu order
	BannerColor     Color  `yaml:"banner_color"`      // header, footer and menu background
	BannerTextColor Color  `yaml:"banner_text_color"` // text and links in the header, footer and menu
	BackgroundColor Color  `yaml:"background_color"`
	TextColor       Color  `yaml:"text_color"`
	Link            Color  `yaml:"link"`
	LinkClicked     Color  `yaml:"link_clicked"`
	TagColor        Color  `yaml:"tag_color"`      // tag pill background
	TagTextColor    Color  `yaml:"tag_text_color"` // tag pill text
	Font            string `yaml:"font"`
	Logo            string `yaml:"logo"`    // header image, relative to the site folder
	Favicon         string `yaml:"favicon"` // browser tab icon, relative to the site folder
	DefaultAuthor   string `yaml:"default_author"` // added to new markdown pages' front matter, if set
	ShowIntroSidebar       bool   `yaml:"show_intro_sidebar"`
	ShowRecentActivitySidebar bool `yaml:"show_recent_activity_sidebar"`
	IntroSidebarImage      string `yaml:"intro_sidebar_image"` // intro sidebar image, relative to the site folder
	IntroName        string      `yaml:"intro_name"`        // intro sidebar heading
	IntroDescription string      `yaml:"intro_description"` // intro sidebar text
	SocialLinks      SocialLinks `yaml:"social_links"`      // intro sidebar links, as name: url pairs; blank ones aren't shown
	Deployment       Deployment  `yaml:"deployment,omitempty"` // how the build folder is published, saved once it's set up
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

// SocialLinks are written in preferences.yaml as a mapping of name: url
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

// DefaultPreferences are used for any setting missing from preferences.yaml.
var DefaultPreferences = Preferences{
	SiteName:        "Gho-Static",
	NavLinks:        []string{"about", "blogs", "gallery", "contact"},
	BannerColor:     Color{Light: "#add8e6", Dark: "#add8e6"},
	BannerTextColor: Color{Light: "#000000", Dark: "#000000"},
	BackgroundColor: Color{Light: "#ffffff", Dark: "#000000"},
	TextColor:       Color{Light: "#000000", Dark: "#ffffff"},
	Link:           Color{Light: "#0000ee", Dark: "#8ab4f8"},
	LinkClicked:     Color{Light: "#551a8b", Dark: "#c58af9"},
	TagColor:        Color{Light: "#add8e6", Dark: "#add8e6"},
	TagTextColor:    Color{Light: "#000000", Dark: "#000000"},
	Font:           "sans-serif",
	Logo:            "static/img/gho_stephanie_64x64.svg",
	Favicon:         "static/img/favicon.ico",
	ShowIntroSidebar:       true,
	ShowRecentActivitySidebar: true,
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
	return readPreferences(os.DirFS(filepath.Dir(path)), filepath.Base(path))
}

// readPreferences reads the preferences file at path in fsys over the
// defaults. A missing file gives the defaults.
func readPreferences(fsys fs.FS, path string) (Preferences, error) {
	prefs := DefaultPreferences
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

// SetDeployment saves d as the deployment setting in the preferences file
// at path, keeping the rest of the file as it is.
func SetDeployment(path string, d Deployment) error {
	return editPreferences(path, func(root *yaml.Node) (bool, error) {
		var value yaml.Node
		if err := value.Encode(d); err != nil {
			return false, err
		}
		old := mappingValue(root, "deployment")
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
	return editPreferences(path, func(root *yaml.Node) (bool, error) {
		list := mappingValue(root, key)
		if list == nil {
			var defaults yaml.Node
			if err := defaults.Encode(DefaultPreferences); err != nil {
				return false, err
			}
			if list = mappingValue(&defaults, key); list == nil {
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

// editPreferences calls edit with the settings in the preferences file at
// path (none if it's missing or empty), writing the file back if edit
// reports a change, keeping the rest of it as it is.
func editPreferences(path string, edit func(root *yaml.Node) (bool, error)) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("can't change %s: it isn't a list of settings", path)
	}
	if changed, err := edit(root); err != nil || !changed {
		return err
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// mappingValue returns the value of key in the yaml mapping m, or nil if
// it has none.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
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
