package commandwindow

import "ssh-man/internal/app/companionwindow"

const ServerArgument = "--ssh-man-command"

type Manager = companionwindow.Manager

func NewManager() *Manager {
	return companionwindow.NewManager(ServerArgument, "quick command window")
}

func ServerIDFromArgs(args []string) (string, bool) {
	return companionwindow.IDFromArgs(args, ServerArgument)
}

func Launch(serverID string) error {
	return companionwindow.Launch(ServerArgument, "quick command window", serverID)
}
