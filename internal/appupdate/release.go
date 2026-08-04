package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseURL = "https://api.github.com/repos/ericwooley/ssh-man/releases/latest"
	releaseAssetName = "ssh-man.dmg"
	maxMetadataBytes = 1 << 20
	maxUpdateBytes   = 250 << 20
)

var (
	versionPattern = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)$`)
	digestPattern  = regexp.MustCompile(`^sha256:([0-9a-fA-F]{64})$`)
)

type releaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Digest      string `json:"digest"`
	DownloadURL string `json:"browser_download_url"`
}

type releaseResponse struct {
	TagName    string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type updatePlan struct {
	Version string
	Asset   releaseAsset
}

type Client struct {
	httpClient       *http.Client
	latestReleaseURL string
	allowDownloadURL func(string) bool
}

func newClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("too many update download redirects")
				}
				if req.URL.Scheme != "https" {
					return fmt.Errorf("update download redirected to non-HTTPS URL %q", req.URL.String())
				}
				return nil
			},
		},
		latestReleaseURL: latestReleaseURL,
		allowDownloadURL: trustedReleaseDownloadURL,
	}
}

func (c *Client) check(ctx context.Context, currentVersion string) (*updatePlan, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.latestReleaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create latest-release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ssh-man/"+currentVersion)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("check latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check latest release: GitHub returned %s", response.Status)
	}

	var release releaseResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxMetadataBytes+1))
	if err := decoder.Decode(&release); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	return planUpdateWithURLPolicy(currentVersion, release, c.allowDownloadURL)
}

func (c *Client) download(ctx context.Context, asset releaseAsset, destination string) error {
	return downloadVerified(
		ctx,
		c.httpClient,
		asset.DownloadURL,
		asset.Digest,
		asset.Size,
		destination,
		c.allowDownloadURL,
	)
}

func planUpdate(currentVersion string, release releaseResponse) (*updatePlan, error) {
	return planUpdateWithURLPolicy(currentVersion, release, trustedReleaseDownloadURL)
}

func planUpdateWithURLPolicy(currentVersion string, release releaseResponse, allowURL func(string) bool) (*updatePlan, error) {
	current, ok := parseVersion(currentVersion)
	if !ok {
		return nil, nil
	}
	latest, ok := parseVersion(release.TagName)
	if !ok {
		return nil, fmt.Errorf("latest release has unsupported version %q", release.TagName)
	}
	if release.Draft {
		return nil, errors.New("latest release is still a draft")
	}
	if release.Prerelease {
		return nil, errors.New("latest release is a prerelease")
	}
	if compareVersion(latest, current) <= 0 {
		return nil, nil
	}

	for _, asset := range release.Assets {
		if asset.Name != releaseAssetName {
			continue
		}
		if asset.Size <= 0 || asset.Size > maxUpdateBytes {
			return nil, fmt.Errorf("release asset size %d is outside the supported range", asset.Size)
		}
		if !digestPattern.MatchString(asset.Digest) {
			return nil, errors.New("release asset is missing a valid SHA-256 digest")
		}
		if allowURL == nil || !allowURL(asset.DownloadURL) {
			return nil, fmt.Errorf("release asset URL is not trusted: %q", asset.DownloadURL)
		}
		return &updatePlan{Version: release.TagName, Asset: asset}, nil
	}

	return nil, fmt.Errorf("latest release does not include %s", releaseAssetName)
}

func downloadVerified(
	ctx context.Context,
	httpClient *http.Client,
	rawURL string,
	digest string,
	expectedSize int64,
	destination string,
	allowURL func(string) bool,
) (returnErr error) {
	if allowURL == nil || !allowURL(rawURL) {
		return fmt.Errorf("update download URL is not trusted: %q", rawURL)
	}
	digestMatch := digestPattern.FindStringSubmatch(digest)
	if len(digestMatch) != 2 {
		return errors.New("update download has no valid SHA-256 digest")
	}
	if expectedSize <= 0 || expectedSize > maxUpdateBytes {
		return fmt.Errorf("update download size %d is outside the supported range", expectedSize)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create update download request: %w", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "ssh-man-updater")
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: server returned %s", response.Status)
	}

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create update download: %w", err)
	}
	defer func() {
		if closeErr := output.Close(); returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close update download: %w", closeErr)
		}
		if returnErr != nil {
			_ = os.Remove(destination)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(output, hasher), io.LimitReader(response.Body, expectedSize+1))
	if err != nil {
		return fmt.Errorf("write update download: %w", err)
	}
	if written != expectedSize {
		return fmt.Errorf("update download size = %d, want %d", written, expectedSize)
	}
	expectedDigest, err := hex.DecodeString(digestMatch[1])
	if err != nil {
		return fmt.Errorf("decode update digest: %w", err)
	}
	if !equalBytes(hasher.Sum(nil), expectedDigest) {
		return errors.New("update download SHA-256 digest does not match release metadata")
	}
	return nil
}

func trustedReleaseDownloadURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "github.com" {
		return false
	}
	return strings.HasPrefix(parsed.EscapedPath(), "/ericwooley/ssh-man/releases/download/") &&
		strings.HasSuffix(parsed.EscapedPath(), "/"+releaseAssetName)
}

type semanticVersion [3]uint64

func parseVersion(value string) (semanticVersion, bool) {
	match := versionPattern.FindStringSubmatch(value)
	if len(match) != 4 {
		return semanticVersion{}, false
	}
	var parsed semanticVersion
	for index := range parsed {
		component, err := strconv.ParseUint(match[index+1], 10, 64)
		if err != nil {
			return semanticVersion{}, false
		}
		parsed[index] = component
	}
	return parsed, true
}

func compareVersion(left, right semanticVersion) int {
	for index := range left {
		switch {
		case left[index] < right[index]:
			return -1
		case left[index] > right[index]:
			return 1
		}
	}
	return 0
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}
