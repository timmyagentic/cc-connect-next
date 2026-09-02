package appfeatures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	featuregithub "github.com/timmyagentic/awesome-agent-app-features/updater/github"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
	"golang.org/x/mod/semver"
)

const maxReleaseListBytes = 2 * 1024 * 1024

var errNoPrerelease = errors.New("no published prerelease is available")

type prereleaseUpdateSource interface {
	LatestPrerelease(context.Context) (featureupdater.Release, error)
}

// githubUpdateSource keeps Stable on the Foundation's audited /releases/latest
// adapter and adds only the missing host policy: selecting the highest valid
// published prerelease from the bounded releases list. Downloads still use
// the Foundation adapter's URL and redirect validation.
type githubUpdateSource struct {
	repository string
	apiBaseURL string
	httpClient *http.Client
	stable     featuregithub.Source
}

func newGitHubUpdateSource() *githubUpdateSource {
	stable := featuregithub.Source{Repository: releaseRepository, UserAgent: ProductName + "-updater/1"}
	return &githubUpdateSource{repository: releaseRepository, stable: stable}
}

func (source *githubUpdateSource) LatestStable(ctx context.Context) (featureupdater.Release, error) {
	return source.stable.LatestStable(ctx)
}

func (source *githubUpdateSource) Download(ctx context.Context, asset featureupdater.Asset, destination io.Writer) error {
	return source.stable.Download(ctx, asset, destination)
}

func (source *githubUpdateSource) LatestPrerelease(ctx context.Context) (featureupdater.Release, error) {
	parts := strings.Split(source.repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return featureupdater.Release{}, fmt.Errorf("release repository is invalid")
	}
	base := strings.TrimRight(source.apiBaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	requestURL, err := url.JoinPath(base, "repos", parts[0], parts[1], "releases")
	if err != nil {
		return featureupdater.Release{}, fmt.Errorf("build prerelease URL: %w", err)
	}
	parsed, err := url.Parse(requestURL)
	if err != nil || (parsed.Scheme != "https" && (parsed.Scheme != "http" || !isLoopbackHost(parsed.Hostname()))) {
		return featureupdater.Release{}, fmt.Errorf("prerelease API URL must use HTTPS")
	}
	query := parsed.Query()
	query.Set("per_page", "100")
	parsed.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return featureupdater.Release{}, fmt.Errorf("create prerelease request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", ProductName+"-updater/1")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := source.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return fmt.Errorf("prerelease API redirect refused")
	}
	response, err := copyClient.Do(request)
	if err != nil {
		return featureupdater.Release{}, fmt.Errorf("request prereleases: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return featureupdater.Release{}, fmt.Errorf("prerelease API returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseListBytes+1))
	if err != nil {
		return featureupdater.Release{}, fmt.Errorf("read prereleases: %w", err)
	}
	if len(data) > maxReleaseListBytes {
		return featureupdater.Release{}, fmt.Errorf("prerelease response exceeded %d bytes", maxReleaseListBytes)
	}
	var releases []githubReleaseWire
	if err := json.Unmarshal(data, &releases); err != nil {
		return featureupdater.Release{}, fmt.Errorf("decode prereleases: %w", err)
	}
	var selected featureupdater.Release
	for _, wire := range releases {
		tag := strings.TrimSpace(wire.TagName)
		if wire.Draft || !wire.Prerelease || !semver.IsValid(tag) || semver.Prerelease(tag) == "" {
			continue
		}
		if wire.TagName != tag {
			return featureupdater.Release{}, fmt.Errorf("prerelease tag contains surrounding whitespace")
		}
		candidate, err := wire.release(source.repository)
		if err != nil {
			return featureupdater.Release{}, err
		}
		if err := validateUpdateReleaseForChannel(candidate, updatechannel.Beta); err != nil {
			return featureupdater.Release{}, err
		}
		if selected.Tag == "" || semver.Compare(candidate.Tag, selected.Tag) > 0 {
			selected = candidate
		}
	}
	if selected.Tag == "" {
		return featureupdater.Release{}, errNoPrerelease
	}
	return selected, nil
}

type githubReleaseWire struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

func (wire githubReleaseWire) release(repository string) (featureupdater.Release, error) {
	release := featureupdater.Release{
		Tag:        wire.TagName,
		URL:        wire.HTMLURL,
		Notes:      wire.Body,
		Draft:      wire.Draft,
		Prerelease: wire.Prerelease,
		Assets:     make([]featureupdater.Asset, 0, len(wire.Assets)),
	}
	if err := validateGitHubReleaseURL(release.URL, repository, release.Tag); err != nil {
		return featureupdater.Release{}, err
	}
	seen := make(map[string]struct{}, len(wire.Assets))
	for _, item := range wire.Assets {
		if item.Name == "" || item.Name != strings.TrimSpace(item.Name) || item.Size < 0 {
			return featureupdater.Release{}, fmt.Errorf("release contains invalid asset metadata")
		}
		if _, exists := seen[item.Name]; exists {
			return featureupdater.Release{}, fmt.Errorf("release contains duplicate asset %q", item.Name)
		}
		seen[item.Name] = struct{}{}
		if err := validateGitHubAssetURL(item.BrowserDownloadURL, repository, release.Tag, item.Name); err != nil {
			return featureupdater.Release{}, err
		}
		release.Assets = append(release.Assets, featureupdater.Asset{
			Name: item.Name, DownloadURL: item.BrowserDownloadURL, Size: item.Size,
		})
	}
	return release, nil
}

func validateGitHubReleaseURL(raw, repository, tag string) error {
	return validateExactGitHubURL(raw, "/"+repository+"/releases/tag/"+tag)
}

func validateGitHubAssetURL(raw, repository, tag, name string) error {
	return validateExactGitHubURL(raw, "/"+repository+"/releases/download/"+tag+"/"+name)
}

func validateExactGitHubURL(raw, expectedPath string) error {
	value, err := url.Parse(raw)
	if err != nil || value.Scheme != "https" || !strings.EqualFold(value.Host, "github.com") ||
		value.User != nil || value.RawQuery != "" || value.Fragment != "" || value.EscapedPath() != expectedPath {
		return fmt.Errorf("release contains an invalid GitHub URL")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}

func validateUpdateReleaseForChannel(release featureupdater.Release, channel updatechannel.Channel) error {
	channel = channel.Effective()
	if channel == updatechannel.Stable {
		return featureupdater.ValidateStableRelease(release)
	}
	tag := strings.TrimSpace(release.Tag)
	if release.Tag != tag || release.Draft || !release.Prerelease || !semver.IsValid(tag) ||
		semver.Prerelease(tag) == "" || semver.Build(tag) != "" {
		return fmt.Errorf("refusing non-beta release %q", release.Tag)
	}
	return nil
}

func isNewerUpdateRelease(candidate, current string, channel updatechannel.Channel) (bool, error) {
	if channel.Effective() == updatechannel.Stable {
		return featureupdater.IsNewerStable(candidate, current)
	}
	if !semver.IsValid(candidate) || semver.Prerelease(candidate) == "" {
		return false, fmt.Errorf("candidate %q is not a beta semantic version", candidate)
	}
	if !semver.IsValid(current) {
		return false, fmt.Errorf("current version %q is not a semantic version", current)
	}
	return semver.Compare(candidate, current) > 0, nil
}

func latestUpdateRelease(ctx context.Context, source featureupdater.Source, channel updatechannel.Channel) (featureupdater.Release, error) {
	if channel.Effective() == updatechannel.Stable {
		return source.LatestStable(ctx)
	}
	prereleases, ok := source.(prereleaseUpdateSource)
	if !ok {
		return featureupdater.Release{}, fmt.Errorf("update source does not support beta releases")
	}
	return prereleases.LatestPrerelease(ctx)
}
