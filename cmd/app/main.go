package app

import (
	"io/fs"

	"ssh-man/internal/app/runner"
)

func Main(assets fs.FS) error {
	return runner.Run(assets)
}
