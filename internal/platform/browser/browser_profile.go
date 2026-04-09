package browser

import (
	"path/filepath"
	"strings"
)

func sanitizeBrowserID(id string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	cleaned := replacer.Replace(strings.TrimSpace(id))
	if cleaned == "" {
		return "browser"
	}
	return cleaned
}

func profileScope(appDataDir string, serverID string, option BrowserOption) string {
	scope := strings.TrimSpace(serverID)
	if scope == "" {
		scope = "shared"
	}

	return filepath.Join(
		appDataDir,
		"browser-profiles",
		sanitizeBrowserID(scope),
		sanitizeBrowserID(option.ID),
	)
}
