package remoteport

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
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
