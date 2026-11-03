package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// Open VSX (open-vsx.org, run by the Eclipse Foundation) is the registry.
// Microsoft's marketplace terms allow it only in Microsoft's own products, so
// it is never queried. HITTABLE_EXT_REGISTRY points at another Open VSX
// server (a company mirror, or a test server).
const defaultRegistry = "https://open-vsx.org"

// EnvRegistry names the registry override variable.
const EnvRegistry = "HITTABLE_EXT_REGISTRY"

func registryBase() string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvRegistry)), "/"); v != "" {
		return v
	}
	return defaultRegistry
}

// apiClient honours HTTPS_PROXY / NO_PROXY through the default transport.
var apiClient = &http.Client{Timeout: 30 * time.Second}

// Summary is one search hit.
type Summary struct {
	ID          string
	Publisher   string // the namespace
	Version     string
	DisplayName string
	Description string
	Downloads   int64
	Rating      float64
	Verified    bool
	Deprecated  bool
	Icon        string // URL of the icon (PNG), "" when it has none
}

// Release is one downloadable build of one version.
type Release struct {
	ID             string
	Version        string
	TargetPlatform string
	Engine         string // engines.vscode
	PreRelease     bool
	Verified       bool
	Download       string
	SHA256         string // URL of a file holding the hex digest
	Signature      string // URL of the .sigzip
	PublicKey      string // URL of the publisher key (PEM)
}

type apiFiles struct {
	Icon      string `json:"icon"`
	Readme    string `json:"readme"`
	Changelog string `json:"changelog"`
	Manifest  string `json:"manifest"`
	Download  string `json:"download"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
	PublicKey string `json:"publicKey"`
}

type apiExtension struct {
	Namespace      string            `json:"namespace"`
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	DisplayName    string            `json:"displayName"`
	Description    string            `json:"description"`
	DownloadCount  int64             `json:"downloadCount"`
	AverageRating  float64           `json:"averageRating"`
	ReviewCount    int               `json:"reviewCount"`
	NamespaceName  string            `json:"namespaceDisplayName"`
	Categories     []string          `json:"categories"`
	License        string            `json:"license"`
	Repository     string            `json:"repository"`
	Homepage       string            `json:"homepage"`
	Timestamp      string            `json:"timestamp"`
	GalleryColor   string            `json:"galleryColor"`
	Verified       bool              `json:"verified"`
	Deprecated     bool              `json:"deprecated"`
	PreRelease     bool              `json:"preRelease"`
	TargetPlatform string            `json:"targetPlatform"`
	Engines        map[string]string `json:"engines"`
	Files          apiFiles          `json:"files"`
}

func getJSON(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "hittable-extensions")
	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach the extension registry (%s): %w", registryBase(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("the extension registry is rate-limiting requests; try again in a minute")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("extension registry: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(v)
}

// Search queries the registry by text, most downloaded first. An empty
// query lists the most installed extensions.
func Search(ctx context.Context, query string, size int) ([]Summary, error) {
	if size <= 0 {
		size = 30
	}
	q := url.Values{"query": {query}, "size": {fmt.Sprint(size)}, "sortBy": {"downloadCount"}, "sortOrder": {"desc"}}
	var res struct {
		Extensions []apiExtension `json:"extensions"`
	}
	if err := getJSON(ctx, registryBase()+"/api/-/search?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(res.Extensions))
	for _, e := range res.Extensions {
		out = append(out, Summary{
			ID: strings.ToLower(e.Namespace + "." + e.Name), Publisher: e.Namespace, Version: e.Version, DisplayName: e.DisplayName,
			Description: e.Description, Downloads: e.DownloadCount, Rating: e.AverageRating, Verified: e.Verified, Deprecated: e.Deprecated,
			Icon: e.Files.Icon,
		})
	}
	return out, nil
}

// Resolve picks the build of id to install here: the newest stable version
// whose engines.vscode accepts APIVersion and whose platform fits this machine,
// falling back to the newest compatible pre-release when no stable one exists.
// version, when set, pins an exact version instead.
func Resolve(ctx context.Context, id, version string) (Release, error) {
	here := TargetPlatform()
	var all []apiExtension
	for _, target := range []string{here, "universal"} {
		for offset := 0; offset < 500; offset += 100 {
			q := url.Values{"extensionId": {id}, "includeAllVersions": {"true"}, "targetPlatform": {target},
				"size": {"100"}, "offset": {fmt.Sprint(offset)}}
			var res struct {
				Extensions []apiExtension `json:"extensions"`
				TotalSize  int            `json:"totalSize"`
			}
			if err := getJSON(ctx, registryBase()+"/api/-/query?"+q.Encode(), &res); err != nil {
				return Release{}, err
			}
			all = append(all, res.Extensions...)
			if offset+100 >= res.TotalSize || hasStableFit(res.Extensions, here) {
				break
			}
		}
	}
	if len(all) == 0 {
		return Release{}, fmt.Errorf("%s is not on %s (Microsoft-only extensions are not published there)", id, registryBase())
	}
	sort.SliceStable(all, func(i, j int) bool { return newer(all[i].Version, all[j].Version) })

	var pick, pre *apiExtension
	tooNew := ""
	for i := range all {
		e := &all[i]
		if version != "" && e.Version != version {
			continue
		}
		if !platformFits(e.TargetPlatform, here) {
			continue
		}
		if !EngineAllows(e.Engines["vscode"], APIVersion) {
			if tooNew == "" {
				tooNew = e.Version + " needs VS Code " + e.Engines["vscode"]
			}
			continue
		}
		if e.PreRelease {
			if pre == nil {
				pre = e
			}
			continue
		}
		pick = e
		break
	}
	if pick == nil {
		pick = pre
	}
	if pick == nil {
		switch {
		case version != "":
			return Release{}, fmt.Errorf("%s has no version %s for %s", id, version, here)
		case tooNew != "":
			return Release{}, fmt.Errorf("%s: every build needs a newer VS Code API than %s (latest: %s)", id, APIVersion, tooNew)
		}
		return Release{}, fmt.Errorf("%s publishes no build for %s", id, here)
	}
	return Release{
		ID: strings.ToLower(pick.Namespace + "." + pick.Name), Version: pick.Version,
		TargetPlatform: pick.TargetPlatform, Engine: pick.Engines["vscode"], PreRelease: pick.PreRelease,
		Verified: pick.Verified, Download: pick.Files.Download, SHA256: pick.Files.SHA256,
		Signature: pick.Files.Signature, PublicKey: pick.Files.PublicKey,
	}, nil
}

func hasStableFit(es []apiExtension, here string) bool {
	for _, e := range es {
		if !e.PreRelease && platformFits(e.TargetPlatform, here) && EngineAllows(e.Engines["vscode"], APIVersion) {
			return true
		}
	}
	return false
}

// fetchSmall reads a small text resource (digest, key, signature bundle).
func fetchSmall(ctx context.Context, u string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "hittable-extensions")
	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// Detail is an extension's registry page: what the Extensions panel shows
// on its cover.
type Detail struct {
	Summary
	PublisherName string // display name of the namespace
	Reviews       int
	Categories    []string
	License       string
	Repository    string
	Homepage      string
	Updated       string // RFC 3339
	Color         string // gallery banner colour, may be ""
	Engine        string
	Readme        string // URLs; "" when the extension has none
	Changelog     string
	ManifestURL   string
}

// Details fetches the latest version's registry entry.
func Details(ctx context.Context, id string) (Detail, error) {
	ns, name, ok := strings.Cut(strings.ToLower(id), ".")
	if !ok {
		return Detail{}, fmt.Errorf("%q is not an extension id", id)
	}
	var e apiExtension
	if err := getJSON(ctx, registryBase()+"/api/"+url.PathEscape(ns)+"/"+url.PathEscape(name), &e); err != nil {
		return Detail{}, err
	}
	return Detail{
		Summary: Summary{ID: strings.ToLower(e.Namespace + "." + e.Name), Publisher: e.Namespace, Version: e.Version,
			DisplayName: e.DisplayName, Description: e.Description, Downloads: e.DownloadCount, Rating: e.AverageRating,
			Verified: e.Verified, Deprecated: e.Deprecated, Icon: e.Files.Icon},
		PublisherName: e.NamespaceName, Reviews: e.ReviewCount, Categories: e.Categories, License: e.License,
		Repository: e.Repository, Homepage: e.Homepage, Updated: e.Timestamp, Color: e.GalleryColor,
		Engine: e.Engines["vscode"], Readme: e.Files.Readme, Changelog: e.Files.Changelog, ManifestURL: e.Files.Manifest,
	}, nil
}

// FetchText reads a registry text file (README, changelog), at most 2 MB.
func FetchText(ctx context.Context, u string) (string, error) {
	b, err := fetchSmall(ctx, u, 2<<20)
	return string(b), err
}

// FetchManifest reads an extension's package.json from the registry, for
// describing what it contributes before it is installed.
func FetchManifest(ctx context.Context, u string) (Manifest, error) {
	var m Manifest
	b, err := fetchSmall(ctx, u, 4<<20)
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(StripJSONC(b), &m)
}

// FetchBytes reads a small binary file (an icon), at most 1 MB.
func FetchBytes(ctx context.Context, u string) ([]byte, error) { return fetchSmall(ctx, u, 1<<20) }
