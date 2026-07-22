//go:build darwin

package browser

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	serverdomain "ssh-man/internal/domain/server"
)

func listRunningBrowserTargets(ctx context.Context, appDataDir string, browsers []BrowserOption, servers []serverdomain.Server) ([]RunningTarget, error) {
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("inspect running browser processes: %w", err)
	}
	return buildRunningTargets(appDataDir, browsers, servers, parseProcessList(string(output))), nil
}

func parseProcessList(output string) []browserProcess {
	processes := []browserProcess{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		separator := strings.IndexAny(line, " \t")
		if separator < 1 {
			continue
		}
		pid, err := strconv.Atoi(line[:separator])
		if err != nil || pid < 1 {
			continue
		}
		command := strings.TrimSpace(line[separator:])
		if command != "" {
			processes = append(processes, browserProcess{PID: pid, Command: command})
		}
	}
	return processes
}
