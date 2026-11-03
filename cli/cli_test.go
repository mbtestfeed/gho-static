package cli

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gho-static/deployment"
	"gho-static/gho_static"
)

// parsed is the part of a Command that Parse sets and tests compare.
type parsed struct {
	Mode                           Mode
	Site, GenPath, PostName, Theme string
	AllThemes, Yes, Clean, Quiet   bool
	Options                        gho_static.Options
	method, badMethod, collection  string
	methodGiven                    bool
}

func summarise(c Command) parsed {
	return parsed{
		Mode: c.Mode, Site: c.Site, GenPath: c.GenPath, PostName: c.PostName, Theme: c.Theme,
		AllThemes: c.AllThemes, Yes: c.Yes, Clean: c.Clean, Quiet: c.Quiet, Options: c.Options,
		method: c.method, badMethod: c.badMethod, collection: c.collection, methodGiven: c.methodGiven,
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    parsed
		wantErr string
	}{
		{name: "build", args: []string{"mysite"}, want: parsed{Mode: Build, Site: "mysite"}},
		{name: "help", args: []string{"-h", "mysite"}, want: parsed{Mode: Help}},
		{name: "usage", args: []string{"--usage"}, want: parsed{Mode: Help}},
		{name: "combined short flags", args: []string{"-qyp", "mysite"}, want: parsed{Mode: Build, Site: "mysite", Quiet: true, Yes: true, Options: gho_static.Options{Publish: true}}},
		{name: "boolean flag before the site folder", args: []string{"--preview-publish", "mysite"}, want: parsed{Mode: Build, Site: "mysite", Options: gho_static.Options{Preview: true}}},
		{name: "undocumented flags", args: []string{"--publish-with-themes", "--no-rebuild", "--deploy", "mysite"}, want: parsed{Mode: Deploy, Site: "mysite", Options: gho_static.Options{WithThemes: true, NoRebuild: true}}},
		{name: "generate site with -g", args: []string{"-g", "mysite"}, want: parsed{Mode: GenerateSite, GenPath: "mysite"}},
		{name: "generate site with --generate", args: []string{"--generate", "mysite"}, want: parsed{Mode: GenerateSite, GenPath: "mysite"}},
		{name: "generate clean site", args: []string{"-gc", "mysite"}, want: parsed{Mode: GenerateSite, GenPath: "mysite", Clean: true}},
		{name: "generate site with all themes", args: []string{"--generate", "mysite", "--get-all-themes"}, want: parsed{Mode: GenerateSite, GenPath: "mysite", AllThemes: true}},
		{name: "generate page", args: []string{"--generate", "content/about", "mysite"}, want: parsed{Mode: GeneratePage, Site: "mysite", GenPath: filepath.Join("mysite", "content", "about"), collection: "about"}},
		{name: "generate post", args: []string{"--generate", "content/blogs/first", "mysite"}, want: parsed{Mode: GeneratePost, Site: "mysite", GenPath: "content/blogs/first", PostName: "first", collection: "blogs"}},
		{name: "generate theme", args: []string{"--generate", "theme_templates/mine", "mysite"}, want: parsed{Mode: GenerateTheme, Site: "mysite", GenPath: filepath.Join("mysite", filepath.FromSlash(gho_static.CustomThemesDir), "mine"), Theme: "mine"}},
		{name: "use theme", args: []string{"--use-theme", "brochureware", "mysite"}, want: parsed{Mode: UseTheme, Site: "mysite", Theme: "brochureware"}},
		{name: "get all themes", args: []string{"--get-all-themes", "mysite"}, want: parsed{Mode: GetAllThemes, Site: "mysite"}},
		{name: "list themes", args: []string{"--list-themes", "mysite"}, want: parsed{Mode: ListThemes, Site: "mysite"}},
		{name: "drafts", args: []string{"--drafts", "mysite"}, want: parsed{Mode: PublishDrafts, Site: "mysite"}},
		{name: "deploy", args: []string{"--deploy", "mysite"}, want: parsed{Mode: Deploy, Site: "mysite"}},
		{name: "deployment method", args: []string{"--deployment-method", "github-pages", "mysite"}, want: parsed{Mode: SetupDeployment, Site: "mysite", method: "github-pages", methodGiven: true}},
		{name: "deployment method missing takes the site folder back", args: []string{"--deployment-method", "mysite"}, want: parsed{Mode: SetupDeployment, Site: "mysite", badMethod: "mysite", methodGiven: true}},
		{name: "deployment method missing at the end", args: []string{"mysite", "--deployment-method"}, want: parsed{Mode: SetupDeployment, Site: "mysite", methodGiven: true}},

		{name: "no site folder", args: nil, wantErr: ErrMissingPath.Error()},
		{name: "only flags", args: []string{"-q"}, wantErr: ErrMissingPath.Error()},
		{name: "unknown long flag", args: []string{"--bogus", "mysite"}, wantErr: "unknown flag --bogus"},
		{name: "unknown short flag", args: []string{"-z", "mysite"}, wantErr: "unknown flag -z"},
		{name: "publish and preview", args: []string{"--publish", "--preview-publish", "mysite"}, wantErr: "can't be used together"},
		{name: "invalid theme name", args: []string{"--use-theme", "a/b", "mysite"}, wantErr: "--use-theme needs a theme name"},
		{name: "theme flag with generate", args: []string{"-g", "mysite", "--use-theme", "brochureware"}, wantErr: "can't be used with --generate"},
		{name: "deploy setting without a value", args: []string{"--deploy-setting", "username", "mysite"}, wantErr: "needs a name=value setting"},
		{name: "page nested too deep", args: []string{"--generate", "content/a/b/c", "mysite"}, wantErr: "nested one level deep"},
		{name: "page without a site folder", args: []string{"--generate", "content/about"}, wantErr: "missing site folder"},
		{name: "theme without a site folder", args: []string{"--generate", "theme_templates/mine"}, wantErr: "missing site folder"},
		{name: "all themes with a page", args: []string{"--generate", "content/about", "--get-all-themes", "mysite"}, wantErr: "--get-all-themes can't be used when generating"},
		{name: "reserved page name", args: []string{"--generate", "content/themes", "mysite"}, wantErr: "reserved for the themes showcase"},
		{name: "reserved collection name", args: []string{"--generate", "content/tags/first", "mysite"}, wantErr: "reserved for the tags page"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, err := Parse(c.args)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("Parse(%q) error = %v, want one containing %q", c.args, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := summarise(cmd); got != c.want {
				t.Errorf("Parse(%q) =\n%+v\nwant\n%+v", c.args, got, c.want)
			}
		})
	}
}

func TestParseMissingPathIsErrMissingPath(t *testing.T) {
	if _, err := Parse(nil); !errors.Is(err, ErrMissingPath) {
		t.Errorf("Parse(nil) error = %v, want ErrMissingPath", err)
	}
}

func TestParseCleanSitePrefs(t *testing.T) {
	cmd, err := Parse([]string{"-gc", filepath.Join("sites", "blank")})
	if err != nil {
		t.Fatal(err)
	}
	p := cmd.Prefs
	if p.SiteName != "blank" || p.IntroName != "" || p.IntroSidebarImage != "" {
		t.Errorf("clean site prefs = %+v, want it named after its folder, without an intro name or image", p)
	}
	if len(p.SocialLinks) != len(gho_static.DefaultPreferences.SocialLinks) {
		t.Fatalf("clean site has %d social links, want %d", len(p.SocialLinks), len(gho_static.DefaultPreferences.SocialLinks))
	}
	for i, l := range p.SocialLinks {
		if l.Name != gho_static.DefaultPreferences.SocialLinks[i].Name || l.URL != "" {
			t.Errorf("clean site social link %d = %+v, want the default's name without a URL", i, l)
		}
	}
}

func TestParseDeploySettings(t *testing.T) {
	cmd, err := Parse([]string{"--deploy-setting", "username=me", "--deploy-setting", "token=a=b", "mysite"})
	if err != nil {
		t.Fatal(err)
	}
	if want := (deployment.Settings{"username": "me", "token": "a=b"}); !maps.Equal(cmd.given, want) {
		t.Errorf("given settings = %v, want %v", cmd.given, want)
	}
}

// testNow is when Resolve is run in tests, dating generated posts.
var testNow = time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)

// writeFiles writes each of files (slash separated paths) with its contents
// into dir, creating folders as needed.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// resolve parses args and resolves them, answering with input, and returns
// the command, what was written and Resolve's error.
func resolve(t *testing.T, args []string, input string, interactive bool) (Command, string, error) {
	t.Helper()
	cmd, err := Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	term, out := newTerminal(input, interactive)
	cmd, err = Resolve(cmd, term, testNow)
	return cmd, out.String(), err
}

// wantErr fails the test unless err contains want, or is nil if want is "".
func wantErr(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want one containing %q", err, want)
	}
}

func TestResolveGenerateSite(t *testing.T) {
	existing, empty := t.TempDir(), t.TempDir()
	writeFiles(t, existing, map[string]string{"notes.txt": ""})
	file := filepath.Join(existing, "notes.txt")
	cases := []struct {
		name         string
		args         []string
		site         string // the folder to generate, or a new one if ""
		input        string
		interactive  bool
		wantMethod   string
		wantSettings deployment.Settings
		wantAsked    []string
		wantErr      string
	}{
		{name: "existing folder", args: []string{"-gy"}, site: existing, wantErr: "already exists, and isn't empty"},
		{name: "existing file", args: []string{"-gy"}, site: file, wantErr: "already exists, and isn't a folder"},
		{name: "empty folder is used", args: []string{"-gy"}, site: empty, wantMethod: "none"},
		{name: "yes deploys nowhere", args: []string{"-gy"}, wantMethod: "none"},
		{
			name: "asks for the method and its settings", args: []string{"-g"}, input: "1\nme\n", interactive: true,
			wantMethod: "github-pages", wantSettings: deployment.Settings{"username": "me"},
			wantAsked: []string{"How will you deploy the site?", "GitHub username: "},
		},
		{
			name: "method and settings given", args: []string{"-gy", "--deployment-method", "github-pages", "--deploy-setting", "username=me"},
			wantMethod: "github-pages", wantSettings: deployment.Settings{"username": "me"},
		},
		{
			name: "secrets aren't saved", args: []string{"-gy", "--deployment-method", "cloudflare-pages", "--deploy-setting", "project=p", "--deploy-setting", "api_token=secret"},
			wantMethod: "cloudflare-pages", wantSettings: deployment.Settings{"project": "p"},
		},
		{name: "unknown setting", args: []string{"-gy", "--deployment-method", "github-pages", "--deploy-setting", "bogus=1"}, wantErr: `unknown github-pages setting "bogus"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site := c.site
			if site == "" {
				site = filepath.Join(t.TempDir(), "mysite")
			}
			cmd, out, err := resolve(t, append(c.args, site), c.input, c.interactive)
			wantErr(t, err, c.wantErr)
			if c.wantErr != "" {
				return
			}
			if cmd.Deployment.Method != c.wantMethod || !maps.Equal(cmd.Deployment.Settings, c.wantSettings) {
				t.Errorf("deployment = %+v, want method %q with settings %v", cmd.Deployment, c.wantMethod, c.wantSettings)
			}
			if cmd.Prefs.Deployment.Method != c.wantMethod {
				t.Errorf("the site's preferences deploy with %q, want %q", cmd.Prefs.Deployment.Method, c.wantMethod)
			}
			for _, q := range c.wantAsked {
				if !strings.Contains(out, q) {
					t.Errorf("%q wasn't asked:\n%q", q, out)
				}
			}
		})
	}
}

func TestResolveDeployment(t *testing.T) {
	cases := []struct {
		name         string
		files        map[string]string // in the site folder
		args         []string          // before the site folder
		input        string
		wantMethod   string
		wantSettings deployment.Settings
		wantErr      string
	}{
		{name: "deploy with yes and nothing set up", args: []string{"--deploy", "--yes"}, wantErr: "has no deployment method or settings set up"},
		{
			name: "deploy with yes and everything given", args: []string{"--deploy", "--yes", "--deployment-method", "cloudflare-pages", "--deploy-setting", "account_id=a", "--deploy-setting", "project=p", "--deploy-setting", "api_token=t"},
			wantMethod: "cloudflare-pages", wantSettings: deployment.Settings{"account_id": "a", "project": "p"},
		},
		{
			name:  "deploy with yes uses saved settings",
			files: map[string]string{"settings.yaml": "deployment:\n  method: cloudflare-pages\n  account_id: a\n  project: p\n"},
			args:  []string{"--deploy", "--yes", "--deploy-setting", "api_token=t"}, wantMethod: "cloudflare-pages", wantSettings: deployment.Settings{"account_id": "a", "project": "p"},
		},
		{
			name:  "given settings replace saved ones",
			files: map[string]string{"settings.yaml": "deployment:\n  method: cloudflare-pages\n  account_id: a\n  project: p\n"},
			args:  []string{"--deploy", "--yes", "--deploy-setting", "project=q", "--deploy-setting", "api_token=t"}, wantMethod: "cloudflare-pages", wantSettings: deployment.Settings{"account_id": "a", "project": "q"},
		},
		{
			name:  "unknown saved setting",
			files: map[string]string{"settings.yaml": "deployment:\n  method: cloudflare-pages\n  bogus: 1\n"},
			args:  []string{"--deploy", "--yes"}, wantErr: `unknown cloudflare-pages setting "bogus"`,
		},
		{name: "set up asks for the method", args: []string{"--deployment-method"}, input: "3\n", wantMethod: "none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The token's environment variable would be used over nothing given.
			cloudflare, _ := deployment.Find("cloudflare-pages")
			t.Setenv(cloudflare.EnvVar("api_token"), "")
			site := t.TempDir()
			writeFiles(t, site, c.files)
			cmd, _, err := resolve(t, append(c.args, site), c.input, true)
			wantErr(t, err, c.wantErr)
			if c.wantErr != "" {
				return
			}
			if cmd.Deployment.Method != c.wantMethod || !maps.Equal(cmd.Deployment.Settings, c.wantSettings) {
				t.Errorf("deployment = %+v, want method %q with settings %v", cmd.Deployment, c.wantMethod, c.wantSettings)
			}
		})
	}
}

func TestResolveUnknownDeploymentMethod(t *testing.T) {
	_, _, err := resolve(t, []string{"--deployment-method", filepath.Join(t.TempDir(), "nowhere")}, "", false)
	wantErr(t, err, "unknown deployment method")
}

func TestResolveGeneratePageAndPost(t *testing.T) {
	site := map[string]string{
		"content/about/about.md":   "",
		"content/blogs/blogs.html": "",
		"content/blogs/blogs.md":   "",
	}
	cases := []struct {
		name        string
		args        []string // before the site folder
		input       string
		interactive bool
		wantGenPath string // within the site folder, slash separated
		wantNavLink string
		wantErr     string
	}{
		{name: "new page with yes gets a menu link", args: []string{"-gy", "content/news"}, wantGenPath: "content/news", wantNavLink: "news"},
		{name: "new page asks for a menu link", args: []string{"-g", "content/news"}, input: "\n", interactive: true, wantGenPath: "content/news", wantNavLink: "news"},
		{name: "new page menu link declined", args: []string{"-g", "content/news"}, input: "n\n", interactive: true, wantGenPath: "content/news"},
		{name: "new page with no one to ask gets no menu link", args: []string{"-g", "content/news"}, wantGenPath: "content/news"},
		{name: "post in a collection isn't asked about", args: []string{"-g", "content/blogs/first"}, interactive: true, wantGenPath: "content/blogs/20260601_first"},
		{name: "post in a new collection asks", args: []string{"-g", "content/news/first"}, input: "y\n\n", interactive: true, wantGenPath: "content/news/20260601_first", wantNavLink: "news"},
		{name: "post in a new collection declined", args: []string{"-g", "content/news/first"}, input: "n\n", interactive: true, wantErr: "wasn't created"},
		{name: "post in a new collection with no one to ask", args: []string{"-g", "content/news/first"}, wantErr: "there's no one to ask"},
		{name: "post in a new collection with yes", args: []string{"-gy", "content/news/first"}, wantGenPath: "content/news/20260601_first", wantNavLink: "news"},
		{name: "post in a page", args: []string{"-gy", "content/about/first"}, wantErr: "is a page, not a collection"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, site)
			cmd, _, err := resolve(t, append(c.args, dir), c.input, c.interactive)
			wantErr(t, err, c.wantErr)
			if c.wantErr != "" {
				return
			}
			if want := filepath.Join(dir, filepath.FromSlash(c.wantGenPath)); cmd.GenPath != want {
				t.Errorf("GenPath = %q, want %q", cmd.GenPath, want)
			}
			if cmd.NavLink != c.wantNavLink {
				t.Errorf("NavLink = %q, want %q", cmd.NavLink, c.wantNavLink)
			}
		})
	}
}

func TestResolveBuild(t *testing.T) {
	empty, site := t.TempDir(), t.TempDir()
	writeFiles(t, site, map[string]string{"index.html": ""})
	cases := []struct {
		name    string
		site    string
		wantErr string
	}{
		{name: "missing folder", site: filepath.Join(t.TempDir(), "missing"), wantErr: "doesn't exist. If it's a new site, create it first with `gho-static --generate"},
		{name: "folder without index.html", site: empty, wantErr: "has no index.html. If it's a new site, create it first with `gho-static --generate"},
		{name: "site", site: site},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := resolve(t, []string{c.site}, "", false)
			wantErr(t, err, c.wantErr)
		})
	}
}

func TestResolveMissingSite(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, args := range [][]string{
		{"-gy", "content/about", missing},
		{"-gy", "content/blogs/first", missing},
		{"--use-theme", "brochureware", missing},
		{"--get-all-themes", missing},
		{"--list-themes", missing},
		{"-g", "theme_templates/mine", missing},
	} {
		if _, _, err := resolve(t, args, "", false); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Resolve(%q) error = %v, want one saying the site folder doesn't exist", args, err)
		}
	}
}

func TestResolveGenerateExistingTheme(t *testing.T) {
	site := t.TempDir()
	writeFiles(t, site, map[string]string{gho_static.CustomThemesDir + "/mine/theme.css": ""})
	_, _, err := resolve(t, []string{"-g", "theme_templates/mine", site}, "", false)
	wantErr(t, err, "already has a theme called mine")
}
