// Package deployment sets up and deploys a site's build folder with its
// deployment method.
package deployment

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gho-static/deployment/github_pages"
	"gho-static/gho_static"
	"gho-static/utils/output"
)

// Deployer publishes a site's build folder.
type Deployer interface {
	// Check returns an error explaining what to do if a build folder can't
	// be set up for it yet.
	Check() error
	// Setup sets up buildDir to deploy from. The site should be built into
	// it afterwards.
	Setup(buildDir string) error
	// CheckPush returns an error explaining what's missing to push.
	CheckPush() error
	// Push publishes buildDir, once it's set up and built.
	Push(buildDir string) error
	// Settings returns the settings to save in settings.yaml.
	Settings() Settings
}

// Settings are a deployment method's settings by name, as saved in
// settings.yaml, given with --deploy-setting or in environment variables.
type Settings = map[string]string

// Ask asks for a setting with prompt, showing hints beneath it, and returns
// the answer. If secret is set, what's typed isn't shown.
type Ask func(prompt string, hints []string, secret bool) string

// Method is a way of deploying, as chosen with --deployment-method.
type Method struct {
	Name  string // as given to --deployment-method
	Label string // as listed when asking which method to use
	// Settings are the names of the settings it uses, other than Secrets.
	Settings []string
	// Secrets are settings never saved to settings.yaml, such as API
	// keys. Each is given with --deploy-setting, in its environment variable
	// (see EnvVar), or asked for with secret set.
	Secrets []string
	// New returns the method's Deployer for settings, asking with ask
	// (unless nil) for any it needs that aren't set.
	New func(settings Settings, ask Ask, out output.Output) Deployer
	// Detect returns the settings buildDir is set up with, and whether it's
	// set up for this method. Nil if it can't tell.
	Detect func(buildDir string) (Settings, bool)
}

// Default is the name of the method used when none is chosen.
const Default = "none"

// Methods are the ways of deploying, in the order they're offered.
var Methods = []Method{
	{
		Name:     "github-pages",
		Label:    "GitHub Pages",
		Settings: []string{"username"},
		New: func(s Settings, ask Ask, out output.Output) Deployer {
			return github_pages.New(s["username"], ask, out)
		},
		Detect: func(buildDir string) (Settings, bool) {
			username, ok := github_pages.Detect(buildDir)
			return Settings{"username": username}, ok
		},
	},
	{
		Name:  "none",
		Label: "none",
		New:   func(Settings, Ask, output.Output) Deployer { return None{} },
	},
}

// Find returns the method called name.
func Find(name string) (Method, bool) {
	for _, m := range Methods {
		if m.Name == name {
			return m, true
		}
	}
	return Method{}, false
}

// EnvVar returns the environment variable m's secret setting name can be
// given in, e.g. GHO_STATIC_GITHUB_PAGES_TOKEN.
func (m Method) EnvVar(name string) string {
	return "GHO_STATIC_" + strings.ToUpper(strings.ReplaceAll(m.Name+"_"+name, "-", "_"))
}

// Validate returns an error naming any of s that m doesn't use, and those it
// does, saying they came from from.
func (m Method) Validate(s Settings, from string) error {
	known := slices.Concat(m.Settings, m.Secrets)
	var unknown []string
	for name := range s {
		if !slices.Contains(known, name) {
			unknown = append(unknown, strconv.Quote(name))
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	uses := "no settings"
	if len(known) > 0 {
		uses = "only " + strings.Join(known, ", ")
	}
	return fmt.Errorf("unknown %s setting %s in %s: it uses %s", m.Name, strings.Join(unknown, ", "), from, uses)
}

// Gather returns m's settings: those given with --deploy-setting, then its
// secrets' environment variables, then saved (from settings.yaml or
// detected), leaving out any saved secrets.
func (m Method) Gather(saved, given Settings) Settings {
	s := m.Saveable(saved)
	if s == nil {
		s = Settings{}
	}
	for _, name := range m.Secrets {
		if v := os.Getenv(m.EnvVar(name)); v != "" {
			s[name] = v
		}
	}
	maps.Copy(s, given)
	return s
}

// Saveable returns s without m's secrets, to save in settings.yaml.
func (m Method) Saveable(s Settings) Settings {
	var saveable Settings
	for name, v := range s {
		if slices.Contains(m.Secrets, name) {
			continue
		}
		if saveable == nil {
			saveable = Settings{}
		}
		saveable[name] = v
	}
	return saveable
}

// Detect returns the method buildDir is set up for and the settings it's
// set up with, if any.
func Detect(buildDir string) (Method, Settings, bool) {
	for _, m := range Methods {
		if m.Detect == nil {
			continue
		}
		if s, ok := m.Detect(buildDir); ok {
			return m, s, true
		}
	}
	return Method{}, nil, false
}

// None is the Deployer for a build folder that isn't deployed.
type None struct{}

func (None) Check() error       { return nil }
func (None) Setup(string) error { return nil }
func (None) CheckPush() error {
	return errors.New("no deployment method is set up, so there's nothing to deploy to")
}
func (n None) Push(string) error  { return n.CheckPush() }
func (None) Settings() Settings { return nil }

// Deploy checks everything d needs is in place, builds site for publishing
// with opts into its build folder and deploys it, returning the build
// version.
func Deploy(site string, d Deployer, opts gho_static.Options) (string, error) {
	if err := d.CheckPush(); err != nil {
		return "", err
	}
	buildDir := filepath.Join(site, "build")
	if err := d.Setup(buildDir); err != nil {
		return "", err
	}
	opts.Publish = true
	version, err := gho_static.Build(site, opts)
	if err != nil {
		return "", err
	}
	return version, d.Push(buildDir)
}
