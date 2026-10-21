package cloudflare_pages

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// apiBase is the Cloudflare API's address.
const apiBase = "https://api.cloudflare.com/client/v4"

// Limits on uploads, matching Cloudflare's.
const (
	maxFileSize   = 25 << 20 // bytes per file
	maxBatchSize  = 40 << 20 // base64 bytes per upload request
	maxBatchFiles = 5000     // files per upload request
)

// apiError is an unsuccessful response from the Cloudflare API.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("Cloudflare API error (%d): %s", e.Status, e.Message)
}

// isNotFound reports whether err is a 404 response.
func isNotFound(err error) bool {
	var e *apiError
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

// call sends method to url with body of contentType, authorised with token,
// and decodes the response's result into result (unless nil).
func call(method, url, token, contentType string, body io.Reader, result any) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach Cloudflare: %w", err)
	}
	defer resp.Body.Close()
	var r struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("unexpected response from Cloudflare (%s): %w", resp.Status, err)
	}
	if !r.Success {
		var msgs []string
		for _, e := range r.Errors {
			msgs = append(msgs, e.Message)
		}
		return &apiError{Status: resp.StatusCode, Message: strings.Join(msgs, "; ")}
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(r.Result, result)
}

// callJSON sends body as JSON, as call does.
func callJSON(method, url, token string, body, result any) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	return call(method, url, token, "application/json", r, result)
}

// project is a Cloudflare Pages project.
type project struct {
	Subdomain        string `json:"subdomain"` // e.g. mysite.pages.dev
	ProductionBranch string `json:"production_branch"`
}

// domain is a custom domain linked to a project.
type domain struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "active" once it's verified
}

// projectURL returns the API address of account's project name.
func projectURL(account, name string) string {
	return apiBase + "/accounts/" + account + "/pages/projects/" + name
}

// verifyToken checks token is valid, as either an account's own token or a
// user's.
func verifyToken(account, token string) error {
	if account != "" && callJSON(http.MethodGet, apiBase+"/accounts/"+account+"/tokens/verify", token, nil, nil) == nil {
		return nil
	}
	return callJSON(http.MethodGet, apiBase+"/user/tokens/verify", token, nil, nil)
}

func getProject(account, name, token string) (project, error) {
	var p project
	return p, callJSON(http.MethodGet, projectURL(account, name), token, nil, &p)
}

func createProject(account, name, token string) (project, error) {
	var p project
	body := map[string]string{"name": name, "production_branch": "main"}
	return p, callJSON(http.MethodPost, apiBase+"/accounts/"+account+"/pages/projects", token, body, &p)
}

func getDomains(account, name, token string) ([]domain, error) {
	var ds []domain
	return ds, callJSON(http.MethodGet, projectURL(account, name)+"/domains", token, nil, &ds)
}

func addDomain(account, name, token, host string) (domain, error) {
	var d domain
	return d, callJSON(http.MethodPost, projectURL(account, name)+"/domains", token, map[string]string{"name": host}, &d)
}

// asset is a file to deploy.
type asset struct {
	path        string // as served, e.g. /blogs/index.html
	file        string // on disk
	hash        string
	contentType string
}

// assets returns the files in buildDir to deploy, leaving out hidden ones
// such as a .git folder left by another deployment method.
func assets(buildDir string) ([]asset, error) {
	var as []asset
	err := filepath.WalkDir(buildDir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(e.Name(), ".") && p != buildDir {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if e.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if len(data) > maxFileSize {
			return fmt.Errorf("%s is too big for Cloudflare Pages, which takes files up to 25 MiB", p)
		}
		rel, err := filepath.Rel(buildDir, p)
		if err != nil {
			return err
		}
		ext := filepath.Ext(p)
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		// Files are keyed by a hash of their contents and extension, so
		// unchanged ones aren't uploaded again.
		sum := sha256.Sum256([]byte(base64.StdEncoding.EncodeToString(data) + strings.TrimPrefix(ext, ".")))
		as = append(as, asset{
			path:        "/" + filepath.ToSlash(rel),
			file:        p,
			hash:        hex.EncodeToString(sum[:])[:32],
			contentType: contentType,
		})
		return nil
	})
	return as, err
}

// deploy uploads the assets Cloudflare doesn't have yet to account's
// project name, then deploys them all to branch, returning the deployment's
// address.
func deploy(account, name, token, branch string, as []asset) (string, error) {
	var jwt struct {
		JWT string `json:"jwt"`
	}
	if err := callJSON(http.MethodGet, projectURL(account, name)+"/upload-token", token, nil, &jwt); err != nil {
		return "", err
	}

	hashes := make([]string, len(as))
	manifest := map[string]string{}
	for i, a := range as {
		hashes[i] = a.hash
		manifest[a.path] = a.hash
	}
	var missingHashes []string
	if err := callJSON(http.MethodPost, apiBase+"/pages/assets/check-missing", jwt.JWT, map[string][]string{"hashes": hashes}, &missingHashes); err != nil {
		return "", err
	}
	missing := map[string]bool{}
	for _, h := range missingHashes {
		missing[h] = true
	}

	type payload struct {
		Key      string            `json:"key"`
		Value    string            `json:"value"`
		Metadata map[string]string `json:"metadata"`
		Base64   bool              `json:"base64"`
	}
	var batch []payload
	batchSize := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := callJSON(http.MethodPost, apiBase+"/pages/assets/upload", jwt.JWT, batch, nil)
		batch, batchSize = nil, 0
		return err
	}
	for _, a := range as {
		if !missing[a.hash] {
			continue
		}
		// Files with the same contents share a hash, so are uploaded once.
		delete(missing, a.hash)
		data, err := os.ReadFile(a.file)
		if err != nil {
			return "", err
		}
		value := base64.StdEncoding.EncodeToString(data)
		if len(batch) == maxBatchFiles || batchSize+len(value) > maxBatchSize {
			if err := flush(); err != nil {
				return "", err
			}
		}
		batch = append(batch, payload{Key: a.hash, Value: value, Metadata: map[string]string{"contentType": a.contentType}, Base64: true})
		batchSize += len(value)
	}
	if err := flush(); err != nil {
		return "", err
	}
	if err := callJSON(http.MethodPost, apiBase+"/pages/assets/upsert-hashes", jwt.JWT, map[string][]string{"hashes": hashes}, nil); err != nil {
		return "", err
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("manifest", string(data))
	w.WriteField("branch", branch)
	if err := w.Close(); err != nil {
		return "", err
	}
	var deployment struct {
		URL string `json:"url"`
	}
	if err := call(http.MethodPost, projectURL(account, name)+"/deployments", token, w.FormDataContentType(), &body, &deployment); err != nil {
		return "", err
	}
	return deployment.URL, nil
}
