package deployment

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gho-static/gho_static"
	gitclient "gho-static/utils/git-client"
	"gho-static/utils/output"
)

func TestFind(t *testing.T) {
	for _, name := range []string{"github-pages", "cloudflare-pages", "none"} {
		if m, ok := Find(name); !ok || m.Name != name {
			t.Errorf("Find(%q) = %q, %v, want the method", name, m.Name, ok)
		}
	}
	for _, name := range []string{"", "netlify", "GitHub Pages"} {
		if _, ok := Find(name); ok {
			t.Errorf("Find(%q) found a method, want none", name)
		}
	}
	if _, ok := Find(Default); !ok {
		t.Errorf("the default method, %q, isn't a method", Default)
	}
}

func TestEnvVar(t *testing.T) {
	m, _ := Find("cloudflare-pages")
	if got, want := m.EnvVar("api_token"), "GHO_STATIC_CLOUDFLARE_PAGES_API_TOKEN"; got != want {
		t.Errorf("EnvVar = %q, want %q", got, want)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		method  string
		s       Settings
		wantErr string
	}{
		{method: "github-pages", s: nil},
		{method: "github-pages", s: Settings{"username": "me"}},
		{method: "cloudflare-pages", s: Settings{"account_id": "a", "project": "p", "domain": "d", "api_token": "t"}},
		{method: "github-pages", s: Settings{"username": "me", "token": "t", "bogus": "b"}, wantErr: `unknown github-pages setting "bogus", "token" in from: it uses only username`},
		{method: "none", s: Settings{"username": "me"}, wantErr: "it uses no settings"},
	}
	for _, c := range cases {
		m, _ := Find(c.method)
		err := m.Validate(c.s, "from")
		if c.wantErr == "" && err != nil {
			t.Errorf("%s.Validate(%v) = %v, want nil", c.method, c.s, err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Errorf("%s.Validate(%v) = %v, want an error containing %q", c.method, c.s, err, c.wantErr)
		}
	}
}

func TestGather(t *testing.T) {
	m, _ := Find("cloudflare-pages")
	cases := []struct {
		name  string
		env   string
		saved Settings
		given Settings
		want  Settings
	}{
		{name: "nothing", want: Settings{}},
		{name: "saved", saved: Settings{"project": "p"}, want: Settings{"project": "p"}},
		{name: "saved secrets are dropped", saved: Settings{"project": "p", "api_token": "saved"}, want: Settings{"project": "p"}},
		{name: "secrets from the environment", env: "env", want: Settings{"api_token": "env"}},
		{name: "given over saved and the environment", env: "env", saved: Settings{"project": "p"}, given: Settings{"project": "q", "api_token": "given"}, want: Settings{"project": "q", "api_token": "given"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(m.EnvVar("api_token"), c.env)
			if got := m.Gather(c.saved, c.given); !maps.Equal(got, c.want) {
				t.Errorf("Gather = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSaveable(t *testing.T) {
	m, _ := Find("cloudflare-pages")
	if got, want := m.Saveable(Settings{"project": "p", "api_token": "t"}), (Settings{"project": "p"}); !maps.Equal(got, want) {
		t.Errorf("Saveable = %v, want %v", got, want)
	}
	if got := m.Saveable(Settings{"api_token": "t"}); got != nil {
		t.Errorf("Saveable of only secrets = %v, want nil", got)
	}
}

func TestDetect(t *testing.T) {
	if _, _, ok := Detect(t.TempDir()); ok {
		t.Error("detected a method for a plain folder")
	}
	if gitclient.Installed() != nil {
		t.Skip("git isn't installed")
	}
	build := t.TempDir()
	if err := gitclient.Init(build); err != nil {
		t.Fatal(err)
	}
	if err := gitclient.SetRemote(build, "git@github.com:me/me.github.io.git"); err != nil {
		t.Fatal(err)
	}
	m, s, ok := Detect(build)
	if !ok || m.Name != "github-pages" || !maps.Equal(s, Settings{"username": "me"}) {
		t.Errorf("Detect = %q, %v, %v, want github-pages with username me", m.Name, s, ok)
	}
}

func TestNone(t *testing.T) {
	var d Deployer = None{}
	if d.Check() != nil || d.Setup(t.TempDir()) != nil || d.Settings() != nil {
		t.Error("None should have nothing to check, set up or save")
	}
	if d.CheckPush() == nil || d.Push(t.TempDir()) == nil {
		t.Error("None should have nowhere to push to")
	}
}

// fakeDeployer records what's called on it.
type fakeDeployer struct {
	calls        *[]string
	checkPushErr error
}

func (d fakeDeployer) Check() error       { return nil }
func (d fakeDeployer) Settings() Settings { return nil }

func (d fakeDeployer) CheckPush() error {
	*d.calls = append(*d.calls, "checkpush")
	return d.checkPushErr
}

func (d fakeDeployer) Setup(string) error {
	*d.calls = append(*d.calls, "setup")
	return nil
}

func (d fakeDeployer) Push(dir string) error {
	call := "push"
	// Whether there's a published build to push.
	if _, err := os.Stat(filepath.Join(dir, "about", "index.html")); err == nil {
		call += " built"
	}
	*d.calls = append(*d.calls, call)
	return nil
}

// testSite writes a site with a home page and an about page into a new
// temporary folder, returning its path.
func testSite(t *testing.T) string {
	t.Helper()
	site := t.TempDir()
	for name, data := range map[string]string{
		"settings.yaml":                "nav_links: [about]\nshow_intro_sidebar: false\ncollections_displayed_in_recent_sidebar: []\n",
		"index.html":                   "<html>{{.Content}}</html>\n",
		"index.md":                     "# Home\n",
		"structure/header/header.html": "",
		"structure/footer/footer.html": "",
		"content/about/about.md":       "# About\n",
	} {
		p := filepath.Join(site, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(site, filepath.FromSlash(gho_static.GhoStaticThemesDir), gho_static.DefaultPreferences.Theme), 0755); err != nil {
		t.Fatal(err)
	}
	return site
}

func TestDeploy(t *testing.T) {
	quiet := output.Output{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	cases := []struct {
		name         string
		opts         gho_static.Options
		built        bool // whether site is built before deploying
		checkPushErr error
		wantCalls    []string
		wantVersion  bool
		wantErr      string
	}{
		{name: "builds for publishing, then pushes", wantCalls: []string{"checkpush", "setup", "push built"}, wantVersion: true},
		{name: "nothing done if it can't push", checkPushErr: errors.New("no username"), wantCalls: []string{"checkpush"}, wantErr: "no username"},
		{name: "without rebuilding pushes the build as it is", opts: gho_static.Options{NoRebuild: true}, built: true, wantCalls: []string{"checkpush", "setup", "push built"}},
		{name: "without rebuilding needs a build", opts: gho_static.Options{NoRebuild: true}, wantCalls: []string{"checkpush"}, wantErr: "hasn't been built"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site := testSite(t)
			if c.built {
				if _, err := gho_static.Build(site, gho_static.Options{Publish: true, Output: quiet}); err != nil {
					t.Fatal(err)
				}
			}
			var calls []string
			opts := c.opts
			opts.Output = quiet
			version, err := Deploy(site, fakeDeployer{calls: &calls, checkPushErr: c.checkPushErr}, opts)
			if c.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
				t.Fatalf("Deploy error = %v, want one containing %q", err, c.wantErr)
			}
			if !slices.Equal(calls, c.wantCalls) {
				t.Errorf("calls = %q, want %q", calls, c.wantCalls)
			}
			if (version != "") != c.wantVersion {
				t.Errorf("version = %q, want one: %v", version, c.wantVersion)
			}
		})
	}
}
