package explorerwindow

import (
	"ssh-man/internal/app/companionwindow"
)

const ServerArgument = "--ssh-man-explorer"

type Manager = companionwindow.Manager

func NewManager() *Manager {
	return companionwindow.NewManager(ServerArgument, "server explorer")
}

func ServerIDFromArgs(args []string) (string, bool) {
	return companionwindow.IDFromArgs(args, ServerArgument)
}

func Launch(serverID string) error {
	return companionwindow.Launch(ServerArgument, "server explorer", serverID)
}
