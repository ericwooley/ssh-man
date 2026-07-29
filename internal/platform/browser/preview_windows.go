//go:build windows

package browser

func previewLaunchCommand(appDataDir string, serverID string, option BrowserOption, socksPort int) string {
	arguments, err := windowsProxyLaunchArguments(appDataDir, serverID, option, socksPort, false)
	if err != nil {
		return ""
	}
	parts := append([]string{option.LaunchReference}, arguments...)
	return formatWindowsCommand(parts...)
}
