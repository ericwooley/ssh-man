package browser

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	serverdomain "ssh-man/internal/domain/server"
)

type browserProcess struct {
	PID     int
	Command string
}

func buildRunningTargets(appDataDir string, browsers []BrowserOption, servers []serverdomain.Server, processes []browserProcess) []RunningTarget {
	serverNames := make(map[string]string, len(servers))
	for _, server := range servers {
		serverNames[server.ID] = server.Name
	}

	targets := make([]RunningTarget, 0, len(processes))
	for _, process := range processes {
		browser, ok := browserForCommand(browsers, process.Command)
		if !ok {
			continue
		}
		serverID := profileServerID(appDataDir, browser, servers, process.Command)
		kind := RunningTargetRegular
		if serverID != "" {
			kind = RunningTargetProxy
		}
		targets = append(targets, RunningTarget{
			ID:          fmt.Sprintf("browser:%d", process.PID),
			PID:         process.PID,
			BrowserID:   browser.ID,
			BrowserName: browser.DisplayName,
			Kind:        kind,
			ServerID:    serverID,
			ServerName:  serverNames[serverID],
		})
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].Kind != targets[j].Kind {
			return targets[i].Kind == RunningTargetProxy
		}
		if targets[i].ServerName != targets[j].ServerName {
			return targets[i].ServerName < targets[j].ServerName
		}
		if targets[i].BrowserName != targets[j].BrowserName {
			return targets[i].BrowserName < targets[j].BrowserName
		}
		return targets[i].PID < targets[j].PID
	})
	return targets
}

func browserForCommand(browsers []BrowserOption, command string) (BrowserOption, bool) {
	for _, browser := range browsers {
		executable := strings.TrimSpace(browser.ExecutableReference)
		if executable == "" {
			continue
		}
		if command == executable || strings.HasPrefix(command, executable+" ") {
			return browser, true
		}
	}
	return BrowserOption{}, false
}

func profileServerID(appDataDir string, browser BrowserOption, servers []serverdomain.Server, command string) string {
	for _, server := range servers {
		profileRoot := profileScope(appDataDir, server.ID, browser)
		if strings.Contains(command, filepath.Join(profileRoot, "chromium")) ||
			strings.Contains(command, filepath.Join(profileRoot, "firefox")) {
			return server.ID
		}
	}
	return ""
}
