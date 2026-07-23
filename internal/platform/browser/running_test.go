package browser

import (
	"testing"

	serverdomain "ssh-man/internal/domain/server"
)

func TestBuildRunningTargetsDistinguishesProxyAndRegularBrowserInstances(t *testing.T) {
	appDataDir := "/Users/eric/Library/Application Support/ssh-man"
	browsers := []BrowserOption{{
		ID:                  "google-chrome",
		DisplayName:         "Google Chrome",
		LaunchReference:     "/Applications/Google Chrome.app",
		ExecutableReference: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	}}
	servers := []serverdomain.Server{{ID: "server-prod", Name: "Production"}}
	processes := []browserProcess{
		{PID: 101, Command: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
		{PID: 202, Command: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome --proxy-server=socks5://localhost:1080 --user-data-dir=/Users/eric/Library/Application Support/ssh-man/browser-profiles/server-prod/google-chrome/chromium"},
		{PID: 303, Command: "/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Helpers/Google Chrome Helper.app/Contents/MacOS/Google Chrome Helper --type=renderer"},
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
	browsers := []BrowserOption{{
		ID:                  "firefox",
		DisplayName:         "Firefox",
		LaunchReference:     "/Applications/Firefox.app",
		ExecutableReference: "/Applications/Firefox.app/Contents/MacOS/firefox",
	}}
	servers := []serverdomain.Server{{ID: "server-staging", Name: "Staging"}}
	command := "/Applications/Firefox.app/Contents/MacOS/firefox -new-instance -profile /Users/test/Library/Application Support/ssh-man/browser-profiles/server-staging/firefox/firefox"

	targets := buildRunningTargets("/Users/test/Library/Application Support/ssh-man", browsers, servers, []browserProcess{{PID: 404, Command: command}})
	if len(targets) != 1 || targets[0].Kind != RunningTargetProxy || targets[0].ServerName != "Staging" {
		t.Fatalf("unexpected firefox targets: %#v", targets)
	}
}

func TestBuildRunningTargetsRecognizesCustomBrowserExecutable(t *testing.T) {
	browsers := []BrowserOption{{
		ID:                  "custom-kagi",
		DisplayName:         "Kagi Browser",
		LaunchReference:     "/Applications/Kagi Browser.app",
		ExecutableReference: "/Applications/Kagi Browser.app/Contents/MacOS/Kagi Browser",
		Engine:              BrowserEngineChromium,
		Custom:              true,
	}}
	processes := []browserProcess{{
		PID:     505,
		Command: "/Applications/Kagi Browser.app/Contents/MacOS/Kagi Browser",
	}}

	targets := buildRunningTargets("/tmp/ssh-man", browsers, nil, processes)
	if len(targets) != 1 || targets[0].BrowserID != "custom-kagi" || targets[0].Kind != RunningTargetRegular {
		t.Fatalf("custom browser targets = %#v", targets)
	}
}
