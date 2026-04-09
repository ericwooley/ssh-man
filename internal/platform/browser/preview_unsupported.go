//go:build !linux && !darwin

package browser

func previewLaunchCommand(string, string, BrowserOption, int) string {
	return ""
}
