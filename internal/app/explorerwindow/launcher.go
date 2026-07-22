package explorerwindow

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const ServerArgument = "--ssh-man-explorer"

func ServerIDFromArgs(args []string) (string, bool) {
	for index, argument := range args {
		if argument == ServerArgument && index+1 < len(args) {
			serverID := strings.TrimSpace(args[index+1])
			return serverID, serverID != ""
		}
		if strings.HasPrefix(argument, ServerArgument+"=") {
			serverID := strings.TrimSpace(strings.TrimPrefix(argument, ServerArgument+"="))
			return serverID, serverID != ""
		}
	}
	return "", false
}

func Launch(serverID string) error {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate SSH Man executable: %w", err)
	}
	command := exec.Command(executable, ServerArgument, serverID)
	if err := command.Start(); err != nil {
		return fmt.Errorf("open server explorer: %w", err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release server explorer process: %w", err)
	}
	return nil
}
