package remoteport

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	portlinkdomain "ssh-man/internal/domain/portlink"
)

var testPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
}

func TestFindFaviconUsesPageIcon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<html><head><link rel="icon" href="/assets/app.png"></head></html>`))
		case "/assets/app.png":
			response.Header().Set("Content-Type", "image/png")
			_, _ = response.Write(testPNG)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	got, err := FindFavicon(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNG)
	if got != want {
		t.Fatalf("favicon = %q, want %q", got, want)
	}
}

func TestFindFaviconUsesConventionalFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<html><head><title>App</title></head></html>`))
		case "/favicon.ico":
			response.Header().Set("Content-Type", "image/x-icon")
			_, _ = response.Write([]byte{0x00, 0x00, 0x01, 0x00})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	got, err := FindFavicon(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != "data:image/x-icon;base64,AAABAA==" {
		t.Fatalf("favicon = %q", got)
	}
}

func TestFindFaviconDoesNotRequestAnotherOrigin(t *testing.T) {
	var externalRequests atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		externalRequests.Add(1)
		response.Header().Set("Content-Type", "image/png")
		_, _ = response.Write(testPNG)
	}))
	defer external.Close()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<link rel="icon" href="` + external.URL + `/icon.png">`))
			return
		}
		http.NotFound(response, request)
	}))
	defer server.Close()

	if _, err := FindFavicon(context.Background(), server.URL); err == nil {
		t.Fatal("expected a missing favicon error")
	}
	if externalRequests.Load() != 0 {
		t.Fatalf("external requests = %d", externalRequests.Load())
	}
}

func TestFindFaviconReturnsDataThatPortLinkValidationAccepts(t *testing.T) {
	largePNG := make([]byte, 220<<10)
	copy(largePNG, testPNG)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<link rel="icon" href="/large.png">`))
		case "/large.png":
			response.Header().Set("Content-Type", "image/png")
			_, _ = response.Write(largePNG)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	dataURL, err := FindFavicon(context.Background(), server.URL)
	if err != nil {
		return
	}
	link := portlinkdomain.Link{
		ServerID:       "server-1",
		Port:           3000,
		Name:           "Admin",
		Scheme:         portlinkdomain.SchemeHTTP,
		FaviconDataURL: dataURL,
	}
	if err := link.Validate(); err != nil {
		t.Fatalf("found favicon failed link validation: %v", err)
	}
}

func TestFindFaviconAcceptsAnImageNearTheStorageBoundary(t *testing.T) {
	boundaryPNG := make([]byte, 190<<10)
	copy(boundaryPNG, testPNG)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response.Header().Set("Content-Type", "text/html")
			_, _ = response.Write([]byte(`<link rel="icon" href="/boundary.png">`))
		case "/boundary.png":
			response.Header().Set("Content-Type", "image/png")
			_, _ = response.Write(boundaryPNG)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	dataURL, err := FindFavicon(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	link := portlinkdomain.Link{
		ServerID:       "server-1",
		Port:           3000,
		Name:           "Admin",
		Scheme:         portlinkdomain.SchemeHTTP,
		FaviconDataURL: dataURL,
	}
	if err := link.Validate(); err != nil {
		t.Fatalf("boundary favicon failed link validation: %v", err)
	}
}

func TestFindFaviconWithDialerUsesDirectRemoteConnections(t *testing.T) {
	var requestedHosts []string
	var requestedHostsMu sync.Mutex
	dial := func(_ context.Context, _, _ string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			request, err := http.ReadRequest(bufio.NewReader(server))
			if err != nil {
				return
			}
			requestedHostsMu.Lock()
			requestedHosts = append(requestedHosts, request.Host)
			requestedHostsMu.Unlock()
			var contentType string
			var body []byte
			switch request.URL.Path {
			case "/":
				contentType = "text/html"
				body = []byte(`<link rel="icon" href="/app.png">`)
			case "/app.png":
				contentType = "image/png"
				body = testPNG
			default:
				_, _ = fmt.Fprint(server, "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n")
				return
			}
			_, _ = fmt.Fprintf(
				server,
				"HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
				contentType,
				len(body),
			)
			_, _ = server.Write(body)
		}()
		return client, nil
	}

	got, err := FindFaviconWithDialer(
		context.Background(),
		"http://ssh-man-test.localhost:3000",
		dial,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("favicon = %q", got)
	}
	requestedHostsMu.Lock()
	defer requestedHostsMu.Unlock()
	if len(requestedHosts) != 2 {
		t.Fatalf("requested hosts = %#v", requestedHosts)
	}
	for _, host := range requestedHosts {
		if host != "ssh-man-test.localhost:3000" {
			t.Fatalf("requested host = %q", host)
		}
	}
}
