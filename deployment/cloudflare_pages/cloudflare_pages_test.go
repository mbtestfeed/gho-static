package cloudflare_pages

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gho-static/utils/output"
)

// handlerTransport answers requests with a handler, in place of the network.
type handlerTransport struct{ h http.Handler }

func (t handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.h.ServeHTTP(rec, r)
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

// fakeAPI is a Cloudflare API for the account "acc" and its project "proj",
// accepting the API token "tok" and the upload token "jwt". It records what
// it's sent.
type fakeAPI struct {
	t             *testing.T
	projectExists bool
	domains       []domain
	missing       []string // hashes check-missing reports; all those asked about if nil

	created  bool
	added    []string          // domains added
	uploaded []string          // hashes uploaded, in order
	upserted []string          // hashes upserted
	manifest map[string]string // deployed path to hash
	branch   string            // deployed to
}

// useFakeAPI sends the Cloudflare API's requests to f for the rest of the
// test.
func useFakeAPI(t *testing.T, f *fakeAPI) {
	f.t = t
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = handlerTransport{f}
	t.Cleanup(func() { http.DefaultClient.Transport = old })
}

// reply writes a successful API response with result.
func reply(w http.ResponseWriter, result any) {
	json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
}

// fail writes an unsuccessful API response.
func fail(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []map[string]string{{"message": message}}})
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/client/v4")
	proj := "/accounts/acc/pages/projects/proj"
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if strings.HasPrefix(p, "/pages/assets/") {
		if token != "jwt" {
			fail(w, http.StatusUnauthorized, "bad upload token")
			return
		}
	} else if token != "tok" {
		fail(w, http.StatusUnauthorized, "bad token")
		return
	}
	var hashes struct {
		Hashes []string `json:"hashes"`
	}
	switch r.Method + " " + p {
	case "GET /accounts/acc/tokens/verify":
		reply(w, nil)
	case "GET " + proj:
		if !f.projectExists {
			fail(w, http.StatusNotFound, "project not found")
			return
		}
		reply(w, project{Subdomain: "proj.pages.dev", ProductionBranch: "live"})
	case "POST /accounts/acc/pages/projects":
		f.created, f.projectExists = true, true
		reply(w, project{Subdomain: "proj.pages.dev", ProductionBranch: "main"})
	case "GET " + proj + "/domains":
		reply(w, f.domains)
	case "POST " + proj + "/domains":
		var d domain
		json.NewDecoder(r.Body).Decode(&d)
		f.added = append(f.added, d.Name)
		reply(w, domain{Name: d.Name, Status: "pending"})
	case "GET " + proj + "/upload-token":
		reply(w, map[string]string{"jwt": "jwt"})
	case "POST /pages/assets/check-missing":
		json.NewDecoder(r.Body).Decode(&hashes)
		if f.missing != nil {
			reply(w, f.missing)
			return
		}
		reply(w, hashes.Hashes)
	case "POST /pages/assets/upload":
		var batch []struct {
			Key    string `json:"key"`
			Base64 bool   `json:"base64"`
		}
		json.NewDecoder(r.Body).Decode(&batch)
		for _, b := range batch {
			f.uploaded = append(f.uploaded, b.Key)
		}
		reply(w, nil)
	case "POST /pages/assets/upsert-hashes":
		json.NewDecoder(r.Body).Decode(&hashes)
		f.upserted = hashes.Hashes
		reply(w, nil)
	case "POST " + proj + "/deployments":
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			f.t.Errorf("deployment isn't a multipart form: %v", err)
		}
		json.Unmarshal([]byte(r.FormValue("manifest")), &f.manifest)
		f.branch = r.FormValue("branch")
		reply(w, map[string]string{"url": "https://abc.proj.pages.dev"})
	default:
		fail(w, http.StatusNotFound, "no route "+r.Method+" "+p)
	}
}

// writeBuild writes files (slash separated paths) into a new temporary build
// folder, returning its path.
func writeBuild(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// testDeployer returns a Deployer for the fake API's project, with domain,
// writing to a buffer, and the buffer.
func testDeployer(domain string) (Deployer, *bytes.Buffer) {
	var out bytes.Buffer
	return Deployer{AccountID: "acc", Project: "proj", Domain: domain, Token: "tok", Output: output.Output{Out: &out, Err: &out}}, &out
}

// prompts are the starts of the questions New asks, in the order it asks
// them.
var prompts = []string{"Cloudflare account ID", "Cloudflare Pages project", "Custom domain", "Cloudflare API token"}

func TestNew(t *testing.T) {
	cases := []struct {
		name       string
		settings   map[string]string
		answers    map[string]string // by the start of the prompt (see prompts); others get ""
		noAsking   bool
		want       Deployer
		wantAsked  []string
		wantSecret []string
	}{
		{
			name:     "all set",
			settings: map[string]string{"account_id": "a", "project": "p", "domain": "d", "api_token": "t"},
			want:     Deployer{AccountID: "a", Project: "p", Domain: "d", Token: "t"},
		},
		{
			name:       "asks for what's missing",
			answers:    map[string]string{"Cloudflare account ID": "a", "Cloudflare Pages project": "p", "Custom domain": "d", "Cloudflare API token": "t"},
			want:       Deployer{AccountID: "a", Project: "p", Domain: "d", Token: "t"},
			wantAsked:  prompts,
			wantSecret: []string{"Cloudflare API token"},
		},
		{
			name:      "domain only asked with the project",
			settings:  map[string]string{"account_id": "a", "project": "p", "api_token": "t"},
			want:      Deployer{AccountID: "a", Project: "p", Token: "t"},
			wantAsked: nil,
		},
		{
			name:      "domain not asked when the project's skipped",
			settings:  map[string]string{"account_id": "a", "api_token": "t"},
			want:      Deployer{AccountID: "a", Token: "t"},
			wantAsked: []string{"Cloudflare Pages project"},
		},
		{
			name:     "nothing asked without ask",
			noAsking: true,
			want:     Deployer{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var asked, secret []string
			ask := func(prompt string, hints []string, isSecret bool) string {
				i := slices.IndexFunc(prompts, func(start string) bool { return strings.HasPrefix(prompt, start) })
				if i == -1 {
					t.Fatalf("unexpected question %q", prompt)
				}
				asked = append(asked, prompts[i])
				if isSecret {
					secret = append(secret, prompts[i])
				}
				return c.answers[prompts[i]]
			}
			if c.noAsking {
				ask = nil
			}
			got := New(c.settings, ask, output.Output{})
			if got.AccountID != c.want.AccountID || got.Project != c.want.Project || got.Domain != c.want.Domain || got.Token != c.want.Token {
				t.Errorf("New = %+v, want %+v", got, c.want)
			}
			if !slices.Equal(asked, c.wantAsked) {
				t.Errorf("asked %q, want %q", asked, c.wantAsked)
			}
			if !slices.Equal(secret, c.wantSecret) {
				t.Errorf("asked secretly %q, want %q", secret, c.wantSecret)
			}
		})
	}
}

func TestSettings(t *testing.T) {
	d := Deployer{AccountID: "a", Project: "p", Token: "t"}
	s := d.Settings()
	if len(s) != 2 || s["account_id"] != "a" || s["project"] != "p" {
		t.Errorf("Settings = %v, want the account ID and project, without the token or empty domain", s)
	}
	if s := (Deployer{Token: "t"}).Settings(); s != nil {
		t.Errorf("Settings with only a token = %v, want nil", s)
	}
}

func TestCheckPush(t *testing.T) {
	d, _ := testDeployer("")
	if err := d.CheckPush(); err != nil {
		t.Errorf("CheckPush with everything set = %v, want nil", err)
	}
	err := (Deployer{}).CheckPush()
	if err == nil {
		t.Fatal("CheckPush with nothing set = nil, want an error")
	}
	for _, want := range []string{"account ID", "project name", TokenEnvVar} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("CheckPush error doesn't mention %q: %v", want, err)
		}
	}
}

func TestCheck(t *testing.T) {
	useFakeAPI(t, &fakeAPI{})
	if err := (Deployer{}).Check(); err != nil {
		t.Errorf("Check without a token = %v, want nil, as it's added later", err)
	}
	d, _ := testDeployer("")
	if err := d.Check(); err != nil {
		t.Errorf("Check with a valid token = %v, want nil", err)
	}
	d.Token = "bad"
	if err := d.Check(); err == nil || !strings.Contains(err.Error(), "isn't valid") {
		t.Errorf("Check with an invalid token = %v, want an error saying so", err)
	}
}

func TestCallErrors(t *testing.T) {
	useFakeAPI(t, &fakeAPI{})
	err := callJSON(http.MethodGet, projectURL("acc", "proj"), "tok", nil, nil)
	if !isNotFound(err) || !strings.Contains(err.Error(), "project not found") {
		t.Errorf("getting a missing project = %v, want a 404 with its message", err)
	}
	if err := callJSON(http.MethodGet, projectURL("acc", "proj"), "bad", nil, nil); err == nil || isNotFound(err) {
		t.Errorf("calling with a bad token = %v, want an error other than a 404", err)
	}

	http.DefaultClient.Transport = handlerTransport{http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html>not json</html>")
	})}
	if err := callJSON(http.MethodGet, projectURL("acc", "proj"), "tok", nil, nil); err == nil || !strings.Contains(err.Error(), "unexpected response") {
		t.Errorf("a response that isn't JSON = %v, want an error saying so", err)
	}
}

func TestAssets(t *testing.T) {
	build := writeBuild(t, map[string]string{
		"index.html":         "same",
		"about/index.html":   "same",
		"static/a.css":       "same",
		"static/data.weird":  "data",
		".nojekyll":          "",
		".git/HEAD":          "ref",
		"static/.hidden.txt": "hidden",
	})
	as, err := assets(build)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]asset{}
	for _, a := range as {
		byPath[a.path] = a
	}
	var paths []string
	for p := range byPath {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	if want := []string{"/about/index.html", "/index.html", "/static/a.css", "/static/data.weird"}; !slices.Equal(paths, want) {
		t.Errorf("asset paths = %q, want %q, leaving out hidden files", paths, want)
	}
	if !strings.HasPrefix(byPath["/index.html"].contentType, "text/html") {
		t.Errorf("index.html's content type = %q, want text/html", byPath["/index.html"].contentType)
	}
	if got := byPath["/static/data.weird"].contentType; got != "application/octet-stream" {
		t.Errorf("an unknown extension's content type = %q, want application/octet-stream", got)
	}
	if byPath["/index.html"].hash != byPath["/about/index.html"].hash {
		t.Error("files with the same contents and extension have different hashes")
	}
	if byPath["/index.html"].hash == byPath["/static/a.css"].hash {
		t.Error("files with the same contents but different extensions have the same hash")
	}
	if n := len(byPath["/index.html"].hash); n != 32 {
		t.Errorf("hash is %d characters, want 32", n)
	}
}

func TestAssetsTooBig(t *testing.T) {
	build := t.TempDir()
	f, err := os.Create(filepath.Join(build, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(maxFileSize + 1)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assets(build); err == nil || !strings.Contains(err.Error(), "too big") {
		t.Errorf("assets with a file over 25 MiB = %v, want an error saying it's too big", err)
	}
}

func TestPush(t *testing.T) {
	build := writeBuild(t, map[string]string{
		"index.html":       "home",
		"about/index.html": "home",
		"static/a.css":     "css",
	})
	as, err := assets(build)
	if err != nil {
		t.Fatal(err)
	}
	hashOf := map[string]string{}
	for _, a := range as {
		hashOf[a.path] = a.hash
	}

	cases := []struct {
		name         string
		api          fakeAPI
		domain       string
		wantCreated  bool
		wantAdded    []string
		wantUploaded []string
		wantBranch   string
		wantOut      []string
	}{
		{
			name:         "creates the project and uploads everything once",
			wantCreated:  true,
			wantUploaded: []string{hashOf["/index.html"], hashOf["/static/a.css"]},
			wantBranch:   "main",
			wantOut:      []string{"This deployment is at https://abc.proj.pages.dev, and the site at https://proj.pages.dev/"},
		},
		{
			name:         "uploads only what's missing, to the production branch",
			api:          fakeAPI{projectExists: true, missing: []string{hashOf["/static/a.css"]}},
			wantUploaded: []string{hashOf["/static/a.css"]},
			wantBranch:   "live",
		},
		{
			name:         "adds a new domain and explains how to point it",
			api:          fakeAPI{projectExists: true, missing: []string{}},
			domain:       "blog.example.com",
			wantAdded:    []string{"blog.example.com"},
			wantBranch:   "live",
			wantOut:      []string{"blog.example.com isn't verified yet", "CNAME record for it pointing at proj.pages.dev"},
		},
		{
			name:       "leaves an active domain alone",
			api:        fakeAPI{projectExists: true, missing: []string{}, domains: []domain{{Name: "Blog.Example.com", Status: "active"}}},
			domain:     "blog.example.com",
			wantBranch: "live",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			api := c.api
			useFakeAPI(t, &api)
			d, out := testDeployer(c.domain)
			if err := d.Push(build); err != nil {
				t.Fatal(err)
			}
			if api.created != c.wantCreated {
				t.Errorf("project created: %v, want %v", api.created, c.wantCreated)
			}
			if !slices.Equal(api.added, c.wantAdded) {
				t.Errorf("domains added = %q, want %q", api.added, c.wantAdded)
			}
			slices.Sort(api.uploaded)
			slices.Sort(c.wantUploaded)
			if !slices.Equal(api.uploaded, c.wantUploaded) {
				t.Errorf("uploaded = %q, want %q", api.uploaded, c.wantUploaded)
			}
			if len(api.upserted) != len(as) {
				t.Errorf("upserted %d hashes, want one for each of the %d files", len(api.upserted), len(as))
			}
			if len(api.manifest) != len(as) {
				t.Errorf("manifest = %v, want a hash for each of %d files", api.manifest, len(as))
			}
			for p, h := range hashOf {
				if api.manifest[p] != h {
					t.Errorf("manifest[%q] = %q, want %q", p, api.manifest[p], h)
				}
			}
			if api.branch != c.wantBranch {
				t.Errorf("deployed to branch %q, want %q", api.branch, c.wantBranch)
			}
			for _, want := range c.wantOut {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output doesn't contain %q:\n%s", want, out)
				}
			}
			if c.domain != "" && len(c.wantAdded) == 0 && strings.Contains(out.String(), "isn't verified yet") {
				t.Errorf("an active domain was said to be unverified:\n%s", out)
			}
		})
	}
}

func TestSetup(t *testing.T) {
	api := fakeAPI{}
	useFakeAPI(t, &api)
	build := filepath.Join(t.TempDir(), "build")

	// Without everything needed to link, only the folder is made.
	if err := (Deployer{Output: output.Output{Out: io.Discard}}).Setup(build); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(build); err != nil || !info.IsDir() {
		t.Fatalf("Setup didn't create the build folder: %v", err)
	}
	if api.created {
		t.Error("Setup linked a project without the settings to")
	}

	d, _ := testDeployer("")
	if err := d.Setup(build); err != nil {
		t.Fatal(err)
	}
	if !api.created {
		t.Error("Setup with everything set didn't create the missing project")
	}
}
