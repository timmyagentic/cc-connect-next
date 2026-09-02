package appfeatures

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	featureupdater "github.com/timmyagentic/awesome-agent-app-features/updater"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

func TestGitHubUpdateSourceSelectsHighestValidPublishedPrerelease(t *testing.T) {
	release := func(tag string, draft bool, releaseURL string) map[string]any {
		name := "cc-connect-next-" + tag + "-darwin-arm64.tar.gz"
		return map[string]any{
			"tag_name": tag, "html_url": releaseURL, "body": tag,
			"draft": draft, "prerelease": true,
			"assets": []map[string]any{{
				"name": name, "size": 10,
				"browser_download_url": "https://github.com/timmyagentic/cc-connect-next/releases/download/" + tag + "/" + name,
			}},
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/timmyagentic/cc-connect-next/releases" || request.URL.Query().Get("per_page") != "100" {
			t.Fatalf("request = %s", request.URL.String())
		}
		_ = json.NewEncoder(writer).Encode([]map[string]any{
			release("v0.3.0-beta.1", false, "https://github.com/timmyagentic/cc-connect-next/releases/tag/v0.3.0-beta.1"),
			release("v0.3.0-beta.9", true, "https://github.com/timmyagentic/cc-connect-next/releases/tag/v0.3.0-beta.9"),
			release("v0.3.0-beta.2", false, "https://github.com/timmyagentic/cc-connect-next/releases/tag/v0.3.0-beta.2"),
		})
	}))
	defer server.Close()

	source := &githubUpdateSource{
		repository: releaseRepository,
		apiBaseURL: server.URL,
		httpClient: server.Client(),
	}
	got, err := source.LatestPrerelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "v0.3.0-beta.2" || !got.Prerelease || len(got.Assets) != 1 {
		t.Fatalf("selected prerelease = %#v", got)
	}
}

func TestGitHubUpdateSourceFailsClosedOnMalformedPublishedPrerelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode([]map[string]any{{
			"tag_name": "v0.3.0-beta.2", "html_url": "https://evil.example/releases/tag/v0.3.0-beta.2",
			"draft": false, "prerelease": true, "assets": []any{},
		}})
	}))
	defer server.Close()
	source := &githubUpdateSource{repository: releaseRepository, apiBaseURL: server.URL, httpClient: server.Client()}
	if _, err := source.LatestPrerelease(context.Background()); err == nil {
		t.Fatal("malformed published prerelease was silently skipped")
	}
}

func TestCheckForUpdateChannelDoesNotCrossStableAndBeta(t *testing.T) {
	source := &memoryUpdateSource{release: featureupdater.Release{Tag: "v0.3.0-beta.2", Prerelease: true}}
	got, err := CheckForUpdateChannel(context.Background(), "v0.3.0-beta.1", updatechannel.Beta, source)
	if err != nil || got == nil || got.Tag != "v0.3.0-beta.2" {
		t.Fatalf("beta discovery = %#v, %v", got, err)
	}
	if _, err := CheckForUpdateChannel(context.Background(), "v0.2.1", updatechannel.Stable, source); err == nil {
		t.Fatal("stable discovery accepted prerelease metadata")
	}
}
