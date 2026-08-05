# Patched fyne.io/systray

This directory contains the Windows files from `fyne.io/systray` version
`v1.12.2`.

Source: `https://github.com/fyne-io/systray`

The local patch replaces the unsynchronized exit flag with `atomic.Bool`.
This prevents concurrent Windows shutdown paths from calling the exit callback
more than once.

The root `go.mod` file replaces `fyne.io/systray` with this local module.
