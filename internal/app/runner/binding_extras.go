//go:build !bindings

package runner

import "io/fs"

func additionalBindingsForGeneration() []interface{} {
	return nil
}

func maybeRunBindingsGeneration(fs.FS) (bool, error) {
	return false, nil
}
