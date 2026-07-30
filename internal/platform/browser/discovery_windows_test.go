//go:build windows

package browser

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	preferencesdomain "ssh-man/internal/domain/preferences"
)

func TestWindowsBrowserCatalogIncludesChromiumAndFirefoxFamilies(t *testing.T) {
	want := map[string]BrowserEngine{
		"google-chrome":  BrowserEngineChromium,
		"microsoft-edge": BrowserEngineChromium,
		"brave-browser":  BrowserEngineChromium,
		"vivaldi":        BrowserEngineChromium,
		"opera":          BrowserEngineChromium,
		"firefox":        BrowserEngineFirefox,
		"zen":            BrowserEngineFirefox,
		"librewolf":      BrowserEngineFirefox,
		"floorp":         BrowserEngineFirefox,
		"waterfox":       BrowserEngineFirefox,
	}

	for _, candidate := range windowsBrowserCandidates() {
		if engine, ok := want[candidate.ID]; ok {
			if candidate.Engine != engine {
				t.Errorf("%s engine = %q, want %q", candidate.ID, candidate.Engine, engine)
			}
			delete(want, candidate.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("Windows browser catalog is missing %#v", want)
	}
}

func TestWindowsFirefoxCatalogIncludesNonAdminInstallLocation(t *testing.T) {
	for _, candidate := range windowsBrowserCandidates() {
		if candidate.ID != "firefox" {
			continue
		}
		for _, location := range candidate.Locations {
			if location.Environment == "LOCALAPPDATA" &&
				location.RelativePath == `Mozilla Firefox\firefox.exe` {
				return
			}
		}
		t.Fatal("Firefox candidate is missing the non-admin LOCALAPPDATA install location")
	}
	t.Fatal("Firefox candidate is missing from the Windows browser catalog")
}

func TestDiscoverWindowsBrowsersUsesInstallRootsAndPath(t *testing.T) {
	candidates := []windowsBrowserCandidate{
		{
			ID:          "microsoft-edge",
			DisplayName: "Microsoft Edge",
			Locations: []windowsBrowserLocation{
				{Environment: "ProgramFiles(x86)", RelativePath: `Microsoft\Edge\Application\msedge.exe`},
			},
			Engine: BrowserEngineChromium,
		},
		{
			ID:              "firefox",
			DisplayName:     "Firefox",
			PathExecutables: []string{"firefox.exe"},
			Engine:          BrowserEngineFirefox,
		},
	}
	edgePath := filepath.Join(`C:\Program Files (x86)`, `Microsoft\Edge\Application\msedge.exe`)
	firefoxPath := filepath.Join(`D:\Portable`, "firefox.exe")
	getenv := func(name string) string {
		if name == "ProgramFiles(x86)" {
			return `C:\Program Files (x86)`
		}
		return ""
	}
	lookPath := func(name string) (string, error) {
		if name == "firefox.exe" {
			return firefoxPath, nil
		}
		return "", errors.New("not found")
	}
	isFile := func(path string) bool {
		return path == edgePath || path == firefoxPath
	}

	got := discoverWindowsBrowsersWith(candidates, getenv, lookPath, isFile)
	want := []BrowserOption{
		{
			ID:                  "microsoft-edge",
			DisplayName:         "Microsoft Edge",
			LaunchReference:     edgePath,
			ExecutableReference: edgePath,
			Engine:              BrowserEngineChromium,
			SupportsProxyLaunch: true,
		},
		{
			ID:                  "firefox",
			DisplayName:         "Firefox",
			LaunchReference:     firefoxPath,
			ExecutableReference: firefoxPath,
			Engine:              BrowserEngineFirefox,
			SupportsProxyLaunch: true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverWindowsBrowsersWith() = %#v, want %#v", got, want)
	}
}

func TestDiscoverWindowsBrowsersDeduplicatesCaseInsensitivePaths(t *testing.T) {
	path := `C:\Browsers\browser.exe`
	candidates := []windowsBrowserCandidate{
		{
			ID:          "first",
			DisplayName: "First",
			Locations: []windowsBrowserLocation{
				{Environment: "BROWSER_ROOT", RelativePath: "browser.exe"},
			},
			Engine: BrowserEngineChromium,
		},
		{
			ID:              "second",
			DisplayName:     "Second",
			PathExecutables: []string{"second.exe"},
			Engine:          BrowserEngineChromium,
		},
	}
	getenv := func(string) string { return `C:\Browsers` }
	lookPath := func(string) (string, error) { return strings.ToUpper(path), nil }
	isFile := func(candidate string) bool { return strings.EqualFold(candidate, path) }

	got := discoverWindowsBrowsersWith(candidates, getenv, lookPath, isFile)
	if len(got) != 1 || got[0].ID != "first" {
		t.Fatalf("case-insensitive duplicate discovery = %#v, want only first browser", got)
	}
}

func TestDiscoverCustomWindowsBrowserAcceptsExecutableFiles(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "custom browser.exe")
	if err := os.WriteFile(executable, []byte("test"), 0o600); err != nil {
		t.Fatalf("write custom executable: %v", err)
	}

	got := discoverCustomBrowsers([]preferencesdomain.CustomBrowser{
		{
			ID:              "custom-browser",
			DisplayName:     "Custom Browser",
			LaunchReference: executable,
			Icon:            "icon:briefcase",
			Engine:          preferencesdomain.BrowserEngineFirefox,
		},
		{
			ID:              "directory",
			DisplayName:     "Directory",
			LaunchReference: filepath.Dir(executable),
			Engine:          preferencesdomain.BrowserEngineRegular,
		},
	})

	if len(got) != 1 {
		t.Fatalf("custom browsers = %#v, want one executable", got)
	}
	if got[0].LaunchReference != executable || got[0].ExecutableReference != executable {
		t.Fatalf("custom browser executable references = %#v", got[0])
	}
	if got[0].Icon != "icon:briefcase" {
		t.Fatalf("custom browser icon = %q, want icon:briefcase", got[0].Icon)
	}
	if got[0].Engine != BrowserEngineFirefox || !got[0].SupportsProxyLaunch || !got[0].Custom {
		t.Fatalf("custom browser metadata = %#v", got[0])
	}
}
