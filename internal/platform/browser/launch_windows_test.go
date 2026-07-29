//go:build windows

package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWindowsChromiumLaunchArgumentsUseIsolatedServerProfile(t *testing.T) {
	option := BrowserOption{
		ID:                  "microsoft-edge",
		DisplayName:         "Microsoft Edge",
		LaunchReference:     `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		Engine:              BrowserEngineChromium,
		SupportsProxyLaunch: true,
	}
	appDataDir := `C:\Users\Test\AppData\Roaming\ssh-man`

	got, err := windowsProxyLaunchArguments(appDataDir, "server-prod", option, 41001, false)
	if err != nil {
		t.Fatalf("Windows Chromium launch arguments: %v", err)
	}
	wantProfile := filepath.Join(profileScope(appDataDir, "server-prod", option), "chromium")
	want := chromiumProxyArguments("127.0.0.1", wantProfile, 41001)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Windows Chromium arguments = %#v, want %#v", got, want)
	}

	other, err := windowsProxyLaunchArguments(appDataDir, "server-staging", option, 41001, false)
	if err != nil {
		t.Fatalf("other Windows Chromium launch arguments: %v", err)
	}
	if reflect.DeepEqual(got, other) {
		t.Fatal("different servers should not share a Chromium profile")
	}
}

func TestWindowsFirefoxLaunchArgumentsUseIsolatedProfile(t *testing.T) {
	option := BrowserOption{
		ID:                  "firefox",
		DisplayName:         "Firefox",
		LaunchReference:     `C:\Program Files\Mozilla Firefox\firefox.exe`,
		Engine:              BrowserEngineFirefox,
		SupportsProxyLaunch: true,
	}
	appDataDir := filepath.Join(t.TempDir(), "ssh-man")

	got, err := windowsProxyLaunchArguments(appDataDir, "server-prod", option, 41001, true)
	if err != nil {
		t.Fatalf("Windows Firefox launch arguments: %v", err)
	}
	profileDir := filepath.Join(profileScope(appDataDir, "server-prod", option), "firefox")
	want := []string{"-new-instance", "-profile", profileDir}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Windows Firefox arguments = %#v, want %#v", got, want)
	}

	prefs, err := os.ReadFile(filepath.Join(profileDir, "user.js"))
	if err != nil {
		t.Fatalf("read prepared Firefox profile: %v", err)
	}
	if !strings.Contains(string(prefs), `user_pref("network.proxy.socks_port", 41001);`) {
		t.Fatalf("Firefox profile does not use the runtime SOCKS port: %s", prefs)
	}
}

func TestWindowsProxyPreviewMatchesLaunchArguments(t *testing.T) {
	option := BrowserOption{
		ID:                  "microsoft-edge",
		DisplayName:         "Microsoft Edge",
		LaunchReference:     `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		Engine:              BrowserEngineChromium,
		SupportsProxyLaunch: true,
	}
	appDataDir := `C:\Users\Test\App Data\ssh-man`
	arguments, err := windowsProxyLaunchArguments(appDataDir, "server-prod", option, 41001, false)
	if err != nil {
		t.Fatalf("Windows launch arguments: %v", err)
	}
	want := formatWindowsCommand(append([]string{option.LaunchReference}, arguments...)...)

	if got := previewLaunchCommand(appDataDir, "server-prod", option, 41001); got != want {
		t.Fatalf("Windows preview = %q, want launch command %q", got, want)
	}
}
