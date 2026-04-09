package browser

import (
	"fmt"
	"os"
	"path/filepath"
)

func ensureFirefoxProfile(appDataDir string, serverID string, option BrowserOption, socksPort int) (string, error) {
	profileDir := filepath.Join(profileScope(appDataDir, serverID, option), "firefox")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(profileDir, "user.js"), []byte(firefoxUserPrefs(socksPort)), 0o644); err != nil {
		return "", err
	}
	return profileDir, nil
}

func firefoxUserPrefs(socksPort int) string {
	return fmt.Sprintf(`user_pref("network.proxy.type", 1);
user_pref("network.proxy.socks", "localhost");
user_pref("network.proxy.socks_port", %d);
user_pref("network.proxy.socks_version", 5);
user_pref("network.proxy.socks_remote_dns", true);
user_pref("network.proxy.allow_hijacking_localhost", true);
user_pref("network.proxy.no_proxies_on", "");
user_pref("browser.shell.checkDefaultBrowser", false);
`, socksPort)
}
