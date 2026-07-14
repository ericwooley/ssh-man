// Package buildinfo exposes build metadata shared by the desktop and CLI entrypoints.
package buildinfo

// Version is replaced for release builds with the Go linker -X flag.
var Version = "dev"
