// Package buildinfo exposes build metadata shared by the desktop and CLI entrypoints.
package buildinfo

// Version is replaced for release builds with the Go linker -X flag.
var Version = "dev"

// DisplayVersion returns the user-facing release version. Builds without
// release metadata are identified explicitly instead of looking like an
// official release.
func DisplayVersion(version string) string {
	if version == "" || version == "dev" {
		return "Dev build"
	}
	return version
}
