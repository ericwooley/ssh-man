package remoteport

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	faviconPageLimit  = 1 << 20
	faviconImageLimit = 256 << 10
	faviconTimeout    = 8 * time.Second
)

var faviconMediaTypes = map[string]bool{
	"image/gif":                true,
	"image/jpeg":               true,
	"image/png":                true,
	"image/vnd.microsoft.icon": true,
	"image/webp":               true,
	"image/x-icon":             true,
}

func FindFavicon(ctx context.Context, rawBaseURL string) (string, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse service URL: %w", err)
	}
	if err := validateFaviconBaseURL(baseURL); err != nil {
		return "", err
	}
	baseURL.Path = "/"
	baseURL.RawPath = ""
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	client := newFaviconClient(baseURL)
	page, pageURL, err := fetchFaviconResource(ctx, client, baseURL, faviconPageLimit)
	if err != nil {
		return "", fmt.Errorf("load service page: %w", err)
	}

	candidates := faviconCandidates(page, pageURL)
	fallback := *pageURL
	fallback.Path = "/favicon.ico"
	fallback.RawPath = ""
	fallback.RawQuery = ""
	fallback.Fragment = ""
	candidates = append(candidates, &fallback)

	var candidateErrors []error
	for _, candidate := range candidates {
		if !sameFaviconOrigin(baseURL, candidate) {
			continue
		}
		data, _, fetchErr := fetchFaviconResource(ctx, client, candidate, faviconImageLimit)
		if fetchErr != nil {
			candidateErrors = append(candidateErrors, fetchErr)
			continue
		}
		mediaType, mediaErr := faviconMediaType(data)
		if mediaErr != nil {
			candidateErrors = append(candidateErrors, mediaErr)
			continue
		}
		return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	}

	if len(candidateErrors) > 0 {
		return "", fmt.Errorf("no favicon was found: %w", errors.Join(candidateErrors...))
	}
	return "", fmt.Errorf("no favicon was found")
}

func validateFaviconBaseURL(baseURL *url.URL) error {
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return fmt.Errorf("favicon lookup requires an http or https service")
	}
	if baseURL.User != nil {
		return fmt.Errorf("favicon lookup does not accept URL credentials")
	}
	ip := net.ParseIP(baseURL.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("favicon lookup requires a loopback service URL")
	}
	if baseURL.Port() == "" {
		return fmt.Errorf("favicon lookup requires a service port")
	}
	return nil
}

func newFaviconClient(origin *url.URL) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// SSH authenticates the remote host. The loopback URL cannot match the remote TLS certificate.
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
	return &http.Client{
		Transport: transport,
		Timeout:   faviconTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("favicon lookup stopped after too many redirects")
			}
			if !sameFaviconOrigin(origin, request.URL) {
				return fmt.Errorf("favicon lookup blocked a redirect to another origin")
			}
			return nil
		},
	}
}

func fetchFaviconResource(
	ctx context.Context,
	client *http.Client,
	target *url.URL,
	limit int64,
) ([]byte, *url.URL, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml,image/*;q=0.9,*/*;q=0.1")

	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, response.Request.URL, fmt.Errorf("%s returned %s", target.String(), response.Status)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, response.Request.URL, err
	}
	if int64(len(data)) > limit {
		return nil, response.Request.URL, fmt.Errorf("%s exceeded the response size limit", target.String())
	}
	return data, response.Request.URL, nil
}

func faviconCandidates(page []byte, pageURL *url.URL) []*url.URL {
	tokenizer := html.NewTokenizer(strings.NewReader(string(page)))
	var candidates []*url.URL
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return candidates
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		name, hasAttributes := tokenizer.TagName()
		if !strings.EqualFold(string(name), "link") || !hasAttributes {
			continue
		}

		var rel string
		var href string
		for hasAttributes {
			key, value, more := tokenizer.TagAttr()
			switch strings.ToLower(string(key)) {
			case "rel":
				rel = string(value)
			case "href":
				href = string(value)
			}
			hasAttributes = more
		}
		if href == "" || !hasIconRelation(rel) {
			continue
		}
		parsed, err := url.Parse(strings.TrimSpace(href))
		if err != nil {
			continue
		}
		candidates = append(candidates, pageURL.ResolveReference(parsed))
	}
}

func hasIconRelation(rel string) bool {
	for _, value := range strings.Fields(strings.ToLower(rel)) {
		if value == "icon" || strings.HasSuffix(value, "-icon") {
			return true
		}
	}
	return false
}

func sameFaviconOrigin(first *url.URL, second *url.URL) bool {
	return strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

func faviconMediaType(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("favicon response was empty")
	}
	mediaType, _, err := mime.ParseMediaType(http.DetectContentType(data))
	if err != nil || !faviconMediaTypes[mediaType] {
		return "", fmt.Errorf("favicon response was not a supported image")
	}
	return mediaType, nil
}
