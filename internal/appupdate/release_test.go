package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanUpdateSelectsNewerStableDMG(t *testing.T) {
	release := releaseResponse{
		TagName: "1.4.0",
		Assets: []releaseAsset{{
			Name:        "ssh-man.dmg",
			Size:        42,
			Digest:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			DownloadURL: "https://github.com/ericwooley/ssh-man/releases/download/1.4.0/ssh-man.dmg",
		}},
	}

	plan, err := planUpdate("1.3.9", release)
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	if plan == nil {
		t.Fatal("expected an update plan")
	}
	if plan.Version != "1.4.0" || plan.Asset.Size != 42 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanUpdateDoesNotDowngradeOrUpdateDevBuilds(t *testing.T) {
	release := releaseResponse{
		TagName: "1.4.0",
		Assets: []releaseAsset{{
			Name:        "ssh-man.dmg",
			Size:        42,
			Digest:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			DownloadURL: "https://github.com/ericwooley/ssh-man/releases/download/1.4.0/ssh-man.dmg",
		}},
	}

	for _, current := range []string{"1.4.0", "1.5.0", "dev", ""} {
		t.Run(current, func(t *testing.T) {
			plan, err := planUpdate(current, release)
			if err != nil {
				t.Fatalf("plan update: %v", err)
			}
			if plan != nil {
				t.Fatalf("plan = %#v, want nil", plan)
			}
		})
	}
}

func TestPlanUpdateRejectsUnverifiedOrPrereleaseAssets(t *testing.T) {
	tests := []struct {
		name    string
		release releaseResponse
	}{
		{
			name: "prerelease",
			release: releaseResponse{
				TagName:    "1.4.0",
				Prerelease: true,
			},
		},
		{
			name: "missing digest",
			release: releaseResponse{
				TagName: "1.4.0",
				Assets: []releaseAsset{{
					Name:        "ssh-man.dmg",
					Size:        42,
					DownloadURL: "https://github.com/ericwooley/ssh-man/releases/download/1.4.0/ssh-man.dmg",
				}},
			},
		},
		{
			name: "untrusted download URL",
			release: releaseResponse{
				TagName: "1.4.0",
				Assets: []releaseAsset{{
					Name:        "ssh-man.dmg",
					Size:        42,
					Digest:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					DownloadURL: "https://example.com/ssh-man.dmg",
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if plan, err := planUpdate("1.3.0", tt.release); err == nil || plan != nil {
				t.Fatalf("plan = %#v, error = %v; want rejected release", plan, err)
			}
		})
	}
}

func TestPlanExperimentalUpdateSelectsNewestRelease(t *testing.T) {
	releases := []releaseResponse{
		{
			TagName:    "1.5.0",
			Prerelease: true,
			Assets: []releaseAsset{{
				Name:        "ssh-man.dmg",
				Size:        42,
				Digest:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				DownloadURL: "https://github.com/ericwooley/ssh-man/releases/download/1.5.0/ssh-man.dmg",
			}},
		},
		{
			TagName: "1.4.0",
			Assets: []releaseAsset{{
				Name:        "ssh-man.dmg",
				Size:        42,
				Digest:      "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				DownloadURL: "https://github.com/ericwooley/ssh-man/releases/download/1.4.0/ssh-man.dmg",
			}},
		},
	}

	plan, err := planExperimentalUpdate("1.3.0", releases)
	if err != nil {
		t.Fatalf("plan experimental update: %v", err)
	}
	if plan == nil || plan.Version != "1.5.0" {
		t.Fatalf("plan = %#v, want experimental version 1.5.0", plan)
	}
}

func TestClientChecksLatestOfficialReleaseEndpoint(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept header = %q", request.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"tag_name":"2.0.0",
			"draft":false,
			"prerelease":false,
			"assets":[{
				"name":"ssh-man.dmg",
				"size":42,
				"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"browser_download_url":%q
			}]
		}`, server.URL+"/ssh-man.dmg")
	}))
	t.Cleanup(server.Close)

	client := &Client{
		httpClient:       server.Client(),
		latestReleaseURL: server.URL,
		allowDownloadURL: func(rawURL string) bool { return strings.HasPrefix(rawURL, server.URL) },
	}
	plan, err := client.check(context.Background(), "1.9.9", false)
	if err != nil {
		t.Fatalf("check latest release: %v", err)
	}
	if plan == nil || plan.Version != "2.0.0" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestClientChecksExperimentalReleaseEndpoint(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/releases" {
			t.Errorf("request path = %q, want /releases", request.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{
			"tag_name":"2.1.0",
			"draft":false,
			"prerelease":true,
			"assets":[{
				"name":"ssh-man.dmg",
				"size":42,
				"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"browser_download_url":%q
			}]
		}]`, server.URL+"/ssh-man.dmg")
	}))
	t.Cleanup(server.Close)

	client := &Client{
		httpClient:       server.Client(),
		latestReleaseURL: server.URL + "/latest",
		releasesURL:      server.URL + "/releases",
		allowDownloadURL: func(rawURL string) bool { return strings.HasPrefix(rawURL, server.URL) },
	}
	plan, err := client.check(context.Background(), "1.9.9", true)
	if err != nil {
		t.Fatalf("check experimental release: %v", err)
	}
	if plan == nil || plan.Version != "2.1.0" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestDownloadVerifiedChecksDigestAndSize(t *testing.T) {
	payload := []byte("signed update payload")
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "ssh-man.dmg")
	err := downloadVerified(
		context.Background(),
		server.Client(),
		server.URL,
		digest,
		int64(len(payload)),
		destination,
		func(rawURL string) bool { return strings.HasPrefix(rawURL, server.URL) },
	)
	if err != nil {
		t.Fatalf("download verified: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read downloaded update: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded payload = %q", got)
	}

	badDestination := filepath.Join(t.TempDir(), "bad.dmg")
	err = downloadVerified(
		context.Background(),
		server.Client(),
		server.URL,
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		int64(len(payload)),
		badDestination,
		func(string) bool { return true },
	)
	if err == nil {
		t.Fatal("digest mismatch should fail")
	}
	if _, statErr := os.Stat(badDestination); !os.IsNotExist(statErr) {
		t.Fatalf("unverified download should be removed, stat error = %v", statErr)
	}
}
