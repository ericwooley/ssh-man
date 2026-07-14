//go:build !darwin

package cli

import (
	"context"
	"fmt"
)

func defaultLauncher(context.Context) error {
	return fmt.Errorf("automatic app startup is currently available on macOS; start SSH Man first")
}
