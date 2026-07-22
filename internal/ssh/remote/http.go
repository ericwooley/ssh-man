package remote

import (
	"net/http"
	"net/url"
	"path"
	"strings"
)

const ContentPathPrefix = "/__ssh_man_remote__/raw"

func (s *Service) ContentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, ContentPathPrefix+"/") {
			next.ServeHTTP(response, request)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		escapedPath := strings.TrimPrefix(request.URL.EscapedPath(), ContentPathPrefix)
		remotePath, err := url.PathUnescape(escapedPath)
		if err != nil {
			http.Error(response, "Invalid remote path", http.StatusBadRequest)
			return
		}
		remotePath = cleanRemotePath(remotePath)
		file, info, err := s.Open(remotePath)
		if err != nil {
			http.Error(response, "Remote file unavailable", http.StatusNotFound)
			return
		}
		defer file.Close()

		sample := make([]byte, 512)
		count, _ := file.Read(sample)
		_, _ = file.Seek(0, 0)
		mimeType := contentTypeFor(remotePath, sample[:count])
		response.Header().Set("Content-Type", mimeType)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(mimeType, "text/html") || strings.EqualFold(path.Ext(remotePath), ".svg") {
			response.Header().Set("Content-Security-Policy", "sandbox allow-forms; default-src 'self' data: blob: https: http:; script-src 'none'; style-src 'self' 'unsafe-inline' data: https: http:; img-src 'self' data: blob: https: http:; font-src 'self' data: https: http:; media-src 'self' data: blob: https: http:; object-src 'none'")
		}
		http.ServeContent(response, request, info.Name(), info.ModTime(), file)
	})
}
