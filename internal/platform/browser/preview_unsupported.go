//go:build !linux && !darwin && !windows

package browser

func previewLaunchCommand(string, string, BrowserOption, int) string {
	return ""
}
