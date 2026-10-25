// Package cloudflare_pages deploys a site's build folder to Cloudflare
// Pages with its Direct Upload API, optionally linking a custom domain
// registered elsewhere.
package cloudflare_pages

import (
	"fmt"
	"os"
	"strings"

	"gho-static/utils/output"
)

// TokenEnvVar is the environment variable the API token can be given in.
const TokenEnvVar = "GHO_STATIC_CLOUDFLARE_PAGES_API_TOKEN"

// Deployer deploys a build folder to the Cloudflare Pages project Project in
// account AccountID, served at Domain too if set.
type Deployer struct {
	AccountID string
	Project   string
	Domain    string // e.g. blog.example.com, or "" for <project>.pages.dev only
	Token     string // API token with the Cloudflare Pages: Edit permission; never saved
	output.Output
}

// New returns a Deployer for settings, asking with ask (unless nil) for any
// that aren't set. The domain is only asked for along with the project, so
// it isn't asked again on each deploy.
func New(settings map[string]string, ask func(prompt string, hints []string, secret bool) string, out output.Output) Deployer {
	d := Deployer{AccountID: settings["account_id"], Project: settings["project"], Domain: settings["domain"], Token: settings["api_token"], Output: out}
	if ask == nil {
		return d
	}
	if d.AccountID == "" {
		d.AccountID = ask("Cloudflare account ID: ", []string{"shown on the right of your account's overview page in the Cloudflare dashboard, or press enter to add it later"}, false)
	}
	if d.Project == "" {
		d.Project = ask("Cloudflare Pages project name: ", []string{"lowercase letters, numbers and dashes. The site is served at <project>.pages.dev, and the project is created if it doesn't exist", "or press enter to add it later"}, false)
		if d.Project != "" && d.Domain == "" {
			d.Domain = ask("Custom domain (optional): ", []string{"e.g. blog.example.com, bought from any registrar, or press enter to skip"}, false)
		}
	}
	if d.Token == "" {
		d.Token = ask("Cloudflare API token: ", []string{"create one with the Cloudflare Pages: Edit permission at https://dash.cloudflare.com/profile/api-tokens", "it's never saved: set " + TokenEnvVar + " to skip this question, or press enter to add it later"}, true)
	}
	return d
}

// Settings returns d's settings to save, leaving out the token.
func (d Deployer) Settings() map[string]string {
	s := map[string]string{}
	for name, v := range map[string]string{"account_id": d.AccountID, "project": d.Project, "domain": d.Domain} {
		if v != "" {
			s[name] = v
		}
	}
	if len(s) == 0 {
		return nil
	}
	return s
}

// Check returns an error explaining what to do if d's token is set but
// isn't valid.
func (d Deployer) Check() error {
	if d.Token == "" {
		return nil
	}
	if err := verifyToken(d.AccountID, d.Token); err != nil {
		return fmt.Errorf("the Cloudflare API token isn't valid. Create one with the Cloudflare Pages: Edit permission at https://dash.cloudflare.com/profile/api-tokens\n\n%w", err)
	}
	return nil
}

// CheckPush returns an error explaining what's missing to deploy.
func (d Deployer) CheckPush() error {
	var missing []string
	if d.AccountID == "" {
		missing = append(missing, "no account ID is set (--deploy-setting account_id=<id>)")
	}
	if d.Project == "" {
		missing = append(missing, "no project name is set (--deploy-setting project=<name>)")
	}
	if d.Token == "" {
		missing = append(missing, "no API token is set ("+TokenEnvVar+")")
	}
	if len(missing) > 0 {
		return fmt.Errorf("can't deploy to Cloudflare Pages: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Setup creates buildDir, and links d's project and domain if everything
// needed to is set.
func (d Deployer) Setup(buildDir string) error {
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", buildDir, err)
	}
	if d.CheckPush() != nil {
		return nil
	}
	_, err := d.link()
	return err
}

// link creates d's project if it doesn't exist, and adds d's domain to it
// if it isn't already, explaining how to point the domain at it.
func (d Deployer) link() (project, error) {
	var p project
	active := false
	err := d.Step("Linking to the Cloudflare Pages project "+d.Project+". One second...", func() error {
		var err error
		if p, err = getProject(d.AccountID, d.Project, d.Token); isNotFound(err) {
			p, err = createProject(d.AccountID, d.Project, d.Token)
		}
		if err != nil || d.Domain == "" {
			return err
		}
		domains, err := getDomains(d.AccountID, d.Project, d.Token)
		if err != nil {
			return err
		}
		for _, dom := range domains {
			if strings.EqualFold(dom.Name, d.Domain) {
				active = dom.Status == "active"
				return nil
			}
		}
		_, err = addDomain(d.AccountID, d.Project, d.Token, d.Domain)
		return err
	})
	if err == nil && d.Domain != "" && !active {
		d.Infof("%s isn't verified yet. To serve the site at it, add a CNAME record for it pointing at %s with your domain's DNS provider. A root domain (e.g. example.com) needs its DNS moved to Cloudflare instead. It can take a while to verify.\n", d.Domain, p.Subdomain)
	}
	return p, err
}

// Push links d's project and domain, then deploys buildDir to it.
func (d Deployer) Push(buildDir string) error {
	p, err := d.link()
	if err != nil {
		return err
	}
	branch := p.ProductionBranch
	if branch == "" {
		branch = "main"
	}
	var url string
	err = d.Step("Deploying to "+p.Subdomain+". Shouldn't take long...", func() error {
		as, err := assets(buildDir)
		if err != nil {
			return err
		}
		url, err = deploy(d.AccountID, d.Project, d.Token, branch, as)
		return err
	})
	if err == nil {
		d.Successf("This deployment is at %s, and the site at https://%s/\n", url, p.Subdomain)
	}
	return err
}
