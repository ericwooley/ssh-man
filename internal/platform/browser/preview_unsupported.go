//go:build !linux && !darwin

package browser

func previewLaunchCommand(option BrowserOption, socksPort int) string {
	return ""
}
