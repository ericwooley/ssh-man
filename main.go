package main

import (
	"context"
	"embed"
	"log"
	"os"

	"ssh-man/internal/app/commandwindow"
	"ssh-man/internal/app/explorerwindow"
	"ssh-man/internal/app/previewwindow"
	"ssh-man/internal/app/runner"
	"ssh-man/internal/app/settingswindow"
	"ssh-man/internal/cli"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if settingswindow.FromArgs(os.Args[1:]) {
		if err := runner.RunSettings(assets); err != nil {
			log.Fatalf("run settings: %v", err)
		}
		return
	}
	if serverID, ok := commandwindow.ServerIDFromArgs(os.Args[1:]); ok {
		if err := runner.RunCommand(assets, serverID); err != nil {
			log.Fatalf("run quick command window: %v", err)
		}
		return
	}
	if request, ok := previewwindow.RequestFromArgs(os.Args[1:]); ok {
		if err := runner.RunPreview(assets, request); err != nil {
			log.Fatalf("run file preview: %v", err)
		}
		return
	}
	if serverID, ok := explorerwindow.ServerIDFromArgs(os.Args[1:]); ok {
		if err := runner.RunExplorer(assets, serverID); err != nil {
			log.Fatalf("run server explorer: %v", err)
		}
		return
	}
	if !isBindingsGeneration() && cli.IsCLIInvocation(os.Args[1:]) {
		os.Exit(cli.Run(context.Background(), os.Args[1:], cli.DefaultDependencies()))
	}
	if err := runner.Run(assets); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
