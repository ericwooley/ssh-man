package browser

import "strings"

func sanitizeBrowserID(id string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	cleaned := replacer.Replace(strings.TrimSpace(id))
	if cleaned == "" {
		return "browser"
	}
	return cleaned
}
