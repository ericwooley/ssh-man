package main

import (
	"context"
	"embed"
	"log"
	"os"

	"ssh-man/internal/app/runner"
	"ssh-man/internal/cli"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if cli.IsCLIInvocation(os.Args[1:]) {
		os.Exit(cli.Run(context.Background(), os.Args[1:], cli.DefaultDependencies()))
	}
	if err := runner.Run(assets); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
