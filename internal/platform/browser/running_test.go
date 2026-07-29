package browser

import (
	"path/filepath"
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

func TestBuildRunningTargetsDistinguishesProxyAndRegularBrowserInstances(t *testing.T) {
	root := t.TempDir()
	appDataDir := filepath.Join(root, "Application Data", "ssh-man")
	executable := filepath.Join(root, "Browsers", "Google Chrome", "chrome")
	browsers := []BrowserOption{{
		ID:                  "google-chrome",
		DisplayName:         "Google Chrome",
		LaunchReference:     executable,
		ExecutableReference: executable,
	}}
	servers := []serverdomain.Server{{ID: "server-prod", Name: "Production"}}
	profileDir := filepath.Join(profileScope(appDataDir, "server-prod", browsers[0]), "chromium")
	processes := []browserProcess{
		{PID: 101, Command: executable},
		{PID: 202, Command: executable + " --proxy-server=socks5://localhost:1080 --user-data-dir=" + profileDir},
		{PID: 303, Command: filepath.Join(root, "Browsers", "Google Chrome Helper") + " --type=renderer"},
	}

	targets := buildRunningTargets(appDataDir, browsers, servers, processes)
	if len(targets) != 2 {
		t.Fatalf("targets = %#v, want two main browser processes", targets)
	}
	if targets[0].Kind != RunningTargetProxy || targets[0].ServerName != "Production" || targets[0].PID != 202 {
		t.Fatalf("unexpected proxy target: %#v", targets[0])
	}
	if targets[1].Kind != RunningTargetRegular || targets[1].ServerID != "" || targets[1].PID != 101 {
		t.Fatalf("unexpected regular target: %#v", targets[1])
	}
}

func TestBuildRunningTargetsRecognizesFirefoxProfilesWithSpaces(t *testing.T) {
	root := t.TempDir()
	appDataDir := filepath.Join(root, "Application Data", "ssh-man")
	executable := filepath.Join(root, "Browsers", "Firefox", "firefox")
	browsers := []BrowserOption{{
		ID:                  "firefox",
		DisplayName:         "Firefox",
		LaunchReference:     executable,
		ExecutableReference: executable,
	}}
	servers := []serverdomain.Server{{ID: "server-staging", Name: "Staging"}}
	profileDir := filepath.Join(profileScope(appDataDir, "server-staging", browsers[0]), "firefox")
	command := executable + " -new-instance -profile " + profileDir

	targets := buildRunningTargets(appDataDir, browsers, servers, []browserProcess{{PID: 404, Command: command}})
	if len(targets) != 1 || targets[0].Kind != RunningTargetProxy || targets[0].ServerName != "Staging" {
		t.Fatalf("unexpected firefox targets: %#v", targets)
	}
}

func TestBuildRunningTargetsRecognizesCustomBrowserExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "Browsers", "Kagi Browser", "kagi")
	browsers := []BrowserOption{{
		ID:                  "custom-kagi",
		DisplayName:         "Kagi Browser",
		LaunchReference:     executable,
		ExecutableReference: executable,
		Engine:              BrowserEngineChromium,
		Custom:              true,
	}}
	processes := []browserProcess{{
		PID:     505,
		Command: executable,
	}}

	targets := buildRunningTargets(filepath.Join(root, "ssh-man"), browsers, nil, processes)
	if len(targets) != 1 || targets[0].BrowserID != "custom-kagi" || targets[0].Kind != RunningTargetRegular {
		t.Fatalf("custom browser targets = %#v", targets)
	}
}
