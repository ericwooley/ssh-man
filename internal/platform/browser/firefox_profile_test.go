package browser

import (
	"strings"
	"testing"
)

func TestFirefoxUserPrefsSuppressPromptsAndKeepWebRTCBehindProxy(t *testing.T) {
	prefs := firefoxUserPrefs(41001, "firefox")

	for _, expected := range []string{
		`user_pref("browser.shell.checkDefaultBrowser", false);`,
		`user_pref("browser.aboutwelcome.enabled", false);`,
		`user_pref("startup.homepage_welcome_url", "");`,
		`user_pref("startup.homepage_welcome_url.additional", "");`,
		`user_pref("startup.homepage_override_url", "");`,
		`user_pref("identity.fxaccounts.enabled", false);`,
		`user_pref("browser.newtabpage.activity-stream.asrouter.userprefs.cfr.addons", false);`,
		`user_pref("browser.newtabpage.activity-stream.asrouter.userprefs.cfr.features", false);`,
		`user_pref("browser.disableResetPrompt", true);`,
		`user_pref("media.peerconnection.ice.proxy_only_if_behind_proxy", true);`,
	} {
		if !strings.Contains(prefs, expected) {
			t.Errorf("Firefox user.js missing %q", expected)
		}
	}
}

func TestZenFirefoxUserPrefsSkipZenWelcomeScreen(t *testing.T) {
	prefs := firefoxUserPrefs(41001, "zen")
	if !strings.Contains(prefs, `user_pref("zen.welcome-screen.seen", true);`) {
		t.Fatalf("Zen user.js should mark its welcome screen as seen")
	}
}

func TestOtherFirefoxUserPrefsDoNotIncludeZenSettings(t *testing.T) {
	prefs := firefoxUserPrefs(41001, "firefox")
	if strings.Contains(prefs, "zen.") {
		t.Fatalf("Firefox user.js should not contain Zen-specific preferences")
	}
}
